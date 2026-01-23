//go:build e2e

package resilience

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/operations"
)

// TestTimeout_CheckCmd_EnforcesTimeout verifies that check_cmd times out
// when it exceeds the configured timeout.
func TestTimeout_CheckCmd_EnforcesTimeout(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("timeout.conf")

	// Write template
	env.WriteTemplate("timeout.tmpl", `key: {{ getv "/key" }}
`)

	// Write config with check_cmd that sleeps longer than timeout
	// The command sleeps for 10 seconds, but timeout is 1 second
	env.WriteConfig("timeout.toml", fmt.Sprintf(`[template]
src = "timeout.tmpl"
dest = "%s"
check_cmd = "sleep 10"
check_cmd_timeout = "1s"
keys = ["/key"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	start := time.Now()
	exitCode, err := confd.Wait()
	duration := time.Since(start)
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// The command should timeout after ~1 second, not take 10 seconds
	if duration > 5*time.Second {
		t.Errorf("check_cmd timeout was not enforced. Expected ~1s, took %v", duration)
	}

	// Expect non-zero exit due to check_cmd timeout/failure
	if exitCode == 0 {
		t.Error("Expected non-zero exit code when check_cmd times out")
	}

	// Destination file should NOT be created when check_cmd fails
	if _, err := os.Stat(destPath); err == nil {
		t.Error("Destination file should NOT be created when check_cmd times out")
	}
}

// TestTimeout_ReloadCmd_EnforcesTimeout verifies that reload_cmd times out
// when it exceeds the configured timeout.
func TestTimeout_ReloadCmd_EnforcesTimeout(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("reload-timeout.conf")
	markerPath := filepath.Join(env.DestDir, "reload-started")

	// Write template
	env.WriteTemplate("reload.tmpl", `key: {{ getv "/key" }}
`)

	// Write config with reload_cmd that creates a marker then sleeps longer than timeout
	// The command creates marker, sleeps for 10 seconds, but timeout is 1 second
	env.WriteConfig("reload.toml", fmt.Sprintf(`[template]
src = "reload.tmpl"
dest = "%s"
reload_cmd = "/bin/sh -c 'touch %s && sleep 10'"
reload_cmd_timeout = "1s"
keys = ["/key"]
`, destPath, markerPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	start := time.Now()
	exitCode, err := confd.Wait()
	duration := time.Since(start)
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// The command should timeout after ~1 second, not take 10 seconds
	if duration > 5*time.Second {
		t.Errorf("reload_cmd timeout was not enforced. Expected ~1s, took %v", duration)
	}

	// Note: reload_cmd runs AFTER file is written, so destination should exist
	// even if reload_cmd times out
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Destination file should be created even when reload_cmd times out")
	}

	// The marker file should exist (proves reload_cmd started)
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("Reload marker should exist (command should have started)")
	}

	// Exit code behavior depends on failure mode - just log it
	t.Logf("Exit code when reload_cmd times out: %d", exitCode)
}

// TestTimeout_CheckCmd_GlobalDefault verifies that check_cmd uses
// the global default timeout when not specified per-resource.
func TestTimeout_CheckCmd_GlobalDefault(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("global-timeout.conf")

	// Write template
	env.WriteTemplate("global.tmpl", `key: {{ getv "/key" }}
`)

	// Write config WITHOUT per-resource timeout (will use global)
	env.WriteConfig("global.toml", fmt.Sprintf(`[template]
src = "global.tmpl"
dest = "%s"
check_cmd = "sleep 10"
keys = ["/key"]
`, destPath))

	// Run confd with global check-cmd-timeout of 1 second
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir,
		"--log-level", "error", "--check-cmd-timeout", "1s")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	start := time.Now()
	_, err = confd.Wait()
	duration := time.Since(start)
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// The global timeout should enforce the 1 second limit
	if duration > 5*time.Second {
		t.Errorf("Global check-cmd-timeout was not enforced. Expected ~1s, took %v", duration)
	}

	// Destination should NOT be created
	if _, err := os.Stat(destPath); err == nil {
		t.Error("Destination file should NOT be created when check_cmd times out")
	}
}

// TestTimeout_PerResourceOverridesGlobal verifies that per-resource timeout
// takes precedence over global timeout.
func TestTimeout_PerResourceOverridesGlobal(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("precedence.conf")
	markerPath := filepath.Join(env.DestDir, "cmd-completed")

	// Write template
	env.WriteTemplate("precedence.tmpl", `key: {{ getv "/key" }}
`)

	// Write config with per-resource timeout of 5 seconds
	// The command sleeps for 2 seconds, so it should succeed with per-resource timeout
	// but would fail with global 1s timeout
	env.WriteConfig("precedence.toml", fmt.Sprintf(`[template]
src = "precedence.tmpl"
dest = "%s"
check_cmd = "/bin/sh -c 'sleep 2 && echo done > %s'"
check_cmd_timeout = "5s"
keys = ["/key"]
`, destPath, markerPath))

	// Run confd with global check-cmd-timeout of 1 second
	// Per-resource 5s timeout should override and allow the 2s command to complete
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir,
		"--log-level", "error", "--check-cmd-timeout", "1s")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// Should succeed because per-resource timeout (5s) allows the 2s command to complete
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 (per-resource timeout should override global), got %d", exitCode)
	}

	// Destination file should be created
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Destination file should be created when check_cmd succeeds")
	}

	// Marker should exist (proves command completed)
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Error("Command marker should exist (proves command completed within per-resource timeout)")
	} else if strings.TrimSpace(string(content)) != "done" {
		t.Errorf("Marker has unexpected content: %q", string(content))
	}
}

// TestTimeout_CheckCmd_ZeroMeansNoTimeout verifies that a timeout of 0
// means no timeout is applied.
func TestTimeout_CheckCmd_ZeroMeansNoTimeout(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("no-timeout.conf")
	markerPath := filepath.Join(env.DestDir, "no-timeout-marker")

	// Write template
	env.WriteTemplate("no-timeout.tmpl", `key: {{ getv "/key" }}
`)

	// Write config with a quick command (no real timeout test, just verify 0 works)
	env.WriteConfig("no-timeout.toml", fmt.Sprintf(`[template]
src = "no-timeout.tmpl"
dest = "%s"
check_cmd = "/bin/sh -c 'sleep 1 && echo done > %s'"
check_cmd_timeout = "0s"
keys = ["/key"]
`, destPath, markerPath))

	// Run confd with global timeout of 0 (no timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir,
		"--log-level", "error", "--check-cmd-timeout", "0s")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// Should succeed (no timeout means command can take as long as needed)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with no timeout, got %d", exitCode)
	}

	// Destination file should be created
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Destination file should be created")
	}

	// Marker should exist
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("Marker should exist (proves command completed)")
	}
}

// TestTimeout_MultipleTemplates_IndependentTimeouts verifies that each template
// can have its own timeout settings.
func TestTimeout_MultipleTemplates_IndependentTimeouts(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	fastDestPath := env.DestPath("fast.conf")
	slowDestPath := env.DestPath("slow.conf")
	fastMarkerPath := filepath.Join(env.DestDir, "fast-marker")
	slowMarkerPath := filepath.Join(env.DestDir, "slow-marker")

	// Write templates
	env.WriteTemplate("fast.tmpl", `fast: {{ getv "/fast" }}
`)
	env.WriteTemplate("slow.tmpl", `slow: {{ getv "/slow" }}
`)

	// Fast template with short timeout - command completes quickly
	env.WriteConfig("a-fast.toml", fmt.Sprintf(`[template]
src = "fast.tmpl"
dest = "%s"
check_cmd = "/bin/sh -c 'echo fast > %s'"
check_cmd_timeout = "5s"
keys = ["/fast"]
`, fastDestPath, fastMarkerPath))

	// Slow template with short timeout - command times out
	env.WriteConfig("b-slow.toml", fmt.Sprintf(`[template]
src = "slow.tmpl"
dest = "%s"
check_cmd = "/bin/sh -c 'sleep 10 && echo slow > %s'"
check_cmd_timeout = "1s"
keys = ["/slow"]
`, slowDestPath, slowMarkerPath))

	// Run confd with best-effort mode to process both templates
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("FAST", "fast-value")
	confd.SetEnv("SLOW", "slow-value")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir,
		"--log-level", "error", "--failure-mode", "best-effort")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	start := time.Now()
	_, err = confd.Wait()
	duration := time.Since(start)
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// Total time should be around 1-2 seconds (slow template times out after 1s)
	// Not 10+ seconds (which would indicate timeout wasn't enforced)
	if duration > 5*time.Second {
		t.Errorf("Independent timeouts not working correctly. Expected ~1-2s, took %v", duration)
	}

	// Fast template should succeed
	if _, err := os.Stat(fastDestPath); os.IsNotExist(err) {
		t.Error("Fast template should be created")
	}
	if _, err := os.Stat(fastMarkerPath); os.IsNotExist(err) {
		t.Error("Fast marker should exist")
	}

	// Slow template should NOT be created (check_cmd timed out)
	if _, err := os.Stat(slowDestPath); err == nil {
		t.Error("Slow template should NOT be created (check_cmd timed out)")
	}
	// Slow marker should NOT exist (command was killed before completion)
	if _, err := os.Stat(slowMarkerPath); err == nil {
		t.Error("Slow marker should NOT exist (command was killed)")
	}
}
