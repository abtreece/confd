package template

import (
	"context"
	"fmt"

	"github.com/abtreece/confd/pkg/log"
)

// Preflight performs connectivity and configuration checks without processing templates.
// It verifies:
// - Backend connectivity via HealthCheck
// - Template resources can be loaded
// - Keys are accessible in the backend (warning if not found)
func Preflight(config Config) error {
	log.Info("Running pre-flight checks...")

	// Check backend connectivity
	log.Info("Checking backend connectivity...")
	ctx, cancel := context.WithTimeout(context.Background(), config.PreflightTimeout)
	defer cancel()

	if err := config.StoreClient.HealthCheck(ctx); err != nil {
		log.Error("Backend connectivity check failed: %v", err)
		return fmt.Errorf("backend connectivity failed: %w", err)
	}
	log.Info("Backend: OK")

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

	// Check key accessibility for each template
	log.Info("Checking key accessibility...")
	var warnings []string
	var errors []string

	for _, t := range ts {
		vals, err := config.StoreClient.GetValues(ctx, t.prefixedKeys)
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
