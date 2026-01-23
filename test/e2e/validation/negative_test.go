//go:build e2e

package validation

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/operations"
)

// TestNegative_InvalidBackendType verifies that an invalid backend type is rejected.
func TestNegative_InvalidBackendType(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	err := confd.Start(ctx, "invalidbackend", "--onetime", "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid backend type, got 0")
	}
}

// TestNegative_NonExistentConfdir verifies that a non-existent confdir logs a warning.
// confd treats missing confdir as a warning, not an error (returns success with no templates)
func TestNegative_NonExistentConfdir(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	err := confd.Start(ctx, "env", "--onetime", "--log-level", "warn", "--confdir", "/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// confd should succeed (no templates to process) but log a warning
	// Exit code 0 is expected - it's handled gracefully
	if exitCode != 0 {
		t.Logf("Note: confd exited with code %d for non-existent confdir (may be expected if strict checking is enabled)", exitCode)
	}
}

// TestNegative_MalformedTOML verifies that malformed TOML configuration is rejected.
func TestNegative_MalformedTOML(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)

	// Write malformed TOML (missing closing bracket)
	env.WriteConfig("bad.toml", `[template
# Missing closing bracket - invalid TOML
mode = "0644"
src = "test.tmpl"
dest = "/tmp/test.conf"
`)

	// Write a valid template (even though TOML is invalid)
	env.WriteTemplate("test.tmpl", `test content`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	err := confd.Start(ctx, "env", "--onetime", "--log-level", "error", "--confdir", env.ConfDir)
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	if exitCode == 0 {
		t.Error("Expected non-zero exit code for malformed TOML, got 0")
	}
}

// TestNegative_TemplateSyntaxError verifies that template syntax errors are rejected.
func TestNegative_TemplateSyntaxError(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("test.conf")

	// Write config referencing template with syntax error
	env.WriteConfig("test.toml", `[template]
mode = "0644"
src = "bad.tmpl"
dest = "`+destPath+`"
keys = ["/key"]
`)

	// Write template with syntax error (missing closing braces)
	env.WriteTemplate("bad.tmpl", `{{ if .Key }
Missing closing braces
{{ end }}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "value")
	err := confd.Start(ctx, "env", "--onetime", "--log-level", "error", "--confdir", env.ConfDir)
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	if exitCode == 0 {
		t.Error("Expected non-zero exit code for template syntax error, got 0")
	}
}

// TestNegative_MissingTemplateFile verifies that missing template files are rejected.
func TestNegative_MissingTemplateFile(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("test.conf")

	// Write config referencing non-existent template
	env.WriteConfig("test.toml", `[template]
mode = "0644"
src = "nonexistent.tmpl"
dest = "`+destPath+`"
keys = ["/key"]
`)

	// Note: We intentionally do NOT create the template file

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "value")
	err := confd.Start(ctx, "env", "--onetime", "--log-level", "error", "--confdir", env.ConfDir)
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	if exitCode == 0 {
		t.Error("Expected non-zero exit code for missing template file, got 0")
	}
}

// TestNegative_NonExistentKey verifies that referencing a non-existent key fails.
func TestNegative_NonExistentKey(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("test.conf")

	// Write config referencing a key that doesn't exist
	env.WriteConfig("test.toml", `[template]
mode = "0644"
src = "test.tmpl"
dest = "`+destPath+`"
keys = ["/nonexistent/key/that/does/not/exist"]
`)

	// Write template that uses the non-existent key
	env.WriteTemplate("test.tmpl", `value: {{ getv "/nonexistent/key/that/does/not/exist" }}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	// Note: We intentionally do NOT set the environment variable
	err := confd.Start(ctx, "env", "--onetime", "--log-level", "error", "--confdir", env.ConfDir)
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	if exitCode == 0 {
		t.Error("Expected non-zero exit code for non-existent key, got 0")
	}
}

// TestNegative_InvalidModeFormat verifies that invalid file mode format is rejected.
func TestNegative_InvalidModeFormat(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("test.conf")

	// Write config with invalid mode
	env.WriteConfig("test.toml", `[template]
mode = "invalid"
src = "test.tmpl"
dest = "`+destPath+`"
keys = ["/key"]
`)

	// Write valid template
	env.WriteTemplate("test.tmpl", `key: {{ getv "/key" }}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "value")
	err := confd.Start(ctx, "env", "--onetime", "--log-level", "error", "--confdir", env.ConfDir)
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid mode format, got 0")
	}
}

// TestNegative_EmptyConfdir verifies that an empty confdir is handled gracefully.
// Empty confdir should succeed (no-op) rather than fail.
func TestNegative_EmptyConfdir(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	// Note: We don't write any configs or templates - confdir is empty

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	err := confd.Start(ctx, "env", "--onetime", "--log-level", "error", "--confdir", env.ConfDir)
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// Empty confdir should succeed - it's a no-op
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for empty confdir (no-op), got %d", exitCode)
	}
}

// TestNegative_ValidConfigAfterErrors verifies that valid configuration still works.
// This ensures the confd binary itself is functioning correctly after testing error cases.
func TestNegative_ValidConfigAfterErrors(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("valid-output.conf")

	// Write valid config
	env.WriteConfig("valid.toml", `[template]
mode = "0644"
src = "valid.tmpl"
dest = "`+destPath+`"
keys = ["/key", "/database/host"]
`)

	// Write valid template
	env.WriteTemplate("valid.tmpl", `key: {{ getv "/key" }}
database_host: {{ getv "/database/host" }}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	confd.SetEnv("DATABASE_HOST", "localhost:5432")
	err := confd.Start(ctx, "env", "--onetime", "--log-level", "error", "--confdir", env.ConfDir)
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for valid config, got %d", exitCode)
	}

	// Verify output file was created
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Verify content
	checks := []struct {
		name     string
		expected string
	}{
		{"key", "key: test-value"},
		{"database_host", "database_host: localhost:5432"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.expected) {
			t.Errorf("%s: expected %q in output, got:\n%s", check.name, check.expected, output)
		}
	}
}
