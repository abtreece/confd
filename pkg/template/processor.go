package template

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/metrics"
	util "github.com/abtreece/confd/pkg/util"
)

// trySendErr attempts a non-blocking send of err to ch.
// If ch is full, it logs a warning that includes dest so operators can identify
// which template resource triggered the drop.
func trySendErr(ch chan error, err error, dest string) {
	select {
	case ch <- err:
	default:
		log.Warning("error dropped (errChan full) for %s: %s", dest, err)
	}
}

// Processor defines the interface for template processing strategies.
type Processor interface {
	Process()
}

// Process loads and processes all template resources once.
func Process(config Config) error {
	ts, err := getTemplateResources(config)
	if err != nil {
		return err
	}
	return process(ts, config.FailureMode)
}

func process(ts []*TemplateResource, failureMode FailureMode) error {
	result := processWithResult(ts, failureMode)
	return result.Error()
}

func processWithResult(ts []*TemplateResource, failureMode FailureMode) *BatchProcessResult {
	result := &BatchProcessResult{Total: len(ts), Statuses: make([]TemplateStatus, 0, len(ts))}

	for _, t := range ts {
		status := TemplateStatus{Dest: t.Dest}
		if err := t.process(); err != nil {
			log.Error("%s", err.Error())
			status.Error = err
			result.Failed++
			result.Statuses = append(result.Statuses, status)

			if failureMode == FailModeFast {
				log.Warning("Fail-fast mode: stopping after error in %s", t.Dest)
				break
			}
		} else {
			status.Success = true
			result.Succeeded++
			result.Statuses = append(result.Statuses, status)
		}
	}

	// Record batch metrics
	if metrics.Enabled() {
		metrics.BatchProcessTotal.Inc()
		if result.Failed > 0 {
			metrics.BatchProcessFailed.Inc()
		}
		metrics.BatchProcessTemplatesSucceeded.Add(float64(result.Succeeded))
		metrics.BatchProcessTemplatesFailed.Add(float64(result.Failed))
	}
	return result
}

type intervalProcessor struct {
	config     Config
	stopChan   chan bool
	doneChan   chan bool
	errChan    chan error
	interval   int
	reloadChan <-chan struct{}
}

// IntervalProcessor creates a processor that polls for changes at a fixed interval.
func IntervalProcessor(config Config, stopChan, doneChan chan bool, errChan chan error, interval int, reloadChan <-chan struct{}) Processor {
	return &intervalProcessor{config, stopChan, doneChan, errChan, interval, reloadChan}
}

func (p *intervalProcessor) Process() {
	defer close(p.doneChan)
	ctx := p.config.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Load template resources once at startup. Only reload on SIGHUP (reloadChan).
	ts, err := getTemplateResources(p.config)
	if err != nil {
		if len(ts) == 0 {
			log.Error("Failed to load any template resources: %s", err.Error())
			p.errChan <- err
			return
		}
		log.Warning("Some template resources failed to load: %s", err.Error())
	}

	ticker := time.NewTicker(time.Duration(p.interval) * time.Second)
	defer ticker.Stop()

	for {
		process(ts, p.config.FailureMode)
		select {
		case <-ctx.Done():
			log.Debug("Context cancelled, stopping interval processor")
			return
		case <-p.stopChan:
			return
		case <-p.reloadChan:
			log.Info("Reloading template resources (interval processor)")
			ts, err = getTemplateResources(p.config)
			if err != nil {
				if len(ts) == 0 {
					log.Error("Failed to load any template resources: %s", err.Error())
					p.errChan <- err
					return
				}
				log.Warning("Some template resources failed to load: %s", err.Error())
			}
		case <-ticker.C:
		}
	}
}

type watchProcessor struct {
	watchLoop
}

// WatchProcessor creates a processor that watches for backend changes continuously.
func WatchProcessor(config Config, stopChan, doneChan chan bool, errChan chan error, reloadChan <-chan struct{}) Processor {
	return &watchProcessor{
		watchLoop: watchLoop{
			name:         "watch",
			config:       config,
			stopChan:     stopChan,
			doneChan:     doneChan,
			errChan:      errChan,
			reloadChan:   reloadChan,
			internalStop: make(chan bool),
		},
	}
}

func (p *watchProcessor) Process() {
	p.run(func(ts []*TemplateResource) {
		// Start watching all templates
		for _, t := range ts {
			p.wg.Add(1)
			go p.monitorPrefix(t)
		}
	})
}

func (p *watchProcessor) monitorPrefix(t *TemplateResource) {
	ctx := p.context()

	// Debounce timer management - create once and reuse with Reset.
	// Reset can be safely called on expired timers without draining.
	var debounceTimer *time.Timer
	if t.debounceDur > 0 {
		debounceTimer = time.NewTimer(0)
		debounceTimer.Stop() // Start in stopped state
		defer debounceTimer.Stop()
	}

	p.monitor(t, func() bool {
		// Handle debouncing
		if t.debounceDur > 0 {
			// Reset the debounce timer
			debounceTimer.Reset(t.debounceDur)

			log.Debug("Debouncing changes for %s (%v)", t.Dest, t.debounceDur)

			// Wait for either debounce timer to fire or shutdown
			select {
			case <-ctx.Done():
				p.logContextCancelled(t)
				return false
			case <-debounceTimer.C:
				// Debounce period elapsed, process the template
				log.Debug("Debounce period elapsed for %s, processing", t.Dest)
				if err := t.process(); err != nil {
					trySendErr(p.errChan, err, t.Dest)
				}
			case <-p.internalStop:
				return false
			}
			return true
		}

		// Check for shutdown signals before processing
		select {
		case <-ctx.Done():
			p.logContextCancelled(t)
			return false
		case <-p.internalStop:
			return false
		default:
			// No debouncing, process immediately
			if err := t.process(); err != nil {
				trySendErr(p.errChan, err, t.Dest)
			}
		}
		return true
	})
}

// batchWatchProcessor processes changes in batches after a batch interval
type batchWatchProcessor struct {
	watchLoop
	changeChan chan *TemplateResource
}

// BatchWatchProcessor creates a processor that batches changes before processing.
// Changes from all templates are collected and processed together after the batch interval.
func BatchWatchProcessor(config Config, stopChan, doneChan chan bool, errChan chan error, reloadChan <-chan struct{}) Processor {
	changeChan := make(chan *TemplateResource, 100)
	return &batchWatchProcessor{
		watchLoop: watchLoop{
			name:         "batch watch",
			config:       config,
			stopChan:     stopChan,
			doneChan:     doneChan,
			errChan:      errChan,
			reloadChan:   reloadChan,
			internalStop: make(chan bool),
		},
		changeChan: changeChan,
	}
}

func (p *batchWatchProcessor) Process() {
	p.run(func(ts []*TemplateResource) {
		// Reset change channel for this iteration
		p.changeChan = make(chan *TemplateResource, 100)

		// Start batch processor goroutine
		p.wg.Add(1)
		go p.processBatch()

		// Start monitor goroutines for each template
		for _, t := range ts {
			p.wg.Add(1)
			go p.monitorForBatch(t)
		}
	})
}

func (p *batchWatchProcessor) monitorForBatch(t *TemplateResource) {
	ctx := p.context()

	p.monitor(t, func() bool {
		// Send to batch processor
		select {
		case p.changeChan <- t:
			log.Debug("Queued change for batch processing: %s", t.Dest)
			return true
		case <-ctx.Done():
			p.logContextCancelled(t)
			return false
		case <-p.internalStop:
			return false
		}
	})
}

func (p *batchWatchProcessor) processBatch() {
	defer p.wg.Done()

	ctx := p.context()

	pending := make(map[string]*TemplateResource)
	timer := time.NewTimer(p.config.BatchInterval)
	timer.Stop() // Start with stopped timer
	defer timer.Stop()

	timerRunning := false

	for {
		select {
		case t := <-p.changeChan:
			// Add to pending changes (deduplicates by dest path)
			pending[t.Dest] = t

			// Start or reset the batch timer
			if !timerRunning {
				timer.Reset(p.config.BatchInterval)
				timerRunning = true
				log.Debug("Batch timer started (%v)", p.config.BatchInterval)
			}

		case <-timer.C:
			timerRunning = false
			if len(pending) > 0 {
				log.Info("Processing batch of %d template changes", len(pending))
				p.flush(pending)
			}

		case <-ctx.Done():
			p.flushBeforeShutdown(pending)
			return

		case <-p.internalStop:
			p.flushBeforeShutdown(pending)
			return
		}
	}
}

// flushBeforeShutdown processes any remaining pending changes before shutdown.
func (p *batchWatchProcessor) flushBeforeShutdown(pending map[string]*TemplateResource) {
	if len(pending) == 0 {
		return
	}
	log.Info("Processing %d pending template changes before shutdown", len(pending))
	p.flush(pending)
}

// flush processes the pending template resources and clears the map.
func (p *batchWatchProcessor) flush(pending map[string]*TemplateResource) {
	templates := make([]*TemplateResource, 0, len(pending))
	for _, t := range pending {
		templates = append(templates, t)
	}
	result := processWithResult(templates, p.config.FailureMode)
	if err := result.Error(); err != nil {
		p.errChan <- err
	}
	clear(pending)
}

func getTemplateResources(config Config) ([]*TemplateResource, error) {
	var lastError error
	log.Debug("Loading template resources from config dir %s", config.ConfigDir)
	if _, err := os.Stat(config.ConfigDir); err != nil {
		log.Warning("Cannot load template resources: config dir '%s': %v", config.ConfigDir, err)
		return nil, nil
	}
	paths, err := util.RecursiveFilesLookup(config.ConfigDir, "*toml")
	if err != nil {
		return nil, fmt.Errorf("look up template resources in %q: %w", config.ConfigDir, err)
	}

	if len(paths) < 1 {
		log.Warning("Found no templates")
	}

	// Pre-allocate slice capacity based on discovered template files
	templates := make([]*TemplateResource, 0, len(paths))

	for _, p := range paths {
		log.Debug("Found template: %s", p)
		t, err := NewTemplateResource(p, config)
		if err != nil {
			log.Error("Failed to load template resource %s: %s", p, err.Error())
			lastError = err
			continue
		}
		templates = append(templates, t)
	}

	// Update templates loaded metric
	if metrics.Enabled() {
		metrics.TemplatesLoaded.Set(float64(len(templates)))
	}

	return templates, lastError
}
