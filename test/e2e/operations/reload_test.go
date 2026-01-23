//go:build e2e

package operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestReload_AddNewTemplate verifies that a new template added to conf.d
// is discovered and processed after SIGHUP.
func TestReload_AddNewTemplate(t *testing.T) {
	env := NewTestEnv(t)
	originalDestPath := env.DestPath("original.conf")
	newDestPath := env.DestPath("new.conf")

	// Write original template
	env.WriteTemplate("original.tmpl", `original: {{ getv "/original" }}
`)

	// Write original config
	env.WriteConfig("original.toml", fmt.Sprintf(`[template]
src = "original.tmpl"
dest = "%s"
keys = ["/original"]
`, originalDestPath))

	// Start confd in interval mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("ORIGINAL", "original-value")
	confd.SetEnv("NEW", "new-value")
	err := confd.Start(ctx, "env", "--interval", "2", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for initial config file to be created
	if err := WaitForFile(t, originalDestPath, 10*time.Second, "original: original-value\n"); err != nil {
		t.Fatalf("Initial config file not created: %v", err)
	}

	// Verify new template doesn't exist yet
	if _, err := os.Stat(newDestPath); err == nil {
		t.Fatal("New template should not exist before being added")
	}

	// Add new template and config
	env.WriteTemplate("new.tmpl", `new: {{ getv "/new" }}
`)
	env.WriteConfig("new.toml", fmt.Sprintf(`[template]
src = "new.tmpl"
dest = "%s"
keys = ["/new"]
`, newDestPath))

	// Send SIGHUP to trigger reload
	if err := confd.SendSignal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait for the new config file to be created (indicates template was discovered)
	if err := WaitForFile(t, newDestPath, 10*time.Second, "new: new-value\n"); err != nil {
		t.Fatalf("New template not processed after SIGHUP: %v", err)
	}

	// Verify process is still running
	if !confd.IsRunning() {
		t.Error("confd should continue running after SIGHUP")
	}

	// Verify original template still works
	content, err := os.ReadFile(originalDestPath)
	if err != nil {
		t.Fatalf("Original config file missing: %v", err)
	}
	if string(content) != "original: original-value\n" {
		t.Errorf("Original config has unexpected content: %q", string(content))
	}
}

// TestReload_UpdatedTemplateReprocessed verifies that when a template file
// is modified, it is reprocessed after SIGHUP.
func TestReload_UpdatedTemplateReprocessed(t *testing.T) {
	env := NewTestEnv(t)
	destPath := env.DestPath("updated.conf")
	templatePath := env.WriteTemplate("updated.tmpl", `version: 1
key: {{ getv "/key" }}
`)

	// Write config
	env.WriteConfig("updated.toml", fmt.Sprintf(`[template]
src = "updated.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

	// Start confd in interval mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	err := confd.Start(ctx, "env", "--interval", "2", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for initial config file
	expectedV1 := "version: 1\nkey: test-value\n"
	if err := WaitForFile(t, destPath, 10*time.Second, expectedV1); err != nil {
		t.Fatalf("Initial config file not created: %v", err)
	}

	// Modify the template file (bump version)
	newTemplateContent := `version: 2
key: {{ getv "/key" }}
`
	if err := os.WriteFile(templatePath, []byte(newTemplateContent), 0644); err != nil {
		t.Fatalf("Failed to update template: %v", err)
	}

	// Send SIGHUP to trigger reload (clears template cache)
	if err := confd.SendSignal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait for the updated template to be processed
	expectedV2 := "version: 2\nkey: test-value\n"
	if err := WaitForFile(t, destPath, 10*time.Second, expectedV2); err != nil {
		t.Fatalf("Updated template not processed after SIGHUP: %v", err)
	}

	// Verify process is still running
	if !confd.IsRunning() {
		t.Error("confd should continue running after SIGHUP")
	}
}

// TestReload_RemovedTemplateStopsProcessing verifies that when a template
// config is removed from conf.d, it is no longer processed after SIGHUP.
func TestReload_RemovedTemplateStopsProcessing(t *testing.T) {
	env := NewTestEnv(t)
	persistentDestPath := env.DestPath("persistent.conf")
	removedDestPath := env.DestPath("removed.conf")

	// Write templates
	env.WriteTemplate("persistent.tmpl", `persistent: {{ getv "/persistent" }}
`)
	env.WriteTemplate("removed.tmpl", `removed: {{ getv "/removed" }}
`)

	// Write configs
	env.WriteConfig("persistent.toml", fmt.Sprintf(`[template]
src = "persistent.tmpl"
dest = "%s"
keys = ["/persistent"]
`, persistentDestPath))

	removedConfigPath := env.WriteConfig("removed.toml", fmt.Sprintf(`[template]
src = "removed.tmpl"
dest = "%s"
keys = ["/removed"]
`, removedDestPath))

	// Start confd in interval mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("PERSISTENT", "persistent-value")
	confd.SetEnv("REMOVED", "removed-value")
	err := confd.Start(ctx, "env", "--interval", "2", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for both config files to be created
	if err := WaitForFile(t, persistentDestPath, 10*time.Second, "persistent: persistent-value\n"); err != nil {
		t.Fatalf("Persistent config not created: %v", err)
	}
	if err := WaitForFile(t, removedDestPath, 10*time.Second, "removed: removed-value\n"); err != nil {
		t.Fatalf("Removed config not created initially: %v", err)
	}

	// Delete the removed template's config file
	if err := os.Remove(removedConfigPath); err != nil {
		t.Fatalf("Failed to remove config file: %v", err)
	}

	// Also delete the destination file so we can verify it's not recreated
	if err := os.Remove(removedDestPath); err != nil {
		t.Fatalf("Failed to remove destination file: %v", err)
	}

	// Send SIGHUP to trigger reload
	if err := confd.SendSignal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait a bit for reload to complete
	time.Sleep(3 * time.Second)

	// Verify persistent template still works
	content, err := os.ReadFile(persistentDestPath)
	if err != nil {
		t.Fatalf("Persistent config should still exist: %v", err)
	}
	if string(content) != "persistent: persistent-value\n" {
		t.Errorf("Persistent config has unexpected content: %q", string(content))
	}

	// Verify removed template's destination is NOT recreated
	// (because the config was removed from conf.d)
	if _, err := os.Stat(removedDestPath); err == nil {
		t.Error("Removed template's destination should NOT be recreated after SIGHUP")
	}

	// Verify process is still running
	if !confd.IsRunning() {
		t.Error("confd should continue running after SIGHUP")
	}
}

// TestReload_ModifiedConfigReprocessed verifies that when a template resource
// config (.toml) is modified, it takes effect after SIGHUP.
func TestReload_ModifiedConfigReprocessed(t *testing.T) {
	env := NewTestEnv(t)
	originalDestPath := env.DestPath("original-dest.conf")
	newDestPath := env.DestPath("new-dest.conf")

	// Write template
	env.WriteTemplate("config.tmpl", `key: {{ getv "/key" }}
`)

	// Write config pointing to original destination
	configPath := env.WriteConfig("config.toml", fmt.Sprintf(`[template]
src = "config.tmpl"
dest = "%s"
keys = ["/key"]
`, originalDestPath))

	// Start confd in interval mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	err := confd.Start(ctx, "env", "--interval", "2", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for initial config to be created at original location
	if err := WaitForFile(t, originalDestPath, 10*time.Second, "key: test-value\n"); err != nil {
		t.Fatalf("Initial config not created: %v", err)
	}

	// Modify the config to point to new destination
	newConfig := fmt.Sprintf(`[template]
src = "config.tmpl"
dest = "%s"
keys = ["/key"]
`, newDestPath)
	if err := os.WriteFile(configPath, []byte(newConfig), 0644); err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Send SIGHUP to trigger reload
	if err := confd.SendSignal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait for new destination to be created
	if err := WaitForFile(t, newDestPath, 10*time.Second, "key: test-value\n"); err != nil {
		t.Fatalf("New destination not created after config change: %v", err)
	}

	// Verify process is still running
	if !confd.IsRunning() {
		t.Error("confd should continue running after SIGHUP")
	}
}

// TestReload_MultipleSIGHUPs verifies that confd can handle multiple
// consecutive SIGHUP signals.
func TestReload_MultipleSIGHUPs(t *testing.T) {
	env := NewTestEnv(t)
	destPath := env.DestPath("multiple.conf")

	// Write template
	templatePath := filepath.Join(env.ConfDir, "templates", "multiple.tmpl")

	// Write initial template
	if err := os.WriteFile(templatePath, []byte(`version: 1
key: {{ getv "/key" }}
`), 0644); err != nil {
		t.Fatalf("Failed to write initial template: %v", err)
	}

	// Write config
	env.WriteConfig("multiple.toml", fmt.Sprintf(`[template]
src = "multiple.tmpl"
dest = "%s"
keys = ["/key"]
`, destPath))

	// Start confd in interval mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("KEY", "test-value")
	err := confd.Start(ctx, "env", "--interval", "2", "--confdir", env.ConfDir, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for initial config
	if err := WaitForFile(t, destPath, 10*time.Second, "version: 1\nkey: test-value\n"); err != nil {
		t.Fatalf("Initial config not created: %v", err)
	}

	// Send multiple SIGHUPs with template updates
	for version := 2; version <= 4; version++ {
		// Update template
		newContent := fmt.Sprintf(`version: %d
key: {{ getv "/key" }}
`, version)
		if err := os.WriteFile(templatePath, []byte(newContent), 0644); err != nil {
			t.Fatalf("Failed to update template to version %d: %v", version, err)
		}

		// Send SIGHUP
		if err := confd.SendSignal(syscall.SIGHUP); err != nil {
			t.Fatalf("Failed to send SIGHUP for version %d: %v", version, err)
		}

		// Wait for updated content
		expected := fmt.Sprintf("version: %d\nkey: test-value\n", version)
		if err := WaitForFile(t, destPath, 10*time.Second, expected); err != nil {
			t.Fatalf("Template version %d not processed after SIGHUP: %v", version, err)
		}

		t.Logf("Successfully processed version %d after SIGHUP", version)
	}

	// Verify process is still running after all SIGHUPs
	if !confd.IsRunning() {
		t.Error("confd should continue running after multiple SIGHUPs")
	}
}
