//go:build e2e

package features

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/operations"
)

// TestPerResourceBackend_OverrideGlobal verifies that a template can specify
// its own backend (file) that overrides the global backend (env).
func TestPerResourceBackend_OverrideGlobal(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("secrets.conf")

	// Create file backend data in a subdirectory
	fileBackendDir := filepath.Join(env.BaseDir, "file-backend")
	if err := os.MkdirAll(fileBackendDir, 0755); err != nil {
		t.Fatalf("Failed to create file backend directory: %v", err)
	}

	// Write YAML file for file backend
	yamlPath := filepath.Join(fileBackendDir, "secrets.yaml")
	yamlContent := `secrets:
  database:
    password: super-secret-password
  api:
    key: api-key-12345
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	// Write template that uses keys from file backend
	env.WriteTemplate("secrets.tmpl", `[database]
password = {{ getv "/secrets/database/password" }}

[api]
key = {{ getv "/secrets/api/key" }}
`)

	// Write config with per-resource backend override
	env.WriteConfig("secrets.toml", fmt.Sprintf(`[template]
src = "secrets.tmpl"
dest = "%s"
keys = ["/secrets"]

[backend]
backend = "file"
file = ["%s"]
`, destPath, yamlPath))

	// Run confd with env as global backend (but template uses file backend)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	// Set an env variable that won't be used (global backend is env, but template overrides to file)
	confd.SetEnv("UNUSED_KEY", "unused-value")
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

	// Verify output
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Destination file not created: %v", err)
	}

	expected := `[database]
password = super-secret-password

[api]
key = api-key-12345
`
	if string(content) != expected {
		t.Errorf("Content mismatch.\nExpected:\n%s\nGot:\n%s", expected, string(content))
	}
}

// TestPerResourceBackend_MixedBackends verifies that multiple templates can use
// different backends in a single confd run.
func TestPerResourceBackend_MixedBackends(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	appDestPath := env.DestPath("app.conf")
	secretsDestPath := env.DestPath("secrets.conf")

	// Create file backend data
	fileBackendDir := filepath.Join(env.BaseDir, "file-backend")
	if err := os.MkdirAll(fileBackendDir, 0755); err != nil {
		t.Fatalf("Failed to create file backend directory: %v", err)
	}

	yamlPath := filepath.Join(fileBackendDir, "secrets.yaml")
	yamlContent := `secrets:
  database:
    password: db-password-from-file
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	// Write template that uses env backend (global)
	env.WriteTemplate("app.tmpl", `[app]
name = {{ getv "/app/name" }}
version = {{ getv "/app/version" }}
`)

	// Write template that uses file backend (per-resource override)
	env.WriteTemplate("secrets.tmpl", `[database]
password = {{ getv "/secrets/database/password" }}
`)

	// Write config for app template (uses global env backend - no [backend] section)
	env.WriteConfig("app.toml", fmt.Sprintf(`[template]
src = "app.tmpl"
dest = "%s"
keys = ["/app"]
`, appDestPath))

	// Write config for secrets template (uses file backend via per-resource override)
	env.WriteConfig("secrets.toml", fmt.Sprintf(`[template]
src = "secrets.tmpl"
dest = "%s"
keys = ["/secrets"]

[backend]
backend = "file"
file = ["%s"]
`, secretsDestPath, yamlPath))

	// Run confd with env as global backend
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("APP_NAME", "myapp")
	confd.SetEnv("APP_VERSION", "1.0.0")
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

	// Verify app.conf (from env backend)
	appContent, err := os.ReadFile(appDestPath)
	if err != nil {
		t.Fatalf("app.conf not created: %v", err)
	}

	expectedApp := `[app]
name = myapp
version = 1.0.0
`
	if string(appContent) != expectedApp {
		t.Errorf("app.conf content mismatch.\nExpected:\n%s\nGot:\n%s", expectedApp, string(appContent))
	}

	// Verify secrets.conf (from file backend)
	secretsContent, err := os.ReadFile(secretsDestPath)
	if err != nil {
		t.Fatalf("secrets.conf not created: %v", err)
	}

	expectedSecrets := `[database]
password = db-password-from-file
`
	if string(secretsContent) != expectedSecrets {
		t.Errorf("secrets.conf content mismatch.\nExpected:\n%s\nGot:\n%s", expectedSecrets, string(secretsContent))
	}
}

// TestPerResourceBackend_Precedence verifies that per-resource backend
// configuration takes precedence over global backend for the same data.
func TestPerResourceBackend_Precedence(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("config.conf")

	// Create file backend data with a specific value
	fileBackendDir := filepath.Join(env.BaseDir, "file-backend")
	if err := os.MkdirAll(fileBackendDir, 0755); err != nil {
		t.Fatalf("Failed to create file backend directory: %v", err)
	}

	yamlPath := filepath.Join(fileBackendDir, "config.yaml")
	yamlContent := `config:
  source: file-backend
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	// Write template
	env.WriteTemplate("config.tmpl", `source = {{ getv "/config/source" }}
`)

	// Write config with per-resource backend override
	// Even though we're running with env backend globally, this template uses file
	env.WriteConfig("config.toml", fmt.Sprintf(`[template]
src = "config.tmpl"
dest = "%s"
keys = ["/config"]

[backend]
backend = "file"
file = ["%s"]
`, destPath, yamlPath))

	// Run confd with env as global backend
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	// Set an env variable with the same key path - should NOT be used
	// because per-resource file backend takes precedence
	confd.SetEnv("CONFIG_SOURCE", "env-backend")
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

	// Verify output uses file backend value, not env backend value
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Destination file not created: %v", err)
	}

	expected := "source = file-backend\n"
	if string(content) != expected {
		t.Errorf("Per-resource backend should take precedence.\nExpected: %q\nGot: %q", expected, string(content))
	}
}

// TestPerResourceBackend_Warnings verifies that a [backend] section with a key
// the decoder does not recognise, and with a setting its backend does not use,
// logs a warning naming the resource file, both when processing and with
// --check-config.
func TestPerResourceBackend_Warnings(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("app.conf")

	yamlPath := filepath.Join(env.BaseDir, "data.yaml")
	if err := os.WriteFile(yamlPath, []byte("app:\n  name: demo\n"), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	env.WriteTemplate("app.tmpl", `name = {{ getv "/app/name" }}`+"\n")
	resourcePath := env.WriteConfig("app.toml", fmt.Sprintf(`[template]
src = "app.tmpl"
dest = "%s"
keys = ["/app"]

[backend]
backend = "file"
file = ["%s"]
basic_auth = true
client_ca_keys = "/ca"
`, destPath, yamlPath))

	wantWarnings := []string{
		resourcePath + `: unknown key "client_ca_keys" in [backend] is ignored; use "client_cakeys"`,
		resourcePath + ": --basic-auth has no effect with the file backend",
	}

	binary := operations.BuildConfd(t)
	modes := map[string][]string{
		"onetime":      {"env", "--onetime", "--confdir", env.ConfDir},
		"check-config": {"env", "--check-config", "--confdir", env.ConfDir},
	}
	for mode, args := range modes {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			output, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
			if err != nil {
				t.Fatalf("confd %v failed: %v\n%s", args, err, output)
			}
			for _, want := range wantWarnings {
				if strings.Count(string(output), want) != 1 {
					t.Errorf("want the warning %q exactly once in the output:\n%s", want, output)
				}
			}
		})
	}
}

// TestPerResourceBackend_WarningsInWatchModeWithPolledResource verifies that
// the [backend] warnings are logged exactly once in watch mode when a
// resource's backend cannot watch and the resource is polled instead.
func TestPerResourceBackend_WarningsInWatchModeWithPolledResource(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)

	yamlPath := filepath.Join(env.BaseDir, "data.yaml")
	if err := os.WriteFile(yamlPath, []byte("app:\n  name: demo\n"), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	env.WriteTemplate("app.tmpl", "static\n")
	destPath := env.DestPath("app.conf")
	resourcePath := env.WriteConfig("app.toml", fmt.Sprintf(`[template]
src = "app.tmpl"
dest = "%s"
keys = ["/app"]

[backend]
backend = "env"
basic_auth = true
client_ca_keys = "/ca"
`, destPath))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// The global file backend can watch; the resource's env backend cannot,
	// so confd starts and polls the resource.
	args := []string{"file", "--file", yamlPath, "--watch", "--confdir", env.ConfDir}
	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, operations.BuildConfd(t), args...)
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatalf("start confd %v: %v", args, err)
	}

	// The resources are loaded, and the warnings logged, before the polled
	// resource is first processed.
	renderErr := operations.WaitForFile(t, destPath, 15*time.Second, "static\n")
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal confd: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("confd %v exited with %v after SIGTERM; want a clean stop:\n%s", args, err, output.String())
	}
	if renderErr != nil {
		t.Fatalf("polled resource was not rendered: %v\n%s", renderErr, output.String())
	}
	// confd logs the path with symlinks resolved (/private/var on macOS), so
	// match from the resource path onwards.
	if want := resourcePath + " every 10m0s: env backend cannot watch"; !strings.Contains(output.String(), want) {
		t.Errorf("want %q in the output:\n%s", want, output.String())
	}
	for _, want := range []string{
		resourcePath + `: unknown key "client_ca_keys" in [backend] is ignored; use "client_cakeys"`,
		resourcePath + ": --basic-auth has no effect with the env backend",
	} {
		if strings.Count(output.String(), want) != 1 {
			t.Errorf("want the warning %q exactly once in the output:\n%s", want, output.String())
		}
	}
}
