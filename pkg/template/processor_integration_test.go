package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
)

// processorTestEnv holds the test environment for processor integration tests.
type processorTestEnv struct {
	confDir  string // conf.d directory with TOML configs
	templDir string // templates directory
	destDir  string // output destination directory
	dataDir  string // backend data directory (YAML files)
	t        *testing.T
}

// newProcessorTestEnv creates a new test environment with temp directories.
func newProcessorTestEnv(t *testing.T) *processorTestEnv {
	t.Helper()

	baseDir := t.TempDir()
	confDir := filepath.Join(baseDir, "conf.d")
	templDir := filepath.Join(baseDir, "templates")
	destDir := filepath.Join(baseDir, "dest")
	dataDir := filepath.Join(baseDir, "data")

	for _, dir := range []string{confDir, templDir, destDir, dataDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	return &processorTestEnv{
		confDir:  confDir,
		templDir: templDir,
		destDir:  destDir,
		dataDir:  dataDir,
		t:        t,
	}
}

// writeTemplate creates a template file in the templates directory.
func (e *processorTestEnv) writeTemplate(name, content string) string {
	e.t.Helper()
	path := filepath.Join(e.templDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.t.Fatalf("Failed to write template %s: %v", name, err)
	}
	return path
}

// writeConfig creates a template resource TOML config in conf.d.
func (e *processorTestEnv) writeConfig(name, content string) string {
	e.t.Helper()
	path := filepath.Join(e.confDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.t.Fatalf("Failed to write config %s: %v", name, err)
	}
	return path
}

// writeData creates a YAML data file in the backend data directory.
func (e *processorTestEnv) writeData(name, content string) string {
	e.t.Helper()
	path := filepath.Join(e.dataDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.t.Fatalf("Failed to write data %s: %v", name, err)
	}
	// Sync to ensure fsnotify picks up the write
	f, err := os.Open(path)
	if err != nil {
		e.t.Logf("Warning: failed to open file for sync: %v (write succeeded, but fsnotify may be delayed)", err)
	} else {
		f.Sync()
		f.Close()
	}
	return path
}

// updateData modifies an existing data file to trigger watch events.
func (e *processorTestEnv) updateData(name, content string) {
	e.t.Helper()
	path := filepath.Join(e.dataDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.t.Fatalf("Failed to update data %s: %v", name, err)
	}
	// Sync to ensure fsnotify picks up the write
	f, err := os.Open(path)
	if err != nil {
		e.t.Logf("Warning: failed to open file for sync: %v (write succeeded, but fsnotify may be delayed)", err)
	} else {
		f.Sync()
		f.Close()
	}
}

// readDest reads the content of a destination file.
func (e *processorTestEnv) readDest(name string) (string, error) {
	e.t.Helper()
	path := filepath.Join(e.destDir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// destPath returns the full path to a destination file.
func (e *processorTestEnv) destPath(name string) string {
	return filepath.Join(e.destDir, name)
}

// createConfig creates a Config for the test environment.
func (e *processorTestEnv) createConfig(ctx context.Context) Config {
	storeClient, err := backends.New(backends.Config{
		Backend:  "file",
		YAMLFile: []string{e.dataDir},
		Filter:   "*.yaml",
	})
	if err != nil {
		e.t.Fatalf("Failed to create store client: %v", err)
	}

	return Config{
		Ctx:          ctx,
		ConfDir:      filepath.Dir(e.confDir), // parent of conf.d
		ConfigDir:    e.confDir,
		TemplateDir:  e.templDir,
		StoreClient:  storeClient,
		Noop:         false,
	}
}

// createConfigWithBatch creates a Config with batch interval for batch processor tests.
func (e *processorTestEnv) createConfigWithBatch(ctx context.Context, batchInterval time.Duration) Config {
	cfg := e.createConfig(ctx)
	cfg.BatchInterval = batchInterval
	return cfg
}

// waitForFile waits for a file to exist and optionally contain expected content.
func waitForFile(t *testing.T, path string, timeout time.Duration, expectedContent string) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil {
			if expectedContent == "" || strings.TrimSpace(string(content)) == strings.TrimSpace(expectedContent) {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for file %s with content %q", path, expectedContent)
}

// waitForFileUpdate waits for a file to be updated with new content.
func waitForFileUpdate(t *testing.T, path string, timeout time.Duration, notContent string) (string, error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil {
			trimmed := strings.TrimSpace(string(content))
			if trimmed != "" && trimmed != strings.TrimSpace(notContent) {
				return trimmed, nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", fmt.Errorf("timeout waiting for file %s to update from %q", path, notContent)
}

// TestWatchProcessor_Integration_BasicFileChange tests that WatchProcessor
// detects file changes and updates the output.
func TestWatchProcessor_Integration_BasicFileChange(t *testing.T) {
	env := newProcessorTestEnv(t)

	// Setup template
	env.writeTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	// Setup config
	destPath := env.destPath("output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.writeConfig("test.toml", configContent)

	// Setup initial data
	env.writeData("test.yaml", `test:
  value: initial
`)

	// Create processor
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := env.createConfig(ctx)
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	processor := WatchProcessor(config, stopChan, doneChan, errChan, make(chan struct{}))

	// Start processor in goroutine
	go processor.Process()

	// Wait for initial output
	if err := waitForFile(t, destPath, 5*time.Second, "value=initial"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Update data file to trigger watch
	time.Sleep(100 * time.Millisecond) // Small delay to ensure watch is ready
	env.updateData("test.yaml", `test:
  value: updated
`)

	// Wait for updated output
	content, err := waitForFileUpdate(t, destPath, 5*time.Second, "value=initial")
	if err != nil {
		t.Fatalf("Output not updated after change: %v", err)
	}

	if content != "value=updated" {
		t.Errorf("Expected 'value=updated', got %q", content)
	}

	// Cleanup
	close(stopChan)
	select {
	case <-doneChan:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Processor did not stop within timeout")
	}
}

// TestWatchProcessor_Integration_Debounce tests that rapid changes are debounced.
func TestWatchProcessor_Integration_Debounce(t *testing.T) {
	env := newProcessorTestEnv(t)

	// Setup template that includes a timestamp-like counter
	env.writeTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	// Setup config with debounce
	destPath := env.destPath("debounce-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
debounce = "500ms"
`, destPath)
	env.writeConfig("debounce.toml", configContent)

	// Setup initial data
	env.writeData("test.yaml", `test:
  value: v0
`)

	// Create processor
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config := env.createConfig(ctx)
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	processor := WatchProcessor(config, stopChan, doneChan, errChan, make(chan struct{}))

	// Start processor
	go processor.Process()

	// Wait for initial output
	if err := waitForFile(t, destPath, 5*time.Second, "value=v0"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Track initial mtime
	initialStat, _ := os.Stat(destPath)
	initialMtime := initialStat.ModTime()

	// Make 5 rapid changes within debounce window
	time.Sleep(100 * time.Millisecond)
	for i := 1; i <= 5; i++ {
		env.updateData("test.yaml", fmt.Sprintf(`test:
  value: v%d
`, i))
		time.Sleep(50 * time.Millisecond) // 50ms between changes, well under 500ms debounce
	}

	// Wait for debounce period + processing time
	time.Sleep(800 * time.Millisecond)

	// Check final content - should be v5 (last value)
	content, err := env.readDest("debounce-output.txt")
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if strings.TrimSpace(content) != "value=v5" {
		t.Errorf("Expected 'value=v5', got %q", strings.TrimSpace(content))
	}

	// Verify file was only written once after debounce (mtime should be after all rapid writes)
	finalStat, _ := os.Stat(destPath)
	finalMtime := finalStat.ModTime()

	if !finalMtime.After(initialMtime) {
		t.Error("File mtime should be updated after debounce")
	}

	// Cleanup
	close(stopChan)
	select {
	case <-doneChan:
	case <-time.After(2 * time.Second):
		t.Error("Processor did not stop within timeout")
	}
}

// TestWatchProcessor_Integration_GracefulShutdown tests clean shutdown.
func TestWatchProcessor_Integration_GracefulShutdown(t *testing.T) {
	env := newProcessorTestEnv(t)

	// Setup template
	env.writeTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	destPath := env.destPath("shutdown-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.writeConfig("shutdown.toml", configContent)

	env.writeData("test.yaml", `test:
  value: test
`)

	ctx, cancel := context.WithCancel(context.Background())
	config := env.createConfig(ctx)
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	processor := WatchProcessor(config, stopChan, doneChan, errChan, make(chan struct{}))

	// Start processor
	go processor.Process()

	// Wait for initial processing
	if err := waitForFile(t, destPath, 5*time.Second, ""); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Signal shutdown via context cancellation
	cancel()

	// Verify doneChan is closed
	select {
	case <-doneChan:
		// Success - processor shut down cleanly
	case <-time.After(3 * time.Second):
		t.Error("Processor did not shut down within timeout after context cancellation")
	}
}

// TestWatchProcessor_Integration_MultipleTemplates tests multiple templates
// watching the same prefix.
func TestWatchProcessor_Integration_MultipleTemplates(t *testing.T) {
	env := newProcessorTestEnv(t)

	// Setup two different templates
	env.writeTemplate("tmpl1.tmpl", `template1: {{ getv "/test/value" }}`)
	env.writeTemplate("tmpl2.tmpl", `template2: {{ getv "/test/value" }}`)

	// Setup configs for both templates
	dest1Path := env.destPath("output1.txt")
	dest2Path := env.destPath("output2.txt")

	config1Content := fmt.Sprintf(`[template]
src = "tmpl1.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, dest1Path)
	env.writeConfig("tmpl1.toml", config1Content)

	config2Content := fmt.Sprintf(`[template]
src = "tmpl2.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, dest2Path)
	env.writeConfig("tmpl2.toml", config2Content)

	// Setup initial data
	env.writeData("test.yaml", `test:
  value: shared
`)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := env.createConfig(ctx)
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	processor := WatchProcessor(config, stopChan, doneChan, errChan, make(chan struct{}))

	go processor.Process()

	// Wait for both initial outputs
	if err := waitForFile(t, dest1Path, 5*time.Second, "template1: shared"); err != nil {
		t.Fatalf("Template 1 output not created: %v", err)
	}
	if err := waitForFile(t, dest2Path, 5*time.Second, "template2: shared"); err != nil {
		t.Fatalf("Template 2 output not created: %v", err)
	}

	// Update data
	time.Sleep(100 * time.Millisecond)
	env.updateData("test.yaml", `test:
  value: updated
`)

	// Verify both outputs are updated
	if _, err := waitForFileUpdate(t, dest1Path, 5*time.Second, "template1: shared"); err != nil {
		t.Errorf("Template 1 not updated: %v", err)
	}
	if _, err := waitForFileUpdate(t, dest2Path, 5*time.Second, "template2: shared"); err != nil {
		t.Errorf("Template 2 not updated: %v", err)
	}

	close(stopChan)
	select {
	case <-doneChan:
	case <-time.After(2 * time.Second):
		t.Error("Processor did not stop within timeout")
	}
}

// TestBatchProcessor_Integration_Accumulation tests that changes are batched.
func TestBatchProcessor_Integration_Accumulation(t *testing.T) {
	env := newProcessorTestEnv(t)

	// Setup template
	env.writeTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	destPath := env.destPath("batch-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.writeConfig("batch.toml", configContent)

	env.writeData("test.yaml", `test:
  value: initial
`)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config := env.createConfigWithBatch(ctx, 500*time.Millisecond)
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	processor := BatchWatchProcessor(config, stopChan, doneChan, errChan, make(chan struct{}))

	go processor.Process()

	// Wait for initial output
	if err := waitForFile(t, destPath, 5*time.Second, "value=initial"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Make multiple changes within batch interval
	time.Sleep(100 * time.Millisecond)
	for i := 1; i <= 3; i++ {
		env.updateData("test.yaml", fmt.Sprintf(`test:
  value: batch%d
`, i))
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for batch interval + processing
	time.Sleep(800 * time.Millisecond)

	// Final value should be batch3
	content, err := env.readDest("batch-output.txt")
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if strings.TrimSpace(content) != "value=batch3" {
		t.Errorf("Expected 'value=batch3', got %q", strings.TrimSpace(content))
	}

	close(stopChan)
	select {
	case <-doneChan:
	case <-time.After(2 * time.Second):
		t.Error("Processor did not stop within timeout")
	}
}

// TestBatchProcessor_Integration_GracefulShutdown tests pending changes are flushed.
func TestBatchProcessor_Integration_GracefulShutdown(t *testing.T) {
	env := newProcessorTestEnv(t)

	env.writeTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	destPath := env.destPath("batch-shutdown-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.writeConfig("batch-shutdown.toml", configContent)

	env.writeData("test.yaml", `test:
  value: initial
`)

	ctx, cancel := context.WithCancel(context.Background())
	// Use a 2-second batch interval - long enough to have pending changes when we shutdown
	config := env.createConfigWithBatch(ctx, 2*time.Second)
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	processor := BatchWatchProcessor(config, stopChan, doneChan, errChan, make(chan struct{}))

	go processor.Process()

	// Wait for initial output (batch processes after interval)
	if err := waitForFile(t, destPath, 5*time.Second, "value=initial"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Make a change that will be pending
	time.Sleep(100 * time.Millisecond)
	env.updateData("test.yaml", `test:
  value: pending
`)

	// Wait briefly for change to be detected but not processed
	time.Sleep(200 * time.Millisecond)

	// Shutdown while change is pending (before next batch interval)
	cancel()

	// Wait for processor to finish
	select {
	case <-doneChan:
	case <-time.After(3 * time.Second):
		t.Fatal("Processor did not shut down within timeout")
	}

	// Verify pending change was flushed on shutdown
	content, err := env.readDest("batch-shutdown-output.txt")
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if strings.TrimSpace(content) != "value=pending" {
		t.Errorf("Expected 'value=pending' (flushed on shutdown), got %q", strings.TrimSpace(content))
	}
}

// TestIntervalProcessor_Integration_BasicPolling tests interval-based polling.
func TestIntervalProcessor_Integration_BasicPolling(t *testing.T) {
	env := newProcessorTestEnv(t)

	env.writeTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	destPath := env.destPath("interval-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.writeConfig("interval.toml", configContent)

	env.writeData("test.yaml", `test:
  value: initial
`)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config := env.createConfig(ctx)
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	// Use 1 second interval for faster testing
	processor := IntervalProcessor(config, stopChan, doneChan, errChan, 1, make(chan struct{}))

	go processor.Process()

	// Wait for initial output
	if err := waitForFile(t, destPath, 5*time.Second, "value=initial"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Update data (won't be detected until next poll)
	env.updateData("test.yaml", `test:
  value: polled
`)

	// Wait for next poll interval
	time.Sleep(1500 * time.Millisecond)

	content, err := env.readDest("interval-output.txt")
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if strings.TrimSpace(content) != "value=polled" {
		t.Errorf("Expected 'value=polled', got %q", strings.TrimSpace(content))
	}

	close(stopChan)
	select {
	case <-doneChan:
	case <-time.After(2 * time.Second):
		t.Error("Processor did not stop within timeout")
	}
}

// TestIntervalProcessor_Integration_Shutdown tests shutdown during wait.
func TestIntervalProcessor_Integration_Shutdown(t *testing.T) {
	env := newProcessorTestEnv(t)

	env.writeTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	destPath := env.destPath("interval-shutdown-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.writeConfig("interval-shutdown.toml", configContent)

	env.writeData("test.yaml", `test:
  value: test
`)

	ctx, cancel := context.WithCancel(context.Background())
	config := env.createConfig(ctx)
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	// Use long interval (60s) - we'll cancel before it elapses
	processor := IntervalProcessor(config, stopChan, doneChan, errChan, 60, make(chan struct{}))

	go processor.Process()

	// Wait for initial processing
	if err := waitForFile(t, destPath, 5*time.Second, ""); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Cancel during the long wait
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Should exit quickly, not wait for 60s
	select {
	case <-doneChan:
		// Success - exited before interval
	case <-time.After(3 * time.Second):
		t.Error("Processor did not respond to context cancellation")
	}
}

