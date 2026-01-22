//go:build e2e

package watch

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/containers"
	"github.com/abtreece/confd/test/e2e/testenv"
)

// TestEtcdBatch_Accumulation verifies that changes are accumulated and processed
// at batch intervals with the BatchWatchProcessor.
func TestEtcdBatch_Accumulation(t *testing.T) {
	containerCtx := context.Background()

	// Setup etcd container
	container := containers.NewEtcdContainer()
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
	if err := env.SetBackendValue(containerCtx, "/test/value", "initial"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}

	// Create template
	env.WriteTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	// Create config
	destPath := env.DestPath("batch-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.WriteConfig("batch.toml", configContent)

	// Create cancellable context for processor
	processorCtx, cancelProcessor := context.WithCancel(containerCtx)

	// Create batch watch processor with 500ms batch interval
	processor, _, doneChan, errChan, err := env.CreateBatchWatchProcessor(processorCtx, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create batch watch processor: %v", err)
	}

	go processor.Process()

	// Wait for initial output
	if err := testenv.WaitForFile(t, destPath, 10*time.Second, "value=initial"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Make multiple changes within batch interval
	time.Sleep(100 * time.Millisecond)
	for i := 1; i <= 3; i++ {
		if err := env.SetBackendValue(containerCtx, "/test/value", fmt.Sprintf("batch%d", i)); err != nil {
			t.Fatalf("Failed to set value batch%d: %v", i, err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for batch interval + processing
	time.Sleep(800 * time.Millisecond)

	// Final value should be batch3
	content, err := env.ReadDest("batch-output.txt")
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if strings.TrimSpace(content) != "value=batch3" {
		t.Errorf("Expected 'value=batch3', got %q", strings.TrimSpace(content))
	}

	// Check for errors
	select {
	case err := <-errChan:
		t.Errorf("Unexpected error from processor: %v", err)
	default:
	}

	// Cleanup
	cancelProcessor()
	select {
	case <-doneChan:
	case <-time.After(5 * time.Second):
		t.Error("Processor did not stop within timeout")
	}
}

// NOTE: TestEtcdBatch_GracefulShutdown is not included here because with real backends
// that share the processor context, pending changes cannot be flushed during shutdown
// since the backend operations fail with "context canceled". This behavior is tested
// in the unit tests with the file backend (TestBatchProcessor_Integration_GracefulShutdown)
// which doesn't use the context for backend operations.

// TestConsulBatch_Accumulation verifies batch processing with Consul backend.
func TestConsulBatch_Accumulation(t *testing.T) {
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
	if err := env.SetBackendValue(containerCtx, "/test/value", "initial"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}

	// Create template
	env.WriteTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	// Create config
	destPath := env.DestPath("batch-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.WriteConfig("batch.toml", configContent)

	// Create cancellable context for processor
	processorCtx, cancelProcessor := context.WithCancel(containerCtx)

	// Create batch watch processor with 500ms batch interval
	processor, _, doneChan, errChan, err := env.CreateBatchWatchProcessor(processorCtx, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create batch watch processor: %v", err)
	}

	go processor.Process()

	// Wait for initial output
	if err := testenv.WaitForFile(t, destPath, 10*time.Second, "value=initial"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Make multiple changes within batch interval
	time.Sleep(200 * time.Millisecond) // Consul needs a bit more time
	for i := 1; i <= 3; i++ {
		if err := env.SetBackendValue(containerCtx, "/test/value", fmt.Sprintf("batch%d", i)); err != nil {
			t.Fatalf("Failed to set value batch%d: %v", i, err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for batch interval + processing
	time.Sleep(800 * time.Millisecond)

	// Final value should be batch3
	content, err := env.ReadDest("batch-output.txt")
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if strings.TrimSpace(content) != "value=batch3" {
		t.Errorf("Expected 'value=batch3', got %q", strings.TrimSpace(content))
	}

	// Check for errors
	select {
	case err := <-errChan:
		t.Errorf("Unexpected error from processor: %v", err)
	default:
	}

	// Cleanup
	cancelProcessor()
	select {
	case <-doneChan:
	case <-time.After(5 * time.Second):
		t.Error("Processor did not stop within timeout")
	}
}

// TestRedisBatch_Accumulation verifies batch processing with Redis backend.
func TestRedisBatch_Accumulation(t *testing.T) {
	containerCtx := context.Background()

	// Setup Redis container
	container := containers.NewRedisContainer()
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
	if err := env.SetBackendValue(containerCtx, "/test/value", "initial"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}

	// Create template
	env.WriteTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	// Create config
	destPath := env.DestPath("batch-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.WriteConfig("batch.toml", configContent)

	// Create cancellable context for processor
	processorCtx, cancelProcessor := context.WithCancel(containerCtx)

	// Create batch watch processor with 500ms batch interval
	processor, _, doneChan, errChan, err := env.CreateBatchWatchProcessor(processorCtx, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create batch watch processor: %v", err)
	}

	go processor.Process()

	// Wait for initial output
	if err := testenv.WaitForFile(t, destPath, 10*time.Second, "value=initial"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Make multiple changes within batch interval
	time.Sleep(500 * time.Millisecond) // Redis PubSub needs more time
	for i := 1; i <= 3; i++ {
		if err := env.SetBackendValue(containerCtx, "/test/value", fmt.Sprintf("batch%d", i)); err != nil {
			t.Fatalf("Failed to set value batch%d: %v", i, err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for batch interval + processing
	time.Sleep(800 * time.Millisecond)

	// Final value should be batch3
	content, err := env.ReadDest("batch-output.txt")
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if strings.TrimSpace(content) != "value=batch3" {
		t.Errorf("Expected 'value=batch3', got %q", strings.TrimSpace(content))
	}

	// Check for errors
	select {
	case err := <-errChan:
		t.Errorf("Unexpected error from processor: %v", err)
	default:
	}

	// Cleanup
	cancelProcessor()
	select {
	case <-doneChan:
	case <-time.After(5 * time.Second):
		t.Error("Processor did not stop within timeout")
	}
}
