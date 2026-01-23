//go:build e2e

package features

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

// TestCommands_CheckCmd_AllowsOnSuccess verifies that when check_cmd succeeds,
// the destination file is written.
func TestCommands_CheckCmd_AllowsOnSuccess(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("check-success.conf")

	// Write template
	env.WriteTemplate("check.tmpl", `key: {{ getv "/key" }}
check_cmd: success
`)

	// Write config with check_cmd that validates the staged file
	// {{.src}} is the temp file path before it's moved to dest
	env.WriteConfig("check.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "check.tmpl"
dest = "%s"
check_cmd = "/bin/sh -c 'test -f {{.src}} && grep -q \"key:\" {{.src}}'"
keys = ["/key"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "foobar")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify the file was created
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Destination file not created: %v", err)
	}

	expectedContent := "key: foobar\ncheck_cmd: success\n"
	if string(content) != expectedContent {
		t.Errorf("Content mismatch.\nExpected: %q\nGot: %q", expectedContent, string(content))
	}
}

// TestCommands_CheckCmd_BlocksOnFailure verifies that when check_cmd fails,
// the destination file is NOT written.
func TestCommands_CheckCmd_BlocksOnFailure(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("check-fail.conf")

	// Write template
	env.WriteTemplate("check-fail.tmpl", `key: {{ getv "/key" }}
this should not be written
`)

	// Write config with check_cmd that always fails
	env.WriteConfig("check-fail.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "check-fail.tmpl"
dest = "%s"
check_cmd = "/bin/false"
keys = ["/key"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "foobar")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	// Wait for confd to finish (it may return non-zero due to check_cmd failure)
	confd.Wait()

	// Verify the file was NOT created
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		content, _ := os.ReadFile(destPath)
		t.Errorf("Destination file should NOT exist when check_cmd fails, but it does.\nContent: %q", string(content))
	}
}

// TestCommands_ReloadCmd_ExecutesAfterWrite verifies that reload_cmd is executed
// after a successful file write.
func TestCommands_ReloadCmd_ExecutesAfterWrite(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("reload.conf")
	markerPath := filepath.Join(env.DestDir, "reload-marker")

	// Write template
	env.WriteTemplate("reload.tmpl", `key: {{ getv "/key" }}
reload_cmd: executed
`)

	// Write config with reload_cmd that creates a marker file
	env.WriteConfig("reload.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "reload.tmpl"
dest = "%s"
reload_cmd = "/bin/sh -c 'echo reloaded > %s'"
keys = ["/key"]
`, destPath, markerPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "foobar")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify the destination file was created
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatal("Destination file was not created")
	}

	// Verify the reload marker was created (proves reload_cmd executed)
	markerContent, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("Reload marker not created - reload_cmd was not executed: %v", err)
	}

	if strings.TrimSpace(string(markerContent)) != "reloaded" {
		t.Errorf("Reload marker has unexpected content: %q", string(markerContent))
	}
}

// TestCommands_CheckCmd_TemplatePath verifies that {{.src}} in check_cmd
// is substituted with the staged file path.
func TestCommands_CheckCmd_TemplatePath(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("check-path.conf")
	pathMarkerPath := filepath.Join(env.DestDir, "path-marker")

	// Write template with unique content
	env.WriteTemplate("check-path.tmpl", `UNIQUE_MARKER_12345
key: {{ getv "/key" }}
`)

	// Write config with check_cmd that captures the {{.src}} path
	// and verifies it contains our unique marker
	env.WriteConfig("check-path.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "check-path.tmpl"
dest = "%s"
check_cmd = "/bin/sh -c 'grep -q UNIQUE_MARKER_12345 {{.src}} && echo {{.src}} > %s'"
keys = ["/key"]
`, destPath, pathMarkerPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "foobar")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify the path marker was created
	pathContent, err := os.ReadFile(pathMarkerPath)
	if err != nil {
		t.Fatalf("Path marker not created: %v", err)
	}

	// The path should be a temp file path, not the destination
	capturedPath := strings.TrimSpace(string(pathContent))
	if capturedPath == destPath {
		t.Error("{{.src}} should be the staged temp file, not the destination")
	}
	if capturedPath == "" {
		t.Error("{{.src}} substitution failed - path is empty")
	}
	t.Logf("{{.src}} was substituted with: %s", capturedPath)
}

// TestCommands_ReloadCmd_NotExecutedOnCheckFailure verifies that reload_cmd
// is NOT executed when check_cmd fails.
func TestCommands_ReloadCmd_NotExecutedOnCheckFailure(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("no-reload.conf")
	markerPath := filepath.Join(env.DestDir, "no-reload-marker")

	// Write template
	env.WriteTemplate("no-reload.tmpl", `key: {{ getv "/key" }}`)

	// Write config with check_cmd that fails and reload_cmd
	env.WriteConfig("no-reload.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "no-reload.tmpl"
dest = "%s"
check_cmd = "/bin/false"
reload_cmd = "/bin/sh -c 'echo should-not-exist > %s'"
keys = ["/key"]
`, destPath, markerPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "foobar")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	confd.Wait()

	// Verify the reload marker was NOT created
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Error("Reload marker should NOT exist when check_cmd fails - reload_cmd should not have executed")
	}

	// Verify the destination file was NOT created either
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Error("Destination file should NOT exist when check_cmd fails")
	}
}

// TestCommands_BothCheckAndReload verifies that both check_cmd and reload_cmd
// work together correctly.
func TestCommands_BothCheckAndReload(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("both.conf")
	checkMarkerPath := filepath.Join(env.DestDir, "check-marker")
	reloadMarkerPath := filepath.Join(env.DestDir, "reload-marker")

	// Write template
	env.WriteTemplate("both.tmpl", `key: {{ getv "/key" }}
both commands executed
`)

	// Write config with both check_cmd and reload_cmd
	env.WriteConfig("both.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "both.tmpl"
dest = "%s"
check_cmd = "/bin/sh -c 'echo checked > %s && test -f {{.src}}'"
reload_cmd = "/bin/sh -c 'echo reloaded > %s'"
keys = ["/key"]
`, destPath, checkMarkerPath, reloadMarkerPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "foobar")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify destination file was created
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatal("Destination file was not created")
	}

	// Verify check_cmd executed
	checkContent, err := os.ReadFile(checkMarkerPath)
	if err != nil {
		t.Fatalf("Check marker not created - check_cmd was not executed: %v", err)
	}
	if strings.TrimSpace(string(checkContent)) != "checked" {
		t.Errorf("Check marker has unexpected content: %q", string(checkContent))
	}

	// Verify reload_cmd executed
	reloadContent, err := os.ReadFile(reloadMarkerPath)
	if err != nil {
		t.Fatalf("Reload marker not created - reload_cmd was not executed: %v", err)
	}
	if strings.TrimSpace(string(reloadContent)) != "reloaded" {
		t.Errorf("Reload marker has unexpected content: %q", string(reloadContent))
	}
}
