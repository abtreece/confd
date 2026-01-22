//go:build e2e

package watch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/containers"
	"github.com/abtreece/confd/test/e2e/testenv"
)

// TestConsulWatch_BasicChange verifies that the WatchProcessor detects
// a single key change in Consul and updates the output file.
func TestConsulWatch_BasicChange(t *testing.T) {
	// Use separate contexts for container lifecycle and processor
	containerCtx := context.Background()

	// Setup Consul container
	container := containers.NewConsulContainer()
	env := testenv.NewE2ETestEnv(t, container)

	if err := env.Setup(containerCtx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer func() {
		if err := env.Teardown(containerCtx); err != nil {
			t.Logf("Warning: failed to teardown: %v", err)
		}
	}()

	// Set initial value in Consul
	if err := env.SetBackendValue(containerCtx, "/test/value", "initial"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}

	// Create template
	env.WriteTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	// Create config
	destPath := env.DestPath("output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.WriteConfig("test.toml", configContent)

	// Create cancellable context for processor
	processorCtx, cancelProcessor := context.WithCancel(containerCtx)

	// Create and start watch processor
	processor, _, doneChan, errChan, err := env.CreateWatchProcessor(processorCtx)
	if err != nil {
		t.Fatalf("Failed to create watch processor: %v", err)
	}

	// Start processor in goroutine
	go processor.Process()

	// Wait for initial output
	if err := testenv.WaitForFile(t, destPath, 10*time.Second, "value=initial"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Update value in Consul
	time.Sleep(200 * time.Millisecond) // Allow watch to settle
	if err := env.SetBackendValue(containerCtx, "/test/value", "updated"); err != nil {
		t.Fatalf("Failed to update value: %v", err)
	}

	// Wait for updated output
	content, err := testenv.WaitForFileUpdate(t, destPath, 10*time.Second, "value=initial")
	if err != nil {
		t.Fatalf("Output not updated after change: %v", err)
	}

	if content != "value=updated" {
		t.Errorf("Expected 'value=updated', got %q", content)
	}

	// Check for errors (drain any non-blocking errors)
	select {
	case err := <-errChan:
		t.Errorf("Unexpected error from processor: %v", err)
	default:
	}

	// Cleanup - use context cancellation for reliable shutdown
	cancelProcessor()
	select {
	case <-doneChan:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Processor did not stop within timeout")
	}
}

// TestConsulWatch_MultipleUpdates verifies that the WatchProcessor correctly
// handles multiple sequential updates to the same key.
func TestConsulWatch_MultipleUpdates(t *testing.T) {
	// Use separate contexts for container lifecycle and processor
	containerCtx := context.Background()

	// Setup Consul container
	container := containers.NewConsulContainer()
	env := testenv.NewE2ETestEnv(t, container)

	if err := env.Setup(containerCtx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer func() {
		if err := env.Teardown(containerCtx); err != nil {
			t.Logf("Warning: failed to teardown: %v", err)
		}
	}()

	// Set initial value
	if err := env.SetBackendValue(containerCtx, "/counter/value", "0"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}

	// Create template
	env.WriteTemplate("counter.tmpl", `count={{ getv "/counter/value" }}`)

	// Create config
	destPath := env.DestPath("counter.txt")
	configContent := fmt.Sprintf(`[template]
src = "counter.tmpl"
dest = "%s"
keys = ["/counter"]
prefix = "/"
`, destPath)
	env.WriteConfig("counter.toml", configContent)

	// Create cancellable context for processor
	processorCtx, cancelProcessor := context.WithCancel(containerCtx)

	// Create and start watch processor
	processor, _, doneChan, errChan, err := env.CreateWatchProcessor(processorCtx)
	if err != nil {
		t.Fatalf("Failed to create watch processor: %v", err)
	}

	go processor.Process()

	// Wait for initial output
	if err := testenv.WaitForFile(t, destPath, 10*time.Second, "count=0"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Update multiple times
	for i := 1; i <= 3; i++ {
		time.Sleep(300 * time.Millisecond) // Allow watch to detect previous change

		previousValue := fmt.Sprintf("count=%d", i-1)
		newValue := fmt.Sprintf("%d", i)
		expectedContent := fmt.Sprintf("count=%d", i)

		if err := env.SetBackendValue(containerCtx, "/counter/value", newValue); err != nil {
			t.Fatalf("Failed to set value %d: %v", i, err)
		}

		content, err := testenv.WaitForFileUpdate(t, destPath, 10*time.Second, previousValue)
		if err != nil {
			t.Fatalf("Output not updated for value %d: %v", i, err)
		}

		if content != expectedContent {
			t.Errorf("Update %d: expected %q, got %q", i, expectedContent, content)
		}
	}

	// Check for errors
	select {
	case err := <-errChan:
		t.Errorf("Unexpected error from processor: %v", err)
	default:
	}

	// Cleanup - use context cancellation for reliable shutdown
	cancelProcessor()
	select {
	case <-doneChan:
	case <-time.After(5 * time.Second):
		t.Error("Processor did not stop within timeout")
	}
}

// TestConsulWatch_GracefulShutdown verifies that the WatchProcessor
// shuts down gracefully when the context is cancelled.
func TestConsulWatch_GracefulShutdown(t *testing.T) {
	// Use a separate context for container lifecycle
	containerCtx := context.Background()

	// Setup Consul container
	container := containers.NewConsulContainer()
	env := testenv.NewE2ETestEnv(t, container)

	if err := env.Setup(containerCtx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer func() {
		if err := env.Teardown(containerCtx); err != nil {
			t.Logf("Warning: failed to teardown: %v", err)
		}
	}()

	// Set initial value
	if err := env.SetBackendValue(containerCtx, "/shutdown/test", "running"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}

	// Create template
	env.WriteTemplate("shutdown.tmpl", `status={{ getv "/shutdown/test" }}`)

	// Create config
	destPath := env.DestPath("shutdown.txt")
	configContent := fmt.Sprintf(`[template]
src = "shutdown.tmpl"
dest = "%s"
keys = ["/shutdown"]
prefix = "/"
`, destPath)
	env.WriteConfig("shutdown.toml", configContent)

	// Create cancellable context for processor
	processorCtx, cancelProcessor := context.WithCancel(containerCtx)

	// Create watch processor with cancellable context
	processor, _, doneChan, errChan, err := env.CreateWatchProcessor(processorCtx)
	if err != nil {
		t.Fatalf("Failed to create watch processor: %v", err)
	}

	// Start processor
	go processor.Process()

	// Wait for initial output
	if err := testenv.WaitForFile(t, destPath, 10*time.Second, "status=running"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Cancel the processor context
	cancelProcessor()

	// Verify doneChan is closed (processor stopped)
	select {
	case <-doneChan:
		// Success - processor shut down cleanly
	case <-time.After(5 * time.Second):
		t.Error("Processor did not shut down within timeout after context cancellation")
	}

	// Check for any errors (context cancellation is expected)
	select {
	case err := <-errChan:
		// Context cancellation errors are acceptable
		if err != nil && err != context.Canceled {
			t.Logf("Error during shutdown (may be expected): %v", err)
		}
	default:
	}
}
