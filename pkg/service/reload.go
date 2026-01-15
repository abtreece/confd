package service

import (
	"sync"

	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/template"
)

// ReloadManager coordinates configuration reload across the application.
type ReloadManager struct {
	mu          sync.RWMutex
	subscribers []chan struct{}
}

// NewReloadManager creates a new reload manager.
func NewReloadManager() *ReloadManager {
	return &ReloadManager{
		subscribers: make([]chan struct{}, 0),
	}
}

// Subscribe registers a channel to receive reload notifications.
// Returns the reload channel that will receive signals (not be closed).
func (r *ReloadManager) Subscribe() chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	reloadChan := make(chan struct{}, 1) // Buffered to prevent blocking
	r.subscribers = append(r.subscribers, reloadChan)
	return reloadChan
}

// TriggerReload initiates a configuration reload.
// It clears the template cache and sends signals to all subscribers.
func (r *ReloadManager) TriggerReload() {
	log.Info("Triggering configuration reload")

	// Clear template cache
	template.ClearTemplateCache()
	log.Debug("Template cache cleared")

	// Send signal to all subscribers (non-blocking)
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, ch := range r.subscribers {
		select {
		case ch <- struct{}{}:
			// Signal sent successfully
		default:
			// Channel buffer full (reload already pending), skip
		}
	}

	log.Info("Reload notification sent to %d subscriber(s)", len(r.subscribers))
}
