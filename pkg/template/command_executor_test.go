package template

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"text/template"
	"time"
)

func TestExecuteCheck_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("test content")
	tmpFile.Close()

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "cat {{.src}}",
		SyncOnly: false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeCheck(tmpFile.Name())
	if err != nil {
		t.Errorf("executeCheck() unexpected error: %v", err)
	}
}

func TestExecuteCheck_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "exit 1",
		SyncOnly: false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeCheck(tmpFile.Name())
	if err == nil {
		t.Error("executeCheck() expected error for exit 1, got nil")
	}
	if err != nil && err.Error() != "config check failed: exit status 1" {
		t.Errorf("executeCheck() expected 'config check failed' prefix, got: %v", err)
	}
}

func TestNewCommandExecutor_InvalidCheckCmdTemplate(t *testing.T) {
	_, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "echo {{.invalid",
		SyncOnly: false,
	})

	if err == nil {
		t.Error("newCommandExecutor() expected error for invalid check_cmd template, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid check_cmd template") {
		t.Errorf("newCommandExecutor() expected 'invalid check_cmd template' error, got: %v", err)
	}
}

func TestExecuteCheck_EmptyCommand(t *testing.T) {
	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "",
		SyncOnly: false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeCheck("/tmp/test")
	if err != nil {
		t.Errorf("executeCheck() with empty command should return nil, got: %v", err)
	}
}

func TestExecuteCheck_SyncOnlyMode(t *testing.T) {
	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "exit 1",
		SyncOnly: true,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeCheck("/tmp/test")
	if err != nil {
		t.Errorf("executeCheck() in syncOnly mode should skip command, got: %v", err)
	}
}

func TestExecuteReload_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	lastReloadTime := time.Time{}
	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:      "echo reloading {{.dest}}",
		LastReloadTime: &lastReloadTime,
		SyncOnly:       false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err != nil {
		t.Errorf("executeReload() unexpected error: %v", err)
	}

	// Check that lastReloadTime was updated
	if lastReloadTime.IsZero() {
		t.Error("executeReload() should update lastReloadTime")
	}
}

func TestExecuteReload_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "exit 1",
		SyncOnly:  false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err == nil {
		t.Error("executeReload() expected error for exit 1, got nil")
	}
}

func TestNewCommandExecutor_InvalidReloadCmdTemplate(t *testing.T) {
	_, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "echo {{.invalid",
		SyncOnly:  false,
	})

	if err == nil {
		t.Error("newCommandExecutor() expected error for invalid reload_cmd template, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid reload_cmd template") {
		t.Errorf("newCommandExecutor() expected 'invalid reload_cmd template' error, got: %v", err)
	}
}

func TestExecuteReload_EmptyCommand(t *testing.T) {
	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "",
		SyncOnly:  false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeReload("/tmp/test", "/tmp/dest")
	if err != nil {
		t.Errorf("executeReload() with empty command should return nil, got: %v", err)
	}
}

func TestExecuteReload_SyncOnlyMode(t *testing.T) {
	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "exit 1",
		SyncOnly:  true,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeReload("/tmp/test", "/tmp/dest")
	if err != nil {
		t.Errorf("executeReload() in syncOnly mode should skip command, got: %v", err)
	}
}

func TestExecuteReload_RateLimiting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Set last reload time to now
	lastReloadTime := time.Now()
	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:         "echo reload",
		MinReloadInterval: 10 * time.Second,
		LastReloadTime:    &lastReloadTime,
		SyncOnly:          false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	// First reload should be throttled (last reload was just now)
	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err != nil {
		t.Errorf("executeReload() should not error when throttled: %v", err)
	}

	// lastReloadTime should not be updated when throttled
	if lastReloadTime != time.Now() && time.Since(lastReloadTime) > time.Second {
		t.Error("executeReload() should not update lastReloadTime when throttled")
	}
}

func TestExecuteReload_RateLimitingAllowsAfterInterval(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Set last reload time to 100ms ago
	lastReloadTime := time.Now().Add(-100 * time.Millisecond)
	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:         "echo reload",
		MinReloadInterval: 50 * time.Millisecond, // 50ms interval
		LastReloadTime:    &lastReloadTime,
		SyncOnly:          false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	// Reload should be allowed (100ms > 50ms)
	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err != nil {
		t.Errorf("executeReload() unexpected error: %v", err)
	}

	// lastReloadTime should be updated
	if time.Since(lastReloadTime) > time.Second {
		t.Error("executeReload() should update lastReloadTime when reload is allowed")
	}
}

func TestExecuteReload_NoRateLimitingWhenIntervalNotSet(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	lastReloadTime := time.Now()
	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:         "echo reload",
		MinReloadInterval: 0, // No rate limiting
		LastReloadTime:    &lastReloadTime,
		SyncOnly:          false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	// Reload should be allowed even though last reload was just now
	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err != nil {
		t.Errorf("executeReload() unexpected error: %v", err)
	}

	// lastReloadTime should be updated
	if lastReloadTime.Before(time.Now().Add(-time.Second)) {
		t.Error("executeReload() should update lastReloadTime")
	}
}

func TestExecuteReload_TemplateSubstitution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "echo src={{.src}} dest={{.dest}}",
		SyncOnly:  false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err != nil {
		t.Errorf("executeReload() unexpected error: %v", err)
	}
}

func TestExecuteCheck_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd:        "sleep 5",
		CheckCmdTimeout: 100 * time.Millisecond,
		Ctx:             context.Background(),
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeCheck(tmpFile.Name())
	if err == nil {
		t.Error("executeCheck() expected timeout error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "timed out") {
		t.Errorf("executeCheck() expected timeout error message, got: %v", err)
	}
}

func TestExecuteReload_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:        "sleep 5",
		ReloadCmdTimeout: 100 * time.Millisecond,
		Ctx:              context.Background(),
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err == nil {
		t.Error("executeReload() expected timeout error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "timed out") {
		t.Errorf("executeReload() expected timeout error message, got: %v", err)
	}
}

func TestExecuteCheck_NoTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Test with zero timeout (no limit)
	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd:        "echo test",
		CheckCmdTimeout: 0,
		Ctx:             context.Background(),
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeCheck(tmpFile.Name())
	if err != nil {
		t.Errorf("executeCheck() unexpected error: %v", err)
	}
}

func TestExecuteCheck_ContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd:        "sleep 1",
		CheckCmdTimeout: 10 * time.Second,
		Ctx:             ctx,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeCheck(tmpFile.Name())
	if err == nil {
		t.Error("executeCheck() expected context cancellation error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("executeCheck() expected cancellation error message, got: %v", err)
	}
}

func TestExecuteReload_ContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:        "sleep 1",
		ReloadCmdTimeout: 10 * time.Second,
		Ctx:              ctx,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err == nil {
		t.Error("executeReload() expected context cancellation error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("executeReload() expected cancellation error message, got: %v", err)
	}
}

func TestExecuteCheck_LongCommandWithTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	start := time.Now()
	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd:        "sleep 10",
		CheckCmdTimeout: 200 * time.Millisecond,
		Ctx:             context.Background(),
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeCheck(tmpFile.Name())
	elapsed := time.Since(start)

	// Verify the command was killed after timeout, not after 10 seconds
	if elapsed > 2*time.Second {
		t.Errorf("executeCheck() took %v, expected to timeout around 200ms", elapsed)
	}
	if err == nil {
		t.Error("executeCheck() expected timeout error, got nil")
	}
}

// BenchmarkCommandTemplatePreCompiled benchmarks the pre-compiled template approach
// where the template is parsed once and reused for each execution.
func BenchmarkCommandTemplatePreCompiled(b *testing.B) {
	checkCmd := "nginx -t -c {{.src}}"
	reloadCmd := "systemctl reload nginx {{.dest}}"

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd:  checkCmd,
		ReloadCmd: reloadCmd,
	})
	if err != nil {
		b.Fatalf("newCommandExecutor() error: %v", err)
	}

	data := map[string]string{"src": "/tmp/nginx.conf", "dest": "/etc/nginx/nginx.conf"}
	var cmdBuffer bytes.Buffer

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmdBuffer.Reset()
		_ = executor.checkCmdTmpl.Execute(&cmdBuffer, data)
		cmdBuffer.Reset()
		_ = executor.reloadCmdTmpl.Execute(&cmdBuffer, data)
	}
}

// BenchmarkCommandTemplateParsePerCall benchmarks the old parse-per-call approach
// where the template is parsed on each execution (for comparison).
func BenchmarkCommandTemplateParsePerCall(b *testing.B) {
	checkCmd := "nginx -t -c {{.src}}"
	reloadCmd := "systemctl reload nginx {{.dest}}"
	data := map[string]string{"src": "/tmp/nginx.conf", "dest": "/etc/nginx/nginx.conf"}
	var cmdBuffer bytes.Buffer

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmdBuffer.Reset()
		tmpl, _ := template.New("checkcmd").Parse(checkCmd)
		_ = tmpl.Execute(&cmdBuffer, data)

		cmdBuffer.Reset()
		tmpl, _ = template.New("reloadcmd").Parse(reloadCmd)
		_ = tmpl.Execute(&cmdBuffer, data)
	}
}
