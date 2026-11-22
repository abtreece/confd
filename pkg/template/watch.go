package template

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/metrics"
)

// watchLoop is the lifecycle plumbing shared by watchProcessor and
// batchWatchProcessor: loading template resources, restarting on reload,
// and running one watch monitor per template resource.
type watchLoop struct {
	// name identifies the mode in log messages ("watch" or "batch watch").
	name            string
	config          Config
	stopChan        chan bool
	doneChan        chan bool
	errChan         chan error
	wg              sync.WaitGroup
	reloadChan      <-chan struct{}
	internalStop    chan bool
	reloadRequested atomic.Bool
}

func (w *watchLoop) context() context.Context {
	if w.config.Ctx == nil {
		return context.Background()
	}
	return w.config.Ctx
}

// run loads the template resources and calls start to launch the goroutines
// that watch them. Every goroutine start launches must be registered with
// w.wg and must exit once w.internalStop is closed or the context is
// cancelled. On a reload signal the resources are loaded again and start is
// called again; on a stop signal or context cancellation run returns.
func (w *watchLoop) run(start func(ts []*TemplateResource)) {
	defer close(w.doneChan)
	ctx := w.context()

	// Outer loop for handling reloads
	for {
		ts, err := loadTemplateResources(w.config)
		if err != nil {
			w.errChan <- err
			return
		}

		// Calculate total watched keys for metrics
		if metrics.Enabled() {
			totalKeys := 0
			for _, t := range ts {
				totalKeys += len(t.prefixedKeys)
			}
			metrics.WatchedKeys.Set(float64(totalKeys))
		}

		// Reset internal stop channel for this iteration
		w.internalStop = make(chan bool)
		w.reloadRequested.Store(false)

		// Start a goroutine to monitor for reload/stop signals
		stopMonitor := make(chan bool)
		go func() {
			defer close(stopMonitor)
			defer close(w.internalStop)
			select {
			case <-w.stopChan:
				// External stop signal - propagate to internal
			case <-w.reloadChan:
				log.Info("Reloading template resources (%s processor)", w.name)
				w.reloadRequested.Store(true)
			case <-ctx.Done():
				// Context cancelled - stop watching
				log.Debug("Context cancelled, stopping %s processor", w.name)
			}
		}()

		start(ts)
		w.wg.Wait()

		// Wait for the stop monitor so internalStop is not reset under it
		<-stopMonitor

		// If it was a real stop (not reload), exit
		if !w.reloadRequested.Load() {
			return
		}
		// Otherwise, loop to reload templates and restart watches
		log.Info("Restarting %ses with reloaded templates", w.name)
	}
}

// monitor watches the keys of t until the loop is stopped or the context is
// cancelled, calling onChange after every change the backend reports.
// onChange returns false to end the monitor. The caller must have registered
// the monitor with w.wg.
func (w *watchLoop) monitor(t *TemplateResource, onChange func() bool) {
	defer w.wg.Done()
	ctx := w.context()

	// Startup rejects backends that cannot watch (CheckWatchSupport), so this
	// only fails for a resource that a reload switched to such a backend.
	watcher, err := t.watcher()
	if err != nil {
		trySendErr(w.errChan, err, t.Dest)
		return
	}

	for {
		index, err := watcher.WatchPrefix(ctx, t.Prefix, t.prefixedKeys, t.lastIndex, w.internalStop)
		if err != nil {
			// Check if context was cancelled
			if ctx.Err() != nil {
				w.logContextCancelled(t)
				return
			}
			trySendErr(w.errChan, err, t.Dest)
			// Prevent backend errors from consuming all resources.
			if !w.backoff(ctx, t) {
				return
			}
			continue
		}
		t.lastIndex = index

		if !onChange() {
			return
		}
	}
}

// backoff waits out WatchErrorBackoff. It returns false without waiting the
// full duration if the context is cancelled or the loop is stopped.
func (w *watchLoop) backoff(ctx context.Context, t *TemplateResource) bool {
	timer := time.NewTimer(w.config.WatchErrorBackoff)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		w.logContextCancelled(t)
		return false
	case <-w.internalStop:
		return false
	}
}

func (w *watchLoop) logContextCancelled(t *TemplateResource) {
	log.Debug("Context cancelled, stopping %s for %s", w.name, t.Dest)
}
