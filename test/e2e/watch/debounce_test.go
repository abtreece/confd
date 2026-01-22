//go:build e2e

package watch

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/containers"
	"github.com/abtreece/confd/test/e2e/testenv"
)

// TestEtcdWatch_Debounce verifies that rapid changes within the debounce
// window are coalesced into a single update.
func TestEtcdWatch_Debounce(t *testing.T) {
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
	if err := env.SetBackendValue(containerCtx, "/test/value", "v0"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}

	// Create template
	env.WriteTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	// Create config with debounce
	destPath := env.DestPath("debounce-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
debounce = "500ms"
`, destPath)
	env.WriteConfig("debounce.toml", configContent)

	// Create cancellable context for processor
	processorCtx, cancelProcessor := context.WithCancel(containerCtx)

	// Create and start watch processor
	processor, _, doneChan, errChan, err := env.CreateWatchProcessor(processorCtx)
	if err != nil {
		t.Fatalf("Failed to create watch processor: %v", err)
	}

	go processor.Process()

	// Wait for initial output
	if err := testenv.WaitForFile(t, destPath, 10*time.Second, "value=v0"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Track initial mtime
	initialStat, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Failed to stat initial file: %v", err)
	}
	initialMtime := initialStat.ModTime()

	// Make 5 rapid changes within debounce window
	time.Sleep(100 * time.Millisecond)
	for i := 1; i <= 5; i++ {
		if err := env.SetBackendValue(containerCtx, "/test/value", fmt.Sprintf("v%d", i)); err != nil {
			t.Fatalf("Failed to set value v%d: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond) // 50ms between changes, well under 500ms debounce
	}

	// Wait for debounce period + processing time
	time.Sleep(800 * time.Millisecond)

	// Check final content - should be v5 (last value)
	content, err := env.ReadDest("debounce-output.txt")
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if strings.TrimSpace(content) != "value=v5" {
		t.Errorf("Expected 'value=v5', got %q", strings.TrimSpace(content))
	}

	// Verify file was updated after debounce
	finalStat, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Failed to stat final file: %v", err)
	}
	finalMtime := finalStat.ModTime()

	if !finalMtime.After(initialMtime) {
		t.Error("File mtime should be updated after debounce")
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

// TestConsulWatch_Debounce verifies that debounce works with Consul backend.
func TestConsulWatch_Debounce(t *testing.T) {
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
	if err := env.SetBackendValue(containerCtx, "/test/value", "v0"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}

	// Create template
	env.WriteTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	// Create config with debounce
	destPath := env.DestPath("debounce-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
debounce = "500ms"
`, destPath)
	env.WriteConfig("debounce.toml", configContent)

	// Create cancellable context for processor
	processorCtx, cancelProcessor := context.WithCancel(containerCtx)

	// Create and start watch processor
	processor, _, doneChan, errChan, err := env.CreateWatchProcessor(processorCtx)
	if err != nil {
		t.Fatalf("Failed to create watch processor: %v", err)
	}

	go processor.Process()

	// Wait for initial output
	if err := testenv.WaitForFile(t, destPath, 10*time.Second, "value=v0"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Make 5 rapid changes within debounce window
	time.Sleep(200 * time.Millisecond) // Consul needs a bit more time to settle
	for i := 1; i <= 5; i++ {
		if err := env.SetBackendValue(containerCtx, "/test/value", fmt.Sprintf("v%d", i)); err != nil {
			t.Fatalf("Failed to set value v%d: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for debounce period + processing time
	time.Sleep(800 * time.Millisecond)

	// Check final content - should be v5 (last value)
	content, err := env.ReadDest("debounce-output.txt")
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if strings.TrimSpace(content) != "value=v5" {
		t.Errorf("Expected 'value=v5', got %q", strings.TrimSpace(content))
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

// TestRedisWatch_Debounce verifies that debounce works with Redis backend.
func TestRedisWatch_Debounce(t *testing.T) {
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
	if err := env.SetBackendValue(containerCtx, "/test/value", "v0"); err != nil {
		t.Fatalf("Failed to set initial value: %v", err)
	}

	// Create template
	env.WriteTemplate("test.tmpl", `value={{ getv "/test/value" }}`)

	// Create config with debounce
	destPath := env.DestPath("debounce-output.txt")
	configContent := fmt.Sprintf(`[template]
src = "test.tmpl"
dest = "%s"
keys = ["/test"]
prefix = "/"
debounce = "500ms"
`, destPath)
	env.WriteConfig("debounce.toml", configContent)

	// Create cancellable context for processor
	processorCtx, cancelProcessor := context.WithCancel(containerCtx)

	// Create and start watch processor
	processor, _, doneChan, errChan, err := env.CreateWatchProcessor(processorCtx)
	if err != nil {
		t.Fatalf("Failed to create watch processor: %v", err)
	}

	go processor.Process()

	// Wait for initial output
	if err := testenv.WaitForFile(t, destPath, 10*time.Second, "value=v0"); err != nil {
		t.Fatalf("Initial output not created: %v", err)
	}

	// Make 5 rapid changes within debounce window
	time.Sleep(500 * time.Millisecond) // Redis PubSub needs more time to settle
	for i := 1; i <= 5; i++ {
		if err := env.SetBackendValue(containerCtx, "/test/value", fmt.Sprintf("v%d", i)); err != nil {
			t.Fatalf("Failed to set value v%d: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for debounce period + processing time
	time.Sleep(800 * time.Millisecond)

	// Check final content - should be v5 (last value)
	content, err := env.ReadDest("debounce-output.txt")
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if strings.TrimSpace(content) != "value=v5" {
		t.Errorf("Expected 'value=v5', got %q", strings.TrimSpace(content))
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
