//go:build e2e

package operations

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
)

// TestSignals_SIGHUP_ContinuesRunning verifies that confd continues running
// after receiving SIGHUP (reload signal).
func TestSignals_SIGHUP_ContinuesRunning(t *testing.T) {
	env := NewTestEnv(t)
	destPath := env.DestPath("sighup.conf")

	// Write template
	env.WriteTemplate("sighup.tmpl", `database_port={{ getv "/database/port" }}
`)

	// Write config
	env.WriteConfig("sighup.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "sighup.tmpl"
dest = "%s"
keys = ["/database/port"]
`, destPath))

	// Start confd in interval mode (watch mode doesn't work with env backend for this test)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("DATABASE_PORT", "3306")
	err := confd.Start(ctx, "env", "--interval", "2", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for initial config file to be created
	if err := WaitForFile(t, destPath, 10*time.Second, "database_port=3306\n"); err != nil {
		t.Fatalf("Initial config file not created: %v", err)
	}

	// Verify the config file content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}
	if string(content) != "database_port=3306\n" {
		t.Fatalf("Initial config has incorrect value: %q", string(content))
	}

	// Send SIGHUP
	if err := confd.SendSignal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait a moment for the signal to be processed
	time.Sleep(2 * time.Second)

	// Verify process is still running
	if !confd.IsRunning() {
		t.Error("confd exited after SIGHUP (should continue running)")
	}

	// Verify config file still exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Config file missing after SIGHUP")
	}
}

// TestSignals_SIGTERM_GracefulShutdown verifies that confd exits cleanly
// when receiving SIGTERM.
func TestSignals_SIGTERM_GracefulShutdown(t *testing.T) {
	env := NewTestEnv(t)
	destPath := env.DestPath("sigterm.conf")

	// Write template
	env.WriteTemplate("sigterm.tmpl", `database_port={{ getv "/database/port" }}
`)

	// Write config
	env.WriteConfig("sigterm.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "sigterm.tmpl"
dest = "%s"
keys = ["/database/port"]
`, destPath))

	// Start confd in interval mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("DATABASE_PORT", "5432")
	err := confd.Start(ctx, "env", "--interval", "2", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	// Wait for initial config file to be created (indicates confd is ready)
	if err := WaitForFile(t, destPath, 10*time.Second, "database_port=5432\n"); err != nil {
		t.Fatalf("Initial config file not created: %v", err)
	}

	// Send SIGTERM
	if err := confd.SendSignal(syscall.SIGTERM); err != nil {
		t.Fatalf("Failed to send SIGTERM: %v", err)
	}

	// Wait for process to exit with timeout
	exitCodeChan := make(chan int, 1)
	errChan := make(chan error, 1)

	go func() {
		code, err := confd.Wait()
		if err != nil {
			errChan <- err
			return
		}
		exitCodeChan <- code
	}()

	select {
	case exitCode := <-exitCodeChan:
		// Exit code 0 is clean exit, 143 is 128 + SIGTERM (15)
		if exitCode != 0 && exitCode != 143 {
			t.Errorf("confd exited with unexpected code: %d (expected 0 or 143)", exitCode)
		}
	case err := <-errChan:
		t.Fatalf("Error waiting for process: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for confd to exit after SIGTERM")
	}
}
