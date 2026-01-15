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
// The channel will be closed when a reload is triggered.
// Returns the reload channel.
func (r *ReloadManager) Subscribe() chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	reloadChan := make(chan struct{})
	r.subscribers = append(r.subscribers, reloadChan)
	return reloadChan
}

// TriggerReload initiates a configuration reload.
// It clears the template cache and notifies all subscribers.
func (r *ReloadManager) TriggerReload() {
	log.Info("Triggering configuration reload")

	// Clear template cache
	template.ClearTemplateCache()
	log.Debug("Template cache cleared")

	// Notify all subscribers
	r.mu.Lock()
	oldSubscribers := r.subscribers
	r.subscribers = make([]chan struct{}, 0)
	r.mu.Unlock()

	for _, ch := range oldSubscribers {
		close(ch)
	}

	log.Info("Reload notification sent to %d subscriber(s)", len(oldSubscribers))
}
