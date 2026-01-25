package template

import (
	"context"
	"fmt"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
)

// Preflight performs connectivity and configuration checks without processing templates.
// It verifies:
// - Backend connectivity via HealthCheck (global and per-resource backends)
// - Template resources can be loaded
// - Keys are accessible in the backend (warning if not found)
func Preflight(config Config) error {
	log.Info("Running pre-flight checks...")

	ctx, cancel := context.WithTimeout(context.Background(), config.PreflightTimeout)
	defer cancel()

	// Check global backend connectivity (if configured)
	checkedBackends := make(map[backends.StoreClient]bool)
	if config.StoreClient != nil {
		log.Info("Checking global backend connectivity...")
		if err := config.StoreClient.HealthCheck(ctx); err != nil {
			log.Error("Global backend connectivity check failed: %v", err)
			return fmt.Errorf("global backend connectivity failed: %w", err)
		}
		log.Info("Global backend: OK")
		checkedBackends[config.StoreClient] = true
	}

	// Load template resources
	log.Info("Loading template resources...")
	ts, err := getTemplateResources(config)
	if err != nil {
		log.Error("Failed to load template resources: %v", err)
		return fmt.Errorf("failed to load template resources: %w", err)
	}
	log.Info("Templates: %d loaded", len(ts))

	if len(ts) == 0 {
		log.Warning("No template resources found")
		return nil
	}

	// Check connectivity for unique per-resource backends
	for _, t := range ts {
		if !checkedBackends[t.storeClient] {
			log.Info("Checking per-resource backend for %s...", t.Src)
			if err := t.storeClient.HealthCheck(ctx); err != nil {
				log.Error("Per-resource backend check failed for %s: %v", t.Src, err)
				return fmt.Errorf("per-resource backend connectivity failed for %s: %w", t.Src, err)
			}
			log.Info("Per-resource backend for %s: OK", t.Src)
			checkedBackends[t.storeClient] = true
		}
	}

	// Check key accessibility for each template using its own backend client
	log.Info("Checking key accessibility...")
	var warnings []string
	var errors []string

	for _, t := range ts {
		vals, err := t.storeClient.GetValues(ctx, t.prefixedKeys)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: backend error: %v", t.Src, err))
			continue
		}

		if len(vals) == 0 {
			warnings = append(warnings, fmt.Sprintf("%s: no keys found for prefix %s", t.Src, t.Prefix))
		} else {
			log.Info("  %s: %d keys found", t.Src, len(vals))
		}
	}

	// Report warnings
	for _, w := range warnings {
		log.Warning("%s", w)
	}

	// Report errors
	if len(errors) > 0 {
		for _, e := range errors {
			log.Error("%s", e)
		}
		return fmt.Errorf("%d template(s) failed key accessibility check", len(errors))
	}

	log.Info("Pre-flight checks completed successfully")
	return nil
}
