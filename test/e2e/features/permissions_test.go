//go:build e2e && !windows

package features

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/operations"
)

// TestPermissions_Mode0644 verifies that mode 0644 creates a file readable by all.
func TestPermissions_Mode0644(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("perm-644.conf")

	// Write template
	env.WriteTemplate("perm-644.tmpl", `key: {{ getv "/key" }}`)

	// Write config with mode 0644
	env.WriteConfig("perm-644.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "perm-644.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	// Verify the file was created with correct permissions
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode().Perm()
	expectedMode := os.FileMode(0644)
	if mode != expectedMode {
		t.Errorf("Expected mode %o, got %o", expectedMode, mode)
	}
}

// TestPermissions_Mode0600 verifies that mode 0600 creates a file readable only by owner.
func TestPermissions_Mode0600(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("perm-600.conf")

	// Write template
	env.WriteTemplate("perm-600.tmpl", `secret: {{ getv "/key" }}`)

	// Write config with mode 0600
	env.WriteConfig("perm-600.toml", fmt.Sprintf(`[template]
mode = "0600"
src = "perm-600.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "secret-value")
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

	// Verify the file was created with correct permissions
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode().Perm()
	expectedMode := os.FileMode(0600)
	if mode != expectedMode {
		t.Errorf("Expected mode %o, got %o", expectedMode, mode)
	}
}

// TestPermissions_Mode0755 verifies that mode 0755 creates an executable file.
func TestPermissions_Mode0755(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("script.sh")

	// Write template for a shell script
	env.WriteTemplate("script.tmpl", `#!/bin/sh
echo "Key is: {{ getv "/key" }}"
`)

	// Write config with mode 0755
	env.WriteConfig("perm-755.toml", fmt.Sprintf(`[template]
mode = "0755"
src = "script.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	// Verify the file was created with correct permissions
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode().Perm()
	expectedMode := os.FileMode(0755)
	if mode != expectedMode {
		t.Errorf("Expected mode %o, got %o", expectedMode, mode)
	}

	// Verify the file is actually executable
	if mode&0111 == 0 {
		t.Error("File should be executable but has no execute bits set")
	}
}

// TestPermissions_MultipleFiles verifies that different files can have different modes.
func TestPermissions_MultipleFiles(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	dest644 := env.DestPath("multi-644.conf")
	dest600 := env.DestPath("multi-600.conf")
	dest755 := env.DestPath("multi-755.sh")

	// Write templates
	env.WriteTemplate("multi.tmpl", `key: {{ getv "/key" }}`)
	env.WriteTemplate("multi-script.tmpl", `#!/bin/sh
echo {{ getv "/key" }}
`)

	// Write configs with different modes
	env.WriteConfig("multi-644.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "multi.tmpl"
dest = "%s"
keys = ["/key"]
`, dest644))

	env.WriteConfig("multi-600.toml", fmt.Sprintf(`[template]
mode = "0600"
src = "multi.tmpl"
dest = "%s"
keys = ["/key"]
`, dest600))

	env.WriteConfig("multi-755.toml", fmt.Sprintf(`[template]
mode = "0755"
src = "multi-script.tmpl"
dest = "%s"
keys = ["/key"]
`, dest755))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	// Verify each file has correct permissions
	testCases := []struct {
		path     string
		expected os.FileMode
		name     string
	}{
		{dest644, 0644, "mode 0644"},
		{dest600, 0600, "mode 0600"},
		{dest755, 0755, "mode 0755"},
	}

	for _, tc := range testCases {
		info, err := os.Stat(tc.path)
		if err != nil {
			t.Errorf("%s: Failed to stat file: %v", tc.name, err)
			continue
		}

		mode := info.Mode().Perm()
		if mode != tc.expected {
			t.Errorf("%s: Expected mode %o, got %o", tc.name, tc.expected, mode)
		}
	}
}

// TestPermissions_DefaultMode verifies that files without explicit mode get a default.
func TestPermissions_DefaultMode(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("default-mode.conf")

	// Write template
	env.WriteTemplate("default.tmpl", `key: {{ getv "/key" }}`)

	// Write config WITHOUT mode (should use default)
	env.WriteConfig("default.toml", fmt.Sprintf(`[template]
src = "default.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	// Default mode should be 0644
	mode := info.Mode().Perm()
	expectedMode := os.FileMode(0644)
	if mode != expectedMode {
		t.Errorf("Expected default mode %o, got %o", expectedMode, mode)
	}
}
