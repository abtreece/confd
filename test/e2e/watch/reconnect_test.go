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

// TestEtcdContainer_Restart verifies that the etcd container can be restarted
// and the test helper can set values after restart.
func TestEtcdContainer_Restart(t *testing.T) {
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
	if err := env.SetBackendValue(containerCtx, "/test/value", "before-restart"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}
	t.Log("Initial value set successfully")

	// Restart the backend container
	t.Log("Restarting etcd container...")
	if err := env.RestartBackend(containerCtx); err != nil {
		t.Fatalf("Failed to restart backend: %v", err)
	}
	t.Log("etcd container restarted")

	// Set a new value after restart (etcd data is ephemeral)
	if err := env.SetBackendValue(containerCtx, "/test/value", "after-restart"); err != nil {
		t.Fatalf("Failed to set value after restart: %v", err)
	}
	t.Log("New value set after restart successfully")
}

// TestEtcdWatch_GracefulDegradation verifies that the WatchProcessor
// handles backend disconnection gracefully without crashing.
func TestEtcdWatch_GracefulDegradation(t *testing.T) {
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
	destPath := env.DestPath("degradation-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
`, destPath)
	env.WriteConfig("degradation.toml", configContent)

	// Create cancellable context for processor
	processorCtx, cancelProcessor := context.WithCancel(containerCtx)

	// Create and start watch processor
	processor, _, doneChan, errChan, err := env.CreateWatchProcessor(processorCtx)
	if err != nil {
		t.Fatalf("Failed to create watch processor: %v", err)
	}

	go processor.Process()

	// Wait for initial output
	if err := testenv.WaitForFile(t, destPath, 10*time.Second, "value=initial"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}
	t.Log("Initial value processed successfully")

	// Allow watch to settle
	time.Sleep(500 * time.Millisecond)

	// Restart the backend container - this will disrupt the watch connection
	t.Log("Restarting etcd container to test graceful degradation...")
	if err := env.RestartBackend(containerCtx); err != nil {
		t.Fatalf("Failed to restart backend: %v", err)
	}
	t.Log("etcd container restarted")

	// Give the processor time to detect the disconnection
	time.Sleep(3 * time.Second)

	// The processor should still be running (not crashed)
	// Cancel and verify it shuts down cleanly
	cancelProcessor()
	select {
	case <-doneChan:
		t.Log("Processor shut down gracefully after backend restart")
	case <-time.After(10 * time.Second):
		t.Error("Processor did not stop within timeout - may be stuck")
	}

	// Drain any errors - connection errors during restart are expected
	errorCount := drainErrors(errChan)
	t.Logf("Drained %d errors (expected during backend restart)", errorCount)
}

// TestConsulContainer_Restart verifies that the Consul container can be restarted
// and the test helper can set values after restart.
func TestConsulContainer_Restart(t *testing.T) {
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
	if err := env.SetBackendValue(containerCtx, "/test/value", "before-restart"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}
	t.Log("Initial value set successfully")

	// Restart the backend container
	t.Log("Restarting Consul container...")
	if err := env.RestartBackend(containerCtx); err != nil {
		t.Fatalf("Failed to restart backend: %v", err)
	}
	t.Log("Consul container restarted")

	// Set a new value after restart (Consul data is ephemeral in dev mode)
	if err := env.SetBackendValue(containerCtx, "/test/value", "after-restart"); err != nil {
		t.Fatalf("Failed to set value after restart: %v", err)
	}
	t.Log("New value set after restart successfully")
}

// TestRedisContainer_Restart verifies that the Redis container can be restarted
// and the test helper can set values after restart.
func TestRedisContainer_Restart(t *testing.T) {
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
	if err := env.SetBackendValue(containerCtx, "/test/value", "before-restart"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}
	t.Log("Initial value set successfully")

	// Restart the backend container
	t.Log("Restarting Redis container...")
	if err := env.RestartBackend(containerCtx); err != nil {
		t.Fatalf("Failed to restart backend: %v", err)
	}
	t.Log("Redis container restarted")

	// Set a new value after restart (Redis data is ephemeral)
	if err := env.SetBackendValue(containerCtx, "/test/value", "after-restart"); err != nil {
		t.Fatalf("Failed to set value after restart: %v", err)
	}
	t.Log("New value set after restart successfully")
}

// drainErrors drains any errors from the error channel without blocking.
// Returns the count of errors drained.
func drainErrors(errChan chan error) int {
	count := 0
	for {
		select {
		case <-errChan:
			count++
		default:
			return count
		}
	}
}
