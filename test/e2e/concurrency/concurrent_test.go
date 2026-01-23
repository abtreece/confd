//go:build e2e

package concurrency

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/operations"
)

// TestConcurrent_ManyTemplatesProcessing verifies that confd can handle
// many templates processing in a single run without issues.
func TestConcurrent_ManyTemplatesProcessing(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	templateCount := 20

	// Create many templates and configs
	destPaths := make([]string, templateCount)
	for i := 0; i < templateCount; i++ {
		destPaths[i] = env.DestPath(fmt.Sprintf("template-%d.conf", i))

		// Each template uses a unique key
		env.WriteTemplate(fmt.Sprintf("template-%d.tmpl", i),
			fmt.Sprintf(`template-%d: {{ getv "/key%d" }}
timestamp: {{ getenv "TIMESTAMP" }}
`, i, i))

		env.WriteConfig(fmt.Sprintf("template-%d.toml", i), fmt.Sprintf(`[template]
src = "template-%d.tmpl"
dest = "%s"
keys = ["/key%d"]
`, i, destPaths[i], i))
	}

	// Run confd with all templates
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	// Set environment variables for all templates
	for i := 0; i < templateCount; i++ {
		confd.SetEnv(fmt.Sprintf("KEY%d", i), fmt.Sprintf("value-%d", i))
	}
	confd.SetEnv("TIMESTAMP", "2024-01-01")

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

	// Verify all templates were processed correctly
	for i := 0; i < templateCount; i++ {
		content, err := os.ReadFile(destPaths[i])
		if err != nil {
			t.Errorf("Template %d not created: %v", i, err)
			continue
		}

		expected := fmt.Sprintf("template-%d: value-%d\ntimestamp: 2024-01-01\n", i, i)
		if string(content) != expected {
			t.Errorf("Template %d has incorrect content.\nExpected: %q\nGot: %q", i, expected, string(content))
		}
	}
}

// TestConcurrent_ParallelConfdInstances verifies that multiple confd instances
// can run in parallel without interfering with each other.
func TestConcurrent_ParallelConfdInstances(t *testing.T) {
	t.Parallel()

	instanceCount := 5
	var wg sync.WaitGroup
	errors := make(chan error, instanceCount)

	for i := 0; i < instanceCount; i++ {
		wg.Add(1)
		go func(instanceID int) {
			defer wg.Done()

			// Each instance has its own test environment
			env := operations.NewTestEnv(t)
			destPath := env.DestPath("output.conf")

			env.WriteTemplate("output.tmpl", fmt.Sprintf(`instance: %d
key: {{ getv "/key" }}
`, instanceID))

			env.WriteConfig("output.toml", fmt.Sprintf(`[template]
src = "output.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			confd := operations.NewConfdBinary(t)
			confd.SetEnv("KEY", fmt.Sprintf("value-from-instance-%d", instanceID))

			err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
			if err != nil {
				errors <- fmt.Errorf("instance %d: failed to start: %w", instanceID, err)
				return
			}

			exitCode, err := confd.Wait()
			if err != nil {
				errors <- fmt.Errorf("instance %d: wait error: %w", instanceID, err)
				return
			}
			if exitCode != 0 {
				errors <- fmt.Errorf("instance %d: exit code %d", instanceID, exitCode)
				return
			}

			// Verify output
			content, err := os.ReadFile(destPath)
			if err != nil {
				errors <- fmt.Errorf("instance %d: output not created: %w", instanceID, err)
				return
			}

			expected := fmt.Sprintf("instance: %d\nkey: value-from-instance-%d\n", instanceID, instanceID)
			if string(content) != expected {
				errors <- fmt.Errorf("instance %d: incorrect content: %q", instanceID, string(content))
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	var errs []string
	for err := range errors {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		t.Errorf("Parallel instances had errors:\n%s", strings.Join(errs, "\n"))
	}
}

// TestConcurrent_RapidTemplateUpdates verifies that confd handles rapid
// sequential template processing correctly in interval mode.
func TestConcurrent_RapidTemplateUpdates(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("rapid.conf")

	env.WriteTemplate("rapid.tmpl", `version: {{ getv "/version" }}
`)

	env.WriteConfig("rapid.toml", fmt.Sprintf(`[template]
src = "rapid.tmpl"
dest = "%s"
keys = ["/version"]
`, destPath))

	// Start confd in interval mode with short interval
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("VERSION", "1")
	err := confd.Start(ctx, "env", "--interval", "1", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for initial file
	if err := operations.WaitForFile(t, destPath, 10*time.Second, "version: 1\n"); err != nil {
		t.Fatalf("Initial file not created: %v", err)
	}

	// Rapidly check that confd continues to run and process templates
	// (In interval mode with env backend, the value won't change since env vars
	// are set at process start, but we verify stability under rapid polling)
	for i := 0; i < 5; i++ {
		time.Sleep(1500 * time.Millisecond) // Slightly more than interval

		if !confd.IsRunning() {
			t.Fatalf("confd stopped unexpectedly after %d intervals", i+1)
		}

		content, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("File missing after %d intervals: %v", i+1, err)
		}
		if string(content) != "version: 1\n" {
			t.Errorf("Content changed unexpectedly after %d intervals: %q", i+1, string(content))
		}
	}
}

// TestConcurrent_TemplatesWithSharedKeys verifies that multiple templates
// can use the same keys without interference.
func TestConcurrent_TemplatesWithSharedKeys(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath1 := env.DestPath("shared1.conf")
	destPath2 := env.DestPath("shared2.conf")
	destPath3 := env.DestPath("shared3.conf")

	// Three templates all using the same shared key
	env.WriteTemplate("shared1.tmpl", `[service1]
database = {{ getv "/database/host" }}
port = {{ getv "/database/port" }}
`)

	env.WriteTemplate("shared2.tmpl", `[service2]
db_host = {{ getv "/database/host" }}
db_port = {{ getv "/database/port" }}
`)

	env.WriteTemplate("shared3.tmpl", `# Service 3 Config
DATABASE_URL={{ getv "/database/host" }}:{{ getv "/database/port" }}
`)

	env.WriteConfig("shared1.toml", fmt.Sprintf(`[template]
src = "shared1.tmpl"
dest = "%s"
keys = ["/database"]
`, destPath1))

	env.WriteConfig("shared2.toml", fmt.Sprintf(`[template]
src = "shared2.tmpl"
dest = "%s"
keys = ["/database"]
`, destPath2))

	env.WriteConfig("shared3.toml", fmt.Sprintf(`[template]
src = "shared3.tmpl"
dest = "%s"
keys = ["/database"]
`, destPath3))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("DATABASE_HOST", "db.example.com")
	confd.SetEnv("DATABASE_PORT", "5432")

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

	// Verify all three templates have correct content
	content1, err := os.ReadFile(destPath1)
	if err != nil {
		t.Fatalf("shared1.conf not created: %v", err)
	}
	expected1 := "[service1]\ndatabase = db.example.com\nport = 5432\n"
	if string(content1) != expected1 {
		t.Errorf("shared1.conf incorrect.\nExpected: %q\nGot: %q", expected1, string(content1))
	}

	content2, err := os.ReadFile(destPath2)
	if err != nil {
		t.Fatalf("shared2.conf not created: %v", err)
	}
	expected2 := "[service2]\ndb_host = db.example.com\ndb_port = 5432\n"
	if string(content2) != expected2 {
		t.Errorf("shared2.conf incorrect.\nExpected: %q\nGot: %q", expected2, string(content2))
	}

	content3, err := os.ReadFile(destPath3)
	if err != nil {
		t.Fatalf("shared3.conf not created: %v", err)
	}
	expected3 := "# Service 3 Config\nDATABASE_URL=db.example.com:5432\n"
	if string(content3) != expected3 {
		t.Errorf("shared3.conf incorrect.\nExpected: %q\nGot: %q", expected3, string(content3))
	}
}

// TestConcurrent_TemplateCacheConsistency verifies that the template cache
// remains consistent when templates are processed multiple times.
func TestConcurrent_TemplateCacheConsistency(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("cached.conf")
	templatePath := filepath.Join(env.ConfDir, "templates", "cached.tmpl")

	// Write initial template
	if err := os.WriteFile(templatePath, []byte(`version: 1
key: {{ getv "/key" }}
`), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	env.WriteConfig("cached.toml", fmt.Sprintf(`[template]
src = "cached.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

	// Run confd multiple times, modifying template between runs
	for version := 1; version <= 3; version++ {
		// Update template content
		newContent := fmt.Sprintf(`version: %d
key: {{ getv "/key" }}
`, version)
		if err := os.WriteFile(templatePath, []byte(newContent), 0644); err != nil {
			t.Fatalf("Failed to update template to version %d: %v", version, err)
		}

		// Small delay to ensure mtime changes (some filesystems have 1s resolution)
		time.Sleep(100 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		confd := operations.NewConfdBinary(t)
		confd.SetEnv("KEY", fmt.Sprintf("value-v%d", version))

		err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
		if err != nil {
			cancel()
			t.Fatalf("Version %d: Failed to start confd: %v", version, err)
		}

		exitCode, err := confd.Wait()
		cancel()
		if err != nil {
			t.Fatalf("Version %d: Error waiting for confd: %v", version, err)
		}
		if exitCode != 0 {
			t.Errorf("Version %d: Expected exit code 0, got %d", version, exitCode)
		}

		// Verify output reflects the new template version
		content, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("Version %d: Output not created: %v", version, err)
		}

		expected := fmt.Sprintf("version: %d\nkey: value-v%d\n", version, version)
		if string(content) != expected {
			t.Errorf("Version %d: Incorrect content.\nExpected: %q\nGot: %q", version, expected, string(content))
		}
	}
}

// TestConcurrent_LargeTemplateOutput verifies that confd handles templates
// producing large output files correctly.
func TestConcurrent_LargeTemplateOutput(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("large.conf")

	// Create a template that generates a large output using range
	// Each line is about 25 bytes, 1000 lines ≈ 25KB
	env.WriteTemplate("large.tmpl", `# Large configuration file
{{ $prefix := getv "/prefix" }}
{{ range $i := seq 1 1000 }}
line-{{ $i }}: {{ $prefix }}-value-{{ $i }}
{{ end }}
`)

	env.WriteConfig("large.toml", fmt.Sprintf(`[template]
src = "large.tmpl"
dest = "%s"
keys = ["/prefix"]
`, destPath))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("PREFIX", "test")

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

	// Verify output exists and has expected characteristics
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Large output not created: %v", err)
	}

	// Should be at least 20KB total for approximately 1000 lines of output
	if info.Size() < 20000 {
		t.Errorf("Output file too small: %d bytes (expected >20000)", info.Size())
	}

	// Spot check content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	// Check first and last lines
	if !strings.Contains(string(content), "line-1: test-value-1") {
		t.Error("Output missing expected first line")
	}
	if !strings.Contains(string(content), "line-1000: test-value-1000") {
		t.Error("Output missing expected last line")
	}
}

// TestConcurrent_FileBackendWatchMode verifies concurrent template processing
// with file backend in watch mode.
func TestConcurrent_FileBackendWatchMode(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("watch-file.conf")

	// Create file backend data directory
	dataDir := filepath.Join(env.BaseDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	dataFile := filepath.Join(dataDir, "config.yaml")
	initialData := `config:
  version: 1
  name: initial
`
	if err := os.WriteFile(dataFile, []byte(initialData), 0644); err != nil {
		t.Fatalf("Failed to write initial data: %v", err)
	}

	// Write template
	env.WriteTemplate("watch-file.tmpl", `version: {{ getv "/config/version" }}
name: {{ getv "/config/name" }}
`)

	// Write config using file backend
	env.WriteConfig("watch-file.toml", fmt.Sprintf(`[template]
src = "watch-file.tmpl"
dest = "%s"
keys = ["/config"]

[backend]
backend = "file"
file = ["%s"]
`, destPath, dataFile))

	// Start confd in watch mode with file backend
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := operations.NewConfdBinary(t)
	err := confd.Start(ctx, "file", "--watch", "--confdir", env.ConfDir, "--log-level", "error",
		"--file", dataFile)
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for initial file
	if err := operations.WaitForFile(t, destPath, 10*time.Second, "version: 1\nname: initial\n"); err != nil {
		t.Fatalf("Initial file not created: %v", err)
	}

	// Update data file multiple times rapidly
	for version := 2; version <= 4; version++ {
		newData := fmt.Sprintf(`config:
  version: %d
  name: update-%d
`, version, version)
		if err := os.WriteFile(dataFile, []byte(newData), 0644); err != nil {
			t.Fatalf("Failed to update data to version %d: %v", version, err)
		}

		// Wait for change to be detected and processed
		expected := fmt.Sprintf("version: %d\nname: update-%d\n", version, version)
		if err := operations.WaitForFile(t, destPath, 10*time.Second, expected); err != nil {
			t.Fatalf("Version %d not processed: %v", version, err)
		}

		t.Logf("Successfully detected and processed version %d", version)
	}

	// Verify process is still running
	if !confd.IsRunning() {
		t.Error("confd should continue running after multiple file changes")
	}
}
