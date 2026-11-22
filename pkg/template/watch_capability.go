package template

import (
	"errors"
	"fmt"

	"github.com/abtreece/confd/pkg/backends"
)

// CheckWatchSupport verifies that watch mode can run with config: the global
// backend and the backend of every template resource must implement
// backends.Watcher. It returns an error naming each backend that cannot
// watch, and the template resource for per-resource backends.
//
// Template resources that fail to load are not reported here; the processor
// reports them when it loads the resources itself.
func CheckWatchSupport(config Config) error {
	if config.StoreClient != nil {
		if _, ok := config.StoreClient.(backends.Watcher); !ok {
			return fmt.Errorf("watch mode not supported for %s backend", backendLabel(config.Backend))
		}
	}

	// The global backend can watch, so any resource that cannot is using its
	// own [backend] section. Per-resource clients come from the client cache,
	// so loading the resources here does not open extra connections.
	ts, _ := getTemplateResources(config)

	var errs []error
	for _, t := range ts {
		if _, err := t.watcher(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// watcher returns the backend client of t as a backends.Watcher, or an error
// naming the backend and the template resource if the backend cannot watch.
func (t *TemplateResource) watcher() (backends.Watcher, error) {
	watcher, ok := t.storeClient.(backends.Watcher)
	if !ok {
		return nil, fmt.Errorf("watch mode not supported for %s backend (template resource %s)", backendLabel(t.backendName), t.path)
	}
	return watcher, nil
}

// backendLabel returns name, or a placeholder for a Config that was built
// without a backend name.
func backendLabel(name string) string {
	if name == "" {
		return "configured"
	}
	return name
}
