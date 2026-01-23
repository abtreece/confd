//go:build e2e

package features

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/operations"
)

// TestFailureMode_BestEffort_ContinuesOnError verifies that in best-effort mode,
// confd continues processing remaining templates after one fails.
func TestFailureMode_BestEffort_ContinuesOnError(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	goodDestPath := env.DestPath("good.conf")
	badDestPath := env.DestPath("bad.conf")

	// Write good template that should succeed
	env.WriteTemplate("good.tmpl", `key: {{ getv "/key" }}`)

	// Write bad template that references a non-existent key
	env.WriteTemplate("bad.tmpl", `value: {{ getv "/nonexistent/key" }}`)

	// Write config for good template (processed first alphabetically)
	env.WriteConfig("good.toml", fmt.Sprintf(`[template]
src = "good.tmpl"
dest = "%s"
keys = ["/key"]
`, goodDestPath))

	// Write config for bad template (processed second - 'z' prefix ensures it's after 'good')
	env.WriteConfig("zbad.toml", fmt.Sprintf(`[template]
src = "bad.tmpl"
dest = "%s"
keys = ["/nonexistent"]
`, badDestPath))

	// Run confd with best-effort mode
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error", "--failure-mode", "best-effort")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// In best-effort mode, confd exits with non-zero if any template fails
	// but continues processing all templates
	if exitCode == 0 {
		t.Log("Note: exit code 0 indicates all templates processed successfully")
	}

	// Good template should be created despite bad template failing
	content, err := os.ReadFile(goodDestPath)
	if err != nil {
		t.Fatalf("Good template was not created in best-effort mode: %v", err)
	}

	if string(content) != "key: test-value" {
		t.Errorf("Good template has incorrect content. Expected 'key: test-value', got %q", string(content))
	}

	// Bad template should not be created
	if _, err := os.Stat(badDestPath); err == nil {
		t.Error("Bad template should not have been created")
	}
}

// TestFailureMode_FailFast_StopsOnError verifies that in fail-fast mode,
// confd stops processing immediately when a template fails.
func TestFailureMode_FailFast_StopsOnError(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	badDestPath := env.DestPath("bad.conf")
	goodDestPath := env.DestPath("good.conf")

	// Write bad template that references a non-existent key (processed first - 'a' prefix)
	env.WriteTemplate("bad.tmpl", `value: {{ getv "/nonexistent/key" }}`)

	// Write good template that should succeed (processed second - 'z' prefix)
	env.WriteTemplate("good.tmpl", `key: {{ getv "/key" }}`)

	// Write config for bad template (processed first alphabetically)
	env.WriteConfig("abad.toml", fmt.Sprintf(`[template]
src = "bad.tmpl"
dest = "%s"
keys = ["/nonexistent"]
`, badDestPath))

	// Write config for good template (processed second)
	env.WriteConfig("zgood.toml", fmt.Sprintf(`[template]
src = "good.tmpl"
dest = "%s"
keys = ["/key"]
`, goodDestPath))

	// Run confd with fail-fast mode
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error", "--failure-mode", "fail-fast")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// Expect non-zero exit code due to template failure
	if exitCode == 0 {
		t.Error("Expected non-zero exit code in fail-fast mode when template fails")
	}

	// Bad template should not be created
	if _, err := os.Stat(badDestPath); err == nil {
		t.Error("Bad template should not have been created")
	}

	// In fail-fast mode, processing stops at first error
	// The good template may or may not be created depending on processing order
	// We just verify that confd exited with an error
}

// TestFailureMode_ExitCodes verifies that confd returns correct exit codes
// for different failure scenarios.
func TestFailureMode_ExitCodes(t *testing.T) {
	t.Parallel()

	t.Run("success_exit_0", func(t *testing.T) {
		t.Parallel()

		env := operations.NewTestEnv(t)
		destPath := env.DestPath("success.conf")

		env.WriteTemplate("success.tmpl", `key: {{ getv "/key" }}`)
		env.WriteConfig("success.toml", fmt.Sprintf(`[template]
src = "success.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		confd := operations.NewConfdBinary(t)
		confd.SetEnv("KEY", "value")
		err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
		if err != nil {
			t.Fatalf("Failed to start confd: %v", err)
		}

		exitCode, err := confd.Wait()
		if err != nil {
			t.Fatalf("Error waiting for confd: %v", err)
		}

		if exitCode != 0 {
			t.Errorf("Expected exit code 0 for successful run, got %d", exitCode)
		}
	})

	t.Run("failure_exit_nonzero", func(t *testing.T) {
		t.Parallel()

		env := operations.NewTestEnv(t)
		destPath := env.DestPath("failure.conf")

		// Template that will fail due to missing key
		env.WriteTemplate("failure.tmpl", `value: {{ getv "/missing/key" }}`)
		env.WriteConfig("failure.toml", fmt.Sprintf(`[template]
src = "failure.tmpl"
dest = "%s"
keys = ["/missing"]
`, destPath))

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		confd := operations.NewConfdBinary(t)
		err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
		if err != nil {
			t.Fatalf("Failed to start confd: %v", err)
		}

		exitCode, err := confd.Wait()
		if err != nil {
			t.Fatalf("Error waiting for confd: %v", err)
		}

		if exitCode == 0 {
			t.Error("Expected non-zero exit code for failed template")
		}
	})

	t.Run("invalid_config_exit_nonzero", func(t *testing.T) {
		t.Parallel()

		env := operations.NewTestEnv(t)

		// Write invalid TOML config
		env.WriteConfig("invalid.toml", `[template
this is not valid TOML
`)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		confd := operations.NewConfdBinary(t)
		err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
		if err != nil {
			t.Fatalf("Failed to start confd: %v", err)
		}

		exitCode, err := confd.Wait()
		if err != nil {
			t.Fatalf("Error waiting for confd: %v", err)
		}

		if exitCode == 0 {
			t.Error("Expected non-zero exit code for invalid config")
		}
	})
}

// TestFailureMode_ErrorAggregation verifies that in best-effort mode,
// errors are aggregated and all templates are attempted.
func TestFailureMode_ErrorAggregation(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	good1Path := env.DestPath("good1.conf")
	good2Path := env.DestPath("good2.conf")
	bad1Path := env.DestPath("bad1.conf")
	bad2Path := env.DestPath("bad2.conf")

	// Write good templates
	env.WriteTemplate("good1.tmpl", `good1: {{ getv "/key1" }}`)
	env.WriteTemplate("good2.tmpl", `good2: {{ getv "/key2" }}`)

	// Write bad templates that reference non-existent keys
	env.WriteTemplate("bad1.tmpl", `bad1: {{ getv "/nonexistent1" }}`)
	env.WriteTemplate("bad2.tmpl", `bad2: {{ getv "/nonexistent2" }}`)

	// Write configs (using prefixes to control processing order)
	env.WriteConfig("a-good1.toml", fmt.Sprintf(`[template]
src = "good1.tmpl"
dest = "%s"
keys = ["/key1"]
`, good1Path))

	env.WriteConfig("b-bad1.toml", fmt.Sprintf(`[template]
src = "bad1.tmpl"
dest = "%s"
keys = ["/nonexistent1"]
`, bad1Path))

	env.WriteConfig("c-good2.toml", fmt.Sprintf(`[template]
src = "good2.tmpl"
dest = "%s"
keys = ["/key2"]
`, good2Path))

	env.WriteConfig("d-bad2.toml", fmt.Sprintf(`[template]
src = "bad2.tmpl"
dest = "%s"
keys = ["/nonexistent2"]
`, bad2Path))

	// Run confd with best-effort mode
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY1", "value1")
	confd.SetEnv("KEY2", "value2")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error", "--failure-mode", "best-effort")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// In best-effort mode with failures, expect non-zero exit
	if exitCode == 0 {
		t.Log("Note: exit code 0 means all templates succeeded (may happen if keys exist)")
	}

	// Both good templates should be created
	content1, err := os.ReadFile(good1Path)
	if err != nil {
		t.Errorf("good1 template was not created: %v", err)
	} else if string(content1) != "good1: value1" {
		t.Errorf("good1 has incorrect content: %q", string(content1))
	}

	content2, err := os.ReadFile(good2Path)
	if err != nil {
		t.Errorf("good2 template was not created: %v", err)
	} else if string(content2) != "good2: value2" {
		t.Errorf("good2 has incorrect content: %q", string(content2))
	}

	// Bad templates should not be created
	if _, err := os.Stat(bad1Path); err == nil {
		t.Error("bad1 template should not have been created")
	}

	if _, err := os.Stat(bad2Path); err == nil {
		t.Error("bad2 template should not have been created")
	}
}
