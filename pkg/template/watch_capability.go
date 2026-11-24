package template

import (
	"fmt"

	"github.com/abtreece/confd/pkg/backends"
)

// CheckWatchSupport verifies that watch mode can run with config: the global
// backend must implement backends.Watcher. It returns an error naming the
// global backend if it cannot watch.
//
// Template resources are not checked. In watch mode a resource whose own
// [backend] section selects a backend that cannot watch is polled every
// config.PollInterval instead of watched.
func CheckWatchSupport(config Config) error {
	if config.StoreClient != nil {
		if _, ok := config.StoreClient.(backends.Watcher); !ok {
			return fmt.Errorf("watch mode not supported for %s backend", backendLabel(config.Backend))
		}
	}
	return nil
}

// watcher returns the backend client of t as a backends.Watcher. It returns
// false if the backend cannot watch, in which case watch mode polls t.
func (t *TemplateResource) watcher() (backends.Watcher, bool) {
	watcher, ok := t.storeClient.(backends.Watcher)
	return watcher, ok
}

// backendLabel returns name, or a placeholder for a Config that was built
// without a backend name.
func backendLabel(name string) string {
	if name == "" {
		return "configured"
	}
	return name
}
