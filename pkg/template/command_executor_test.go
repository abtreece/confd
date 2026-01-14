package template

import (
	"os"
	"runtime"
	"testing"
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

	executor := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "cat {{.src}}",
		SyncOnly: false,
	})

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

	executor := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "exit 1",
		SyncOnly: false,
	})

	err = executor.executeCheck(tmpFile.Name())
	if err == nil {
		t.Error("executeCheck() expected error for exit 1, got nil")
	}
	if err != nil && err.Error() != "Config check failed: exit status 1" {
		t.Errorf("executeCheck() expected 'Config check failed' prefix, got: %v", err)
	}
}

func TestExecuteCheck_InvalidTemplate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	executor := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "echo {{.invalid",
		SyncOnly: false,
	})

	err = executor.executeCheck(tmpFile.Name())
	if err == nil {
		t.Error("executeCheck() expected error for invalid template, got nil")
	}
}

func TestExecuteCheck_EmptyCommand(t *testing.T) {
	executor := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "",
		SyncOnly: false,
	})

	err := executor.executeCheck("/tmp/test")
	if err != nil {
		t.Errorf("executeCheck() with empty command should return nil, got: %v", err)
	}
}

func TestExecuteCheck_SyncOnlyMode(t *testing.T) {
	executor := newCommandExecutor(commandExecutorConfig{
		CheckCmd: "exit 1",
		SyncOnly: true,
	})

	err := executor.executeCheck("/tmp/test")
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
	executor := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:      "echo reloading {{.dest}}",
		LastReloadTime: &lastReloadTime,
		SyncOnly:       false,
	})

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

	executor := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "exit 1",
		SyncOnly:  false,
	})

	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err == nil {
		t.Error("executeReload() expected error for exit 1, got nil")
	}
}

func TestExecuteReload_InvalidTemplate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	executor := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "echo {{.invalid",
		SyncOnly:  false,
	})

	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err == nil {
		t.Error("executeReload() expected error for invalid template, got nil")
	}
}

func TestExecuteReload_EmptyCommand(t *testing.T) {
	executor := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "",
		SyncOnly:  false,
	})

	err := executor.executeReload("/tmp/test", "/tmp/dest")
	if err != nil {
		t.Errorf("executeReload() with empty command should return nil, got: %v", err)
	}
}

func TestExecuteReload_SyncOnlyMode(t *testing.T) {
	executor := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "exit 1",
		SyncOnly:  true,
	})

	err := executor.executeReload("/tmp/test", "/tmp/dest")
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
	executor := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:         "echo reload",
		MinReloadInterval: 10 * time.Second,
		LastReloadTime:    &lastReloadTime,
		SyncOnly:          false,
	})

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
	executor := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:         "echo reload",
		MinReloadInterval: 50 * time.Millisecond, // 50ms interval
		LastReloadTime:    &lastReloadTime,
		SyncOnly:          false,
	})

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
	executor := newCommandExecutor(commandExecutorConfig{
		ReloadCmd:         "echo reload",
		MinReloadInterval: 0, // No rate limiting
		LastReloadTime:    &lastReloadTime,
		SyncOnly:          false,
	})

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

	executor := newCommandExecutor(commandExecutorConfig{
		ReloadCmd: "echo src={{.src}} dest={{.dest}}",
		SyncOnly:  false,
	})

	err = executor.executeReload(tmpFile.Name(), "/tmp/test.conf")
	if err != nil {
		t.Errorf("executeReload() unexpected error: %v", err)
	}
}
