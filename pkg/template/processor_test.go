package template

import (
	"testing"
)

func TestIntervalProcessor_Creation(t *testing.T) {
	config := Config{}
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error)
	interval := 10

	processor := IntervalProcessor(config, stopChan, doneChan, errChan, interval, make(chan struct{}))
	if processor == nil {
		t.Error("IntervalProcessor() returned nil")
	}

	// Verify it's the right type
	_, ok := processor.(*intervalProcessor)
	if !ok {
		t.Error("IntervalProcessor() did not return *intervalProcessor")
	}
}

func TestWatchProcessor_Creation(t *testing.T) {
	config := Config{}
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error)

	processor := WatchProcessor(config, stopChan, doneChan, errChan, make(chan struct{}))
	if processor == nil {
		t.Error("WatchProcessor() returned nil")
	}

	// Verify it's the right type
	_, ok := processor.(*watchProcessor)
	if !ok {
		t.Error("WatchProcessor() did not return *watchProcessor")
	}
}

func TestProcess_EmptyTemplateResources(t *testing.T) {
	// Test that process with empty slice doesn't error
	err := process([]*TemplateResource{}, FailModeBestEffort)
	if err != nil {
		t.Errorf("process([]) unexpected error: %v", err)
	}
}

func TestProcess_NilTemplateResources(t *testing.T) {
	// Test that process with nil slice doesn't error
	err := process(nil, FailModeBestEffort)
	if err != nil {
		t.Errorf("process(nil) unexpected error: %v", err)
	}
}

func TestGetTemplateResources_NonExistentConfDir(t *testing.T) {
	config := Config{
		ConfDir:   "/nonexistent/path",
		ConfigDir: "/nonexistent/path/conf.d",
	}

	templates, err := getTemplateResources(config)
	// Should return nil, nil when confdir doesn't exist (logs warning)
	if err != nil {
		t.Errorf("getTemplateResources() unexpected error: %v", err)
	}
	if templates != nil {
		t.Errorf("getTemplateResources() = %v, want nil", templates)
	}
}

func TestGetTemplateResources_EmptyConfigDir(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		ConfDir:   tmpDir,
		ConfigDir: tmpDir,
	}

	templates, err := getTemplateResources(config)
	if err != nil {
		t.Errorf("getTemplateResources() unexpected error: %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("getTemplateResources() returned %d templates, want 0", len(templates))
	}
}

func TestProcessWithResult_BestEffortMode(t *testing.T) {
	// Test processWithResult with best-effort mode (continues on errors)
	// Note: This is a unit test that doesn't actually process templates,
	// but tests the structure. Real processing is tested in integration tests.

	result := processWithResult([]*TemplateResource{}, FailModeBestEffort)
	if result == nil {
		t.Fatal("processWithResult() returned nil")
	}
	if result.Total != 0 {
		t.Errorf("processWithResult().Total = %d, want 0", result.Total)
	}
	if result.Succeeded != 0 {
		t.Errorf("processWithResult().Succeeded = %d, want 0", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("processWithResult().Failed = %d, want 0", result.Failed)
	}
	if err := result.Error(); err != nil {
		t.Errorf("processWithResult().Error() = %v, want nil", err)
	}
}

func TestProcessWithResult_FailFastMode(t *testing.T) {
	// Test processWithResult with fail-fast mode
	// Note: This is a unit test that doesn't actually process templates,
	// but tests the structure. Real processing is tested in integration tests.

	result := processWithResult([]*TemplateResource{}, FailModeFast)
	if result == nil {
		t.Fatal("processWithResult() returned nil")
	}
	if result.Total != 0 {
		t.Errorf("processWithResult().Total = %d, want 0", result.Total)
	}
	if result.Succeeded != 0 {
		t.Errorf("processWithResult().Succeeded = %d, want 0", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("processWithResult().Failed = %d, want 0", result.Failed)
	}
	if err := result.Error(); err != nil {
		t.Errorf("processWithResult().Error() = %v, want nil", err)
	}
}

// Note: Full processor tests are in processor_integration_test.go which tests:
// - WatchProcessor with file change detection, debouncing, and graceful shutdown
// - BatchWatchProcessor with batching, deduplication, and graceful shutdown
// - IntervalProcessor with polling and shutdown behavior
//
// These unit tests cover factory creation and edge cases. The integration tests
// use the file backend for real filesystem event detection without requiring
// external services.
