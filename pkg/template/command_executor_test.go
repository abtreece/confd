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

	"github.com/abtreece/confd/pkg/log"
)

func TestExecuteCheck_Success(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("test content")
	tmpFile.Close()

	checkCmd := "cat {{.src}}"
	if runtime.GOOS == "windows" {
		checkCmd = "type {{.src}}"
	}

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd: checkCmd,
		SyncOnly: false,
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	if err = executor.executeCheck(tmpFile.Name()); err != nil {
		t.Errorf("executeCheck() unexpected error: %v", err)
	}
}

func TestExecuteCheck_Failure(t *testing.T) {
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
	if err != nil && err.Error() != "config check failed: run command: exit status 1" {
		t.Errorf("executeCheck() expected 'config check failed: run command: exit status 1', got: %v", err)
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
	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	sleepCmd := "sleep 5"
	if runtime.GOOS == "windows" {
		sleepCmd = "ping -n 6 127.0.0.1 > nul"
	}

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd:        sleepCmd,
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
	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	sleepCmd := "sleep 5"
	if runtime.GOOS == "windows" {
		sleepCmd = "ping -n 6 127.0.0.1 > nul"
	}

	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:        sleepCmd,
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
	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	sleepCmd := "sleep 1"
	if runtime.GOOS == "windows" {
		sleepCmd = "ping -n 2 127.0.0.1 > nul"
	}

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd:        sleepCmd,
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
	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	sleepCmd := "sleep 1"
	if runtime.GOOS == "windows" {
		sleepCmd = "ping -n 2 127.0.0.1 > nul"
	}

	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:        sleepCmd,
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
		// Killing cmd.exe leaves child processes (e.g. ping) as orphans on Windows;
		// CombinedOutput blocks until the orphan exits, so the timing assertion fails.
		t.Skip("Skipping: Windows does not support process-group kill")
	}
	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	sleepCmd := "sleep 10"
	if runtime.GOOS == "windows" {
		sleepCmd = "ping -n 11 127.0.0.1 > nul"
	}

	start := time.Now()
	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd:        sleepCmd,
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

// captureLogs redirects confd's log output to a temp file at the given level
// and returns a function that reads everything logged so far. pkg/log binds its
// handler to os.Stdout when SetLevel is called, so os.Stdout is swapped first.
func captureLogs(t *testing.T, level string) func() string {
	t.Helper()

	logFile, err := os.CreateTemp(t.TempDir(), "confd-log-*.txt")
	if err != nil {
		t.Fatalf("Failed to create log capture file: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = logFile
	log.SetLevel(level)
	t.Cleanup(func() {
		os.Stdout = origStdout
		log.SetLevel("info")
		logFile.Close()
	})

	return func() string {
		t.Helper()
		data, err := os.ReadFile(logFile.Name())
		if err != nil {
			t.Fatalf("Failed to read captured logs: %v", err)
		}
		return string(data)
	}
}

// commandLogMarker stands in for a credential embedded in a command string.
// The marker commands below produce no output, so the marker can only reach
// the logs through the command string itself.
const commandLogMarker = "confd-marker-s3cr3t-token"

// runMarkerCommands runs a failing check command and a succeeding reload
// command that both contain commandLogMarker.
func runMarkerCommands(t *testing.T) {
	t.Helper()

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd:  "test " + commandLogMarker + " = other",
		ReloadCmd: ": " + commandLogMarker,
		Dest:      "/tmp/marker-dest.conf",
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	if err := executor.executeCheck("/tmp/marker-stage"); err == nil {
		t.Fatal("executeCheck() expected error for failing command, got nil")
	}
	if err := executor.executeReload("/tmp/marker-stage", "/tmp/marker-dest.conf"); err != nil {
		t.Fatalf("executeReload() unexpected error: %v", err)
	}
}

func TestCommandExecutor_CommandNotLoggedAtInfo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}
	logs := captureLogs(t, "info")

	runMarkerCommands(t)

	got := logs()
	if strings.Contains(got, commandLogMarker) {
		t.Errorf("command string leaked into info-level logs:\n%s", got)
	}
	// Operators must still be able to tell which command ran.
	for _, want := range []string{
		"Command failed",
		"Command completed successfully",
		"command_type=check",
		"command_type=reload",
		"dest=/tmp/marker-dest.conf",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("info-level logs missing %q:\n%s", want, got)
		}
	}
}

func TestCommandExecutor_CommandLoggedAtDebug(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}
	logs := captureLogs(t, "debug")

	runMarkerCommands(t)

	got := logs()
	for _, want := range []string{
		"command=test " + commandLogMarker + " = other",
		"command=: " + commandLogMarker,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("debug-level logs missing %q:\n%s", want, got)
		}
	}
}

func TestCommandExecutor_ExitStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}
	logs := captureLogs(t, "info")

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "exit 3",
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	err = executor.executeCheck("/tmp/test")
	if err == nil || !strings.Contains(err.Error(), "exit status 3") {
		t.Errorf("executeCheck() expected 'exit status 3' error, got: %v", err)
	}
	if got := logs(); !strings.Contains(got, "exit_code=3") {
		t.Errorf("logs missing exit_code=3:\n%s", got)
	}
}

func TestCommandExecutor_MissingCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}
	logs := captureLogs(t, "info")

	executor, err := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "nonexistent_command_12345",
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	// The shell reports a missing command as exit status 127.
	err = executor.executeReload("/tmp/test", "/tmp/dest")
	if err == nil || !strings.Contains(err.Error(), "exit status 127") {
		t.Errorf("executeReload() expected 'exit status 127' error, got: %v", err)
	}
	if got := logs(); !strings.Contains(got, "exit_code=127") {
		t.Errorf("logs missing exit_code=127:\n%s", got)
	}
}

func TestCommandExecutor_OutputCapture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}
	logs := captureLogs(t, "debug")

	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "echo 'test output'",
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	if err := executor.executeCheck("/tmp/test"); err != nil {
		t.Fatalf("executeCheck() unexpected error: %v", err)
	}

	got := logs()
	for _, want := range []string{"output=test output", "output_size_bytes=12"} {
		if !strings.Contains(got, want) {
			t.Errorf("logs missing %q:\n%s", want, got)
		}
	}
}

func TestCommandExecutor_OutputTruncation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}
	logs := captureLogs(t, "info")

	// 500 "a" bytes fill the logged output limit; everything after is cut.
	const pastLimit = "PAST-LIMIT"
	executor, err := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "printf 'a%.0s' $(seq 1 500); printf " + pastLimit + "; exit 1",
	})
	if err != nil {
		t.Fatalf("newCommandExecutor() unexpected error: %v", err)
	}

	if err := executor.executeCheck("/tmp/test"); err == nil {
		t.Fatal("executeCheck() expected error for exit 1, got nil")
	}

	got := logs()
	wantOutput := "output=" + strings.Repeat("a", 500) + "... (truncated)"
	if !strings.Contains(got, wantOutput) {
		t.Errorf("logs missing output truncated to 500 bytes:\n%s", got)
	}
	if strings.Contains(got, pastLimit) {
		t.Errorf("logs contain output past the truncation limit:\n%s", got)
	}
	if !strings.Contains(got, "output_size_bytes=510") {
		t.Errorf("logs missing full output size (output_size_bytes=510):\n%s", got)
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
