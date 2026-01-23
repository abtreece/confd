//go:build e2e

package operations

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

var (
	buildOnce   sync.Once
	binaryPath  string
	buildErr    error
	projectRoot string
)

// findProjectRoot walks up from the current directory to find the project root
// (identified by the presence of go.mod).
func findProjectRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get caller information")
	}

	dir := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

// BuildConfd builds the confd binary once and returns the path.
// Uses sync.Once to avoid rebuilding across multiple tests.
func BuildConfd(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		projectRoot, buildErr = findProjectRoot()
		if buildErr != nil {
			return
		}

		binaryPath = filepath.Join(projectRoot, "bin", "confd")

		// Build the binary using make
		cmd := exec.Command("make", "build")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("failed to build confd: %w\nOutput: %s", err, output)
			return
		}

		// Verify the binary exists
		if _, err := os.Stat(binaryPath); err != nil {
			buildErr = fmt.Errorf("binary not found after build: %w", err)
		}
	})

	if buildErr != nil {
		t.Fatalf("Failed to build confd: %v", buildErr)
	}

	return binaryPath
}

// ConfdBinary manages running the confd binary for tests.
type ConfdBinary struct {
	binaryPath string
	cmd        *exec.Cmd
	t          *testing.T
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	env        []string
}

// NewConfdBinary creates a new ConfdBinary instance.
// Automatically builds confd if not already built.
func NewConfdBinary(t *testing.T) *ConfdBinary {
	t.Helper()
	return &ConfdBinary{
		binaryPath: BuildConfd(t),
		t:          t,
		env:        os.Environ(),
	}
}

// SetEnv sets an environment variable for the confd process.
// Must be called before Start.
func (c *ConfdBinary) SetEnv(key, value string) {
	// Check if the key already exists and update it
	prefix := key + "="
	for i, env := range c.env {
		if len(env) >= len(prefix) && env[:len(prefix)] == prefix {
			c.env[i] = key + "=" + value
			return
		}
	}
	// Add new environment variable
	c.env = append(c.env, key+"="+value)
}

// Start starts confd with the given arguments.
// Returns after the process is running (not necessarily ready).
func (c *ConfdBinary) Start(ctx context.Context, args ...string) error {
	c.cmd = exec.CommandContext(ctx, c.binaryPath, args...)
	c.cmd.Env = c.env

	var err error
	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start confd: %w", err)
	}

	return nil
}

// WaitForReady polls the health endpoint until confd is ready.
func (c *ConfdBinary) WaitForReady(ctx context.Context, addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://%s/health", addr)

	client := &http.Client{Timeout: 1 * time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for confd to be ready at %s", url)
}

// SendSignal sends a signal to the running process.
func (c *ConfdBinary) SendSignal(sig syscall.Signal) error {
	if c.cmd == nil || c.cmd.Process == nil {
		return fmt.Errorf("process not running")
	}
	return c.cmd.Process.Signal(sig)
}

// Stop terminates the process gracefully with SIGTERM, then SIGKILL if needed.
func (c *ConfdBinary) Stop() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	// Try graceful shutdown first
	if err := c.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		return nil
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		// Force kill if graceful shutdown times out
		return c.cmd.Process.Kill()
	}
}

// Wait waits for the process to exit and returns the exit code.
func (c *ConfdBinary) Wait() (int, error) {
	if c.cmd == nil {
		return -1, fmt.Errorf("process not started")
	}

	err := c.cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}

	return 0, nil
}

// IsRunning returns true if the process is still running.
func (c *ConfdBinary) IsRunning() bool {
	if c.cmd == nil || c.cmd.Process == nil {
		return false
	}
	// Sending signal 0 checks if process exists
	err := c.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// GetFreePort returns an available TCP port.
func GetFreePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

// TestEnv holds the test environment for operations tests.
type TestEnv struct {
	ConfDir  string
	DestDir  string
	BaseDir  string
	t        *testing.T
}

// NewTestEnv creates a new test environment with temp directories.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	baseDir := t.TempDir()
	confDir := filepath.Join(baseDir, "confdir")
	templatesDir := filepath.Join(confDir, "templates")
	confDDir := filepath.Join(confDir, "conf.d")
	destDir := filepath.Join(baseDir, "dest")

	for _, dir := range []string{templatesDir, confDDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	return &TestEnv{
		ConfDir: confDir,
		DestDir: destDir,
		BaseDir: baseDir,
		t:       t,
	}
}

// WriteTemplate creates a template file in the templates directory.
func (e *TestEnv) WriteTemplate(name, content string) string {
	e.t.Helper()
	path := filepath.Join(e.ConfDir, "templates", name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.t.Fatalf("Failed to write template %s: %v", name, err)
	}
	return path
}

// WriteConfig creates a template resource TOML config in conf.d.
func (e *TestEnv) WriteConfig(name, content string) string {
	e.t.Helper()
	path := filepath.Join(e.ConfDir, "conf.d", name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.t.Fatalf("Failed to write config %s: %v", name, err)
	}
	return path
}

// DestPath returns the full path to a destination file.
func (e *TestEnv) DestPath(name string) string {
	return filepath.Join(e.DestDir, name)
}

// WaitForFile waits for a file to exist and optionally contain expected content.
func WaitForFile(t *testing.T, path string, timeout time.Duration, expectedContent string) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil {
			if expectedContent == "" {
				return nil
			}
			if string(content) == expectedContent {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for file %s", path)
}
