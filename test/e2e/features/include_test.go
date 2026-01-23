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

// TestInclude_BasicInclude verifies that a template can include another template file.
func TestInclude_BasicInclude(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("basic-include.txt")

	// Write the included template (header)
	env.WriteTemplate("header.tmpl", `===================================
Title: {{ .title }}
===================================
`)

	// Write main template that includes header
	env.WriteTemplate("main.tmpl", `{{ include "header.tmpl" (map "title" (getv "/title")) }}
Content from main template.
`)

	// Write config
	env.WriteConfig("include.toml", fmt.Sprintf(`[template]
src = "main.tmpl"
dest = "%s"
keys = ["/title"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("TITLE", "Test Include")
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
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Check that header was included
	if !strings.Contains(output, "Title: Test Include") {
		t.Errorf("Expected included header with title, got:\n%s", output)
	}

	// Check that main content is present
	if !strings.Contains(output, "Content from main template.") {
		t.Errorf("Expected main template content, got:\n%s", output)
	}
}

// TestInclude_SubdirectoryInclude verifies that templates can include files from subdirectories.
func TestInclude_SubdirectoryInclude(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("subdir-include.txt")

	// Create partials subdirectory
	partialsDir := filepath.Join(env.ConfDir, "templates", "partials")
	if err := os.MkdirAll(partialsDir, 0755); err != nil {
		t.Fatalf("Failed to create partials directory: %v", err)
	}

	// Write the included template in subdirectory
	footerPath := filepath.Join(partialsDir, "footer.tmpl")
	footerContent := `-----------------------------------
Footer: {{ .text }}
-----------------------------------
`
	if err := os.WriteFile(footerPath, []byte(footerContent), 0644); err != nil {
		t.Fatalf("Failed to write footer template: %v", err)
	}

	// Write main template that includes from subdirectory
	env.WriteTemplate("main.tmpl", `Main content here.

{{ include "partials/footer.tmpl" (map "text" (getv "/footer")) }}`)

	// Write config
	env.WriteConfig("include.toml", fmt.Sprintf(`[template]
src = "main.tmpl"
dest = "%s"
keys = ["/footer"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("FOOTER", "End of document")
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
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Check that footer was included from subdirectory
	if !strings.Contains(output, "Footer: End of document") {
		t.Errorf("Expected included footer, got:\n%s", output)
	}

	if !strings.Contains(output, "Main content here.") {
		t.Errorf("Expected main content, got:\n%s", output)
	}
}

// TestInclude_NestedInclude verifies that included templates can include other templates.
func TestInclude_NestedInclude(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("nested-include.txt")

	// Write the innermost template
	env.WriteTemplate("inner.tmpl", `[Inner: {{ .value }}]`)

	// Write middle template that includes inner
	env.WriteTemplate("middle.tmpl", `{Middle: {{ include "inner.tmpl" (map "value" .nested) }}}`)

	// Write outer/main template that includes middle
	env.WriteTemplate("outer.tmpl", `Outer: {{ include "middle.tmpl" (map "nested" (getv "/nested")) }}`)

	// Write config
	env.WriteConfig("nested.toml", fmt.Sprintf(`[template]
src = "outer.tmpl"
dest = "%s"
keys = ["/nested"]
`, destPath))

	// Run confd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("NESTED", "deeply-nested-value")
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
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Check nested includes worked
	if !strings.Contains(output, "[Inner: deeply-nested-value]") {
		t.Errorf("Expected nested inner value, got:\n%s", output)
	}

	if !strings.Contains(output, "{Middle:") {
		t.Errorf("Expected middle wrapper, got:\n%s", output)
	}

	if !strings.Contains(output, "Outer:") {
		t.Errorf("Expected outer wrapper, got:\n%s", output)
	}
}

// TestInclude_CycleDetection verifies that circular includes are detected and prevented.
func TestInclude_CycleDetection(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("cycle.txt")

	// Write template A that includes B
	env.WriteTemplate("a.tmpl", `A includes B: {{ include "b.tmpl" . }}`)

	// Write template B that includes A (creating a cycle)
	env.WriteTemplate("b.tmpl", `B includes A: {{ include "a.tmpl" . }}`)

	// Write main template that starts the cycle
	env.WriteTemplate("main.tmpl", `Start: {{ include "a.tmpl" (map "key" (getv "/key")) }}`)

	// Write config
	env.WriteConfig("cycle.toml", fmt.Sprintf(`[template]
src = "main.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

	// Run confd - should fail due to cycle detection
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// Expect non-zero exit code due to cycle detection
	if exitCode == 0 {
		t.Error("Expected non-zero exit code due to include cycle, but got 0")
	}

	// Destination file should not be created
	if _, err := os.Stat(destPath); err == nil {
		t.Error("Destination file should not be created when include cycle is detected")
	}
}

// TestInclude_MaxDepth verifies that maximum include depth (10) is enforced.
func TestInclude_MaxDepth(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("maxdepth.txt")

	// Create 12 templates that each include the next one (exceeds max depth of 10)
	for i := 1; i <= 12; i++ {
		var content string
		if i == 12 {
			// Last template doesn't include anything
			content = fmt.Sprintf("Level %d (end)", i)
		} else {
			content = fmt.Sprintf("Level %d -> {{ include \"level%d.tmpl\" . }}", i, i+1)
		}
		env.WriteTemplate(fmt.Sprintf("level%d.tmpl", i), content)
	}

	// Write main template that starts the chain
	env.WriteTemplate("main.tmpl", `Start: {{ include "level1.tmpl" (map "key" (getv "/key")) }}`)

	// Write config
	env.WriteConfig("maxdepth.toml", fmt.Sprintf(`[template]
src = "main.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

	// Run confd - should fail due to max depth exceeded
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// Expect non-zero exit code due to max depth exceeded
	if exitCode == 0 {
		t.Error("Expected non-zero exit code due to max include depth exceeded, but got 0")
	}

	// Destination file should not be created
	if _, err := os.Stat(destPath); err == nil {
		t.Error("Destination file should not be created when max depth is exceeded")
	}
}

// TestInclude_MissingTemplate verifies that missing include files are handled gracefully.
func TestInclude_MissingTemplate(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	destPath := env.DestPath("missing.txt")

	// Write main template that tries to include a non-existent file
	env.WriteTemplate("main.tmpl", `Before: {{ include "nonexistent.tmpl" . }}`)

	// Write config
	env.WriteConfig("missing.toml", fmt.Sprintf(`[template]
src = "main.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

	// Run confd - should fail due to missing include
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	confd := operations.NewConfdBinary(t)
	confd.SetEnv("KEY", "test")
	err := confd.Start(ctx, "env", "--onetime", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}

	exitCode, err := confd.Wait()
	if err != nil {
		t.Fatalf("Error waiting for confd: %v", err)
	}

	// Expect non-zero exit code due to missing template
	if exitCode == 0 {
		t.Error("Expected non-zero exit code due to missing include file, but got 0")
	}

	// Destination file should not be created
	if _, err := os.Stat(destPath); err == nil {
		t.Error("Destination file should not be created when include file is missing")
	}
}
