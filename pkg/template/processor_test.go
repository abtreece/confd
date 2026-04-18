package template

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
)

// erroringStoreClient is a mock StoreClient that returns errors from WatchPrefix
// immediately unless the context is cancelled or the stop channel is closed.
type erroringStoreClient struct{}

func (c *erroringStoreClient) GetValues(_ context.Context, _ []string) (map[string]string, error) {
	return nil, nil
}

func (c *erroringStoreClient) WatchPrefix(ctx context.Context, _ string, _ []string, _ uint64, stopChan chan bool) (uint64, error) {
	select {
	case <-stopChan:
		return 0, errors.New("watch stopped")
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		return 0, errors.New("simulated backend error")
	}
}

func (c *erroringStoreClient) HealthCheck(_ context.Context) error { return nil }
func (c *erroringStoreClient) Close() error                        { return nil }

var _ backends.StoreClient = (*erroringStoreClient)(nil)

// TestMonitorPrefix_NonBlockingErrChan verifies that monitorPrefix does not
// deadlock when errChan is full. With the old blocking send (p.errChan <- err),
// the goroutine would stall on the first error when there is no reader. After
// the fix (non-blocking select), errors are dropped and the goroutine exits
// cleanly when the context is cancelled.
func TestMonitorPrefix_NonBlockingErrChan(t *testing.T) {
	// Suppress the expected "error dropped" Warning logs — this test intentionally
	// floods errChan to verify the goroutine doesn't deadlock, so dropped-error
	// warnings are noise rather than signal here.
	log.SetLevel("error")
	t.Cleanup(func() { log.SetLevel("info") })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	p := &watchProcessor{
		config: Config{
			Ctx:               ctx,
			WatchErrorBackoff: time.Millisecond,
		},
		errChan:      make(chan error), // unbuffered, no reader — fills instantly
		internalStop: make(chan bool),
	}

	tr := &TemplateResource{
		storeClient:  &erroringStoreClient{},
		prefixedKeys: []string{"/test"},
		Dest:         "/tmp/test.conf",
	}

	p.wg.Add(1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.monitorPrefix(tr)
	}()

	select {
	case <-done:
		// goroutine exited cleanly — non-blocking errChan confirmed
	case <-time.After(time.Second):
		t.Fatal("monitorPrefix blocked on full errChan (deadlock regression)")
	}
}

// TestMonitorForBatch_NonBlockingErrChan verifies that monitorForBatch does not
// deadlock when errChan is full, using the same approach as TestMonitorPrefix_NonBlockingErrChan.
func TestMonitorForBatch_NonBlockingErrChan(t *testing.T) {
	log.SetLevel("error")
	t.Cleanup(func() { log.SetLevel("info") })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	p := &batchWatchProcessor{
		config: Config{
			Ctx:               ctx,
			WatchErrorBackoff: time.Millisecond,
		},
		errChan:      make(chan error), // unbuffered, no reader
		changeChan:   make(chan *TemplateResource, 1),
		internalStop: make(chan bool),
	}

	tr := &TemplateResource{
		storeClient:  &erroringStoreClient{},
		prefixedKeys: []string{"/test"},
		Dest:         "/tmp/test.conf",
	}

	p.wg.Add(1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.monitorForBatch(tr)
	}()

	select {
	case <-done:
		// goroutine exited cleanly
	case <-time.After(time.Second):
		t.Fatal("monitorForBatch blocked on full errChan (deadlock regression)")
	}
}

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

func TestGetTemplateResources_NonExistentConfigDir(t *testing.T) {
	config := Config{
		ConfDir:   "/nonexistent/path",
		ConfigDir: "/nonexistent/path/conf.d",
	}

	templates, err := getTemplateResources(config)
	// Should return nil, nil when config dir doesn't exist (logs warning)
	if err != nil {
		t.Errorf("getTemplateResources() unexpected error: %v", err)
	}
	if templates != nil {
		t.Errorf("getTemplateResources() = %v, want nil", templates)
	}
}

func TestGetTemplateResources_ConfDirExistsConfigDirMissing(t *testing.T) {
	// Regression test for: ConfDir existence check passing when ConfigDir is absent.
	// Previously, getTemplateResources statted ConfDir but scanned ConfigDir.
	// If ConfDir existed but ConfigDir did not, the check passed silently and
	// RecursiveFilesLookup returned a confusing EvalSymlinks error.
	confDir := t.TempDir() // ConfDir exists
	config := Config{
		ConfDir:   confDir,
		ConfigDir: confDir + "/conf.d", // ConfigDir does not exist
	}

	templates, err := getTemplateResources(config)
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
