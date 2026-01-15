package template

import (
	"context"
	"sync"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/metrics"
	util "github.com/abtreece/confd/pkg/util"
)

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
	for {
		ts, err := getTemplateResources(p.config)
		if err != nil {
			log.Fatal("%s", err.Error())
			break
		}
		process(ts, p.config.FailureMode)
		select {
		case <-ctx.Done():
			log.Debug("Context cancelled, stopping interval processor")
			return
		case <-p.stopChan:
			return
		case <-p.reloadChan:
			log.Info("Reloading template resources (interval processor)")
			// Re-load templates from conf.d/ on next iteration
			continue
		case <-time.After(time.Duration(p.interval) * time.Second):
			continue
		}
	}
}

type watchProcessor struct {
	config         Config
	stopChan       chan bool
	doneChan       chan bool
	errChan        chan error
	wg             sync.WaitGroup
	reloadChan     <-chan struct{}
	internalStop   chan bool
	reloadRequested bool
}

// WatchProcessor creates a processor that watches for backend changes continuously.
func WatchProcessor(config Config, stopChan, doneChan chan bool, errChan chan error, reloadChan <-chan struct{}) Processor {
	var wg sync.WaitGroup
	return &watchProcessor{
		config:       config,
		stopChan:     stopChan,
		doneChan:     doneChan,
		errChan:      errChan,
		wg:           wg,
		reloadChan:   reloadChan,
		internalStop: make(chan bool),
	}
}

func (p *watchProcessor) Process() {
	defer close(p.doneChan)

	// Outer loop for handling reloads
	for {
		ts, err := getTemplateResources(p.config)
		if err != nil {
			log.Fatal("%s", err.Error())
			return
		}

		// Calculate total watched keys for metrics
		if metrics.Enabled() {
			totalKeys := 0
			for _, t := range ts {
				keys := util.AppendPrefix(t.Prefix, t.Keys)
				totalKeys += len(keys)
			}
			metrics.WatchedKeys.Set(float64(totalKeys))
		}

		// Reset internal stop channel for this iteration
		p.internalStop = make(chan bool)
		p.reloadRequested = false

		// Start a goroutine to monitor for reload/stop signals
		stopMonitor := make(chan bool)
		go func() {
			select {
			case <-p.internalStop:
				close(p.internalStop)
				close(stopMonitor)
			case <-p.reloadChan:
				log.Info("Reloading template resources (watch processor)")
				p.reloadRequested = true
				close(p.internalStop)
				close(stopMonitor)
			}
		}()

		// Start watching all templates
		for _, t := range ts {
			t := t
			p.wg.Add(1)
			go p.monitorPrefix(t)
		}
		p.wg.Wait()

		// Clean up the stop monitor
		select {
		case <-stopMonitor:
		default:
			close(stopMonitor)
		}

		// If it was a real stop (not reload), exit
		if !p.reloadRequested {
			return
		}
		// Otherwise, loop to reload templates and restart watches
		log.Info("Restarting watches with reloaded templates")
	}
}

func (p *watchProcessor) monitorPrefix(t *TemplateResource) {
	defer p.wg.Done()
	keys := util.AppendPrefix(t.Prefix, t.Keys)

	ctx := p.config.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Debounce timer management
	var debounceTimer *time.Timer
	var debounceChan <-chan time.Time

	for {
		index, err := t.storeClient.WatchPrefix(ctx, t.Prefix, keys, t.lastIndex, p.internalStop)
		if err != nil {
			// Check if context was cancelled
			if ctx.Err() != nil {
				log.Debug("Context cancelled, stopping watch for %s", t.Dest)
				return
			}
			p.errChan <- err
			// Prevent backend errors from consuming all resources.
			time.Sleep(p.config.WatchErrorBackoff)
			continue
		}
		t.lastIndex = index

		// Handle debouncing
		if t.debounceDur > 0 {
			// Reset or create debounce timer
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(t.debounceDur)
			debounceChan = debounceTimer.C

			log.Debug("Debouncing changes for %s (%v)", t.Dest, t.debounceDur)

			// Wait for either debounce timer to fire or more changes
			select {
			case <-ctx.Done():
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				log.Debug("Context cancelled, stopping watch for %s", t.Dest)
				return
			case <-debounceChan:
				// Debounce period elapsed, process the template
				log.Debug("Debounce period elapsed for %s, processing", t.Dest)
				if err := t.process(); err != nil {
					p.errChan <- err
				}
			case <-p.internalStop:
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				return
			}
		} else {
			// Check for shutdown signals before processing
			select {
			case <-ctx.Done():
				log.Debug("Context cancelled, stopping watch for %s", t.Dest)
				return
			case <-p.internalStop:
				return
			default:
				// No debouncing, process immediately
				if err := t.process(); err != nil {
					p.errChan <- err
				}
			}
		}
	}
}

// batchWatchProcessor processes changes in batches after a batch interval
type batchWatchProcessor struct {
	config          Config
	stopChan        chan bool
	doneChan        chan bool
	errChan         chan error
	changeChan      chan *TemplateResource
	wg              sync.WaitGroup
	reloadChan      <-chan struct{}
	internalStop    chan bool
	reloadRequested bool
}

// BatchWatchProcessor creates a processor that batches changes before processing.
// Changes from all templates are collected and processed together after the batch interval.
func BatchWatchProcessor(config Config, stopChan, doneChan chan bool, errChan chan error, reloadChan <-chan struct{}) Processor {
	var wg sync.WaitGroup
	changeChan := make(chan *TemplateResource, 100)
	return &batchWatchProcessor{
		config:       config,
		stopChan:     stopChan,
		doneChan:     doneChan,
		errChan:      errChan,
		changeChan:   changeChan,
		wg:           wg,
		reloadChan:   reloadChan,
		internalStop: make(chan bool),
	}
}

func (p *batchWatchProcessor) Process() {
	defer close(p.doneChan)

	// Outer loop for handling reloads
	for {
		ts, err := getTemplateResources(p.config)
		if err != nil {
			log.Fatal("%s", err.Error())
			return
		}

		// Calculate total watched keys for metrics
		if metrics.Enabled() {
			totalKeys := 0
			for _, t := range ts {
				keys := util.AppendPrefix(t.Prefix, t.Keys)
				totalKeys += len(keys)
			}
			metrics.WatchedKeys.Set(float64(totalKeys))
		}

		// Reset internal stop channel and change channel for this iteration
		p.internalStop = make(chan bool)
		p.changeChan = make(chan *TemplateResource, 100)
		p.reloadRequested = false

		// Start a goroutine to monitor for reload/stop signals
		stopMonitor := make(chan bool)
		go func() {
			select {
			case <-p.stopChan:
				close(p.internalStop)
				close(stopMonitor)
			case <-p.reloadChan:
				log.Info("Reloading template resources (batch watch processor)")
				p.reloadRequested = true
				close(p.internalStop)
				close(stopMonitor)
			}
		}()

		// Start batch processor goroutine
		p.wg.Add(1)
		go p.processBatch()

		// Start monitor goroutines for each template
		for _, t := range ts {
			t := t
			p.wg.Add(1)
			go p.monitorForBatch(t)
		}

		p.wg.Wait()

		// Clean up the stop monitor
		select {
		case <-stopMonitor:
		default:
			close(stopMonitor)
		}

		// If it was a real stop (not reload), exit
		if !p.reloadRequested {
			return
		}
		// Otherwise, loop to reload templates and restart watches
		log.Info("Restarting batch watches with reloaded templates")
	}
}

func (p *batchWatchProcessor) monitorForBatch(t *TemplateResource) {
	defer p.wg.Done()
	keys := util.AppendPrefix(t.Prefix, t.Keys)

	ctx := p.config.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		index, err := t.storeClient.WatchPrefix(ctx, t.Prefix, keys, t.lastIndex, p.internalStop)
		if err != nil {
			// Check if context was cancelled
			if ctx.Err() != nil {
				log.Debug("Context cancelled, stopping batch watch for %s", t.Dest)
				return
			}
			p.errChan <- err
			time.Sleep(p.config.WatchErrorBackoff)
			continue
		}
		t.lastIndex = index

		// Send to batch processor
		select {
		case p.changeChan <- t:
			log.Debug("Queued change for batch processing: %s", t.Dest)
		case <-ctx.Done():
			log.Debug("Context cancelled, stopping batch watch for %s", t.Dest)
			return
		case <-p.internalStop:
			return
		}
	}
}

func (p *batchWatchProcessor) processBatch() {
	defer p.wg.Done()

	ctx := p.config.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	pending := make(map[string]*TemplateResource)
	timer := time.NewTimer(p.config.BatchInterval)
	timer.Stop() // Start with stopped timer

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
				// Convert pending map to slice for processWithResult
				templates := make([]*TemplateResource, 0, len(pending))
				for _, t := range pending {
					templates = append(templates, t)
				}
				result := processWithResult(templates, p.config.FailureMode)
				if err := result.Error(); err != nil {
					p.errChan <- err
				}
				// Clear pending map
				clear(pending)
			}

		case <-ctx.Done():
			timer.Stop()
			// Process any remaining pending changes before shutdown
			if len(pending) > 0 {
				log.Info("Processing %d pending template changes before shutdown", len(pending))
				templates := make([]*TemplateResource, 0, len(pending))
				for _, t := range pending {
					templates = append(templates, t)
				}
				result := processWithResult(templates, p.config.FailureMode)
				if err := result.Error(); err != nil {
					p.errChan <- err
				}
			}
			return

		case <-p.internalStop:
			timer.Stop()
			// Process any remaining pending changes before shutdown
			if len(pending) > 0 {
				log.Info("Processing %d pending template changes before shutdown", len(pending))
				templates := make([]*TemplateResource, 0, len(pending))
				for _, t := range pending {
					templates = append(templates, t)
				}
				result := processWithResult(templates, p.config.FailureMode)
				if err := result.Error(); err != nil {
					p.errChan <- err
				}
			}
			return
		}
	}
}

func getTemplateResources(config Config) ([]*TemplateResource, error) {
	var lastError error
	templates := make([]*TemplateResource, 0)
	log.Debug("Loading template resources from confdir %s", config.ConfDir)
	if !util.IsFileExist(config.ConfDir) {
		log.Warning("Cannot load template resources: confdir '%s' does not exist", config.ConfDir)
		return nil, nil
	}
	paths, err := util.RecursiveFilesLookup(config.ConfigDir, "*toml")
	if err != nil {
		return nil, err
	}

	if len(paths) < 1 {
		log.Warning("Found no templates")
	}

	for _, p := range paths {
		log.Debug("Found template: %s", p)
		t, err := NewTemplateResource(p, config)
		if err != nil {
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
