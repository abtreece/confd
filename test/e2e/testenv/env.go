//go:build e2e

package testenv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/template"
	"github.com/abtreece/confd/test/e2e/containers"
)

// E2ETestEnv holds the test environment for E2E tests with real backend containers.
type E2ETestEnv struct {
	confDir   string // conf.d directory with TOML configs
	templDir  string // templates directory
	destDir   string // output destination directory
	container containers.BackendContainer
	t         *testing.T
}

// NewE2ETestEnv creates a new E2E test environment.
func NewE2ETestEnv(t *testing.T, container containers.BackendContainer) *E2ETestEnv {
	t.Helper()

	baseDir := t.TempDir()
	confDir := filepath.Join(baseDir, "conf.d")
	templDir := filepath.Join(baseDir, "templates")
	destDir := filepath.Join(baseDir, "dest")

	for _, dir := range []string{confDir, templDir, destDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	return &E2ETestEnv{
		confDir:   confDir,
		templDir:  templDir,
		destDir:   destDir,
		container: container,
		t:         t,
	}
}

// Setup starts the backend container and prepares the test environment.
func (e *E2ETestEnv) Setup(ctx context.Context) error {
	return e.container.Start(ctx)
}

// Teardown stops the backend container.
func (e *E2ETestEnv) Teardown(ctx context.Context) error {
	return e.container.Stop(ctx)
}

// WriteTemplate creates a template file in the templates directory.
func (e *E2ETestEnv) WriteTemplate(name, content string) string {
	e.t.Helper()
	path := filepath.Join(e.templDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.t.Fatalf("Failed to write template %s: %v", name, err)
	}
	return path
}

// WriteConfig creates a template resource TOML config in conf.d.
func (e *E2ETestEnv) WriteConfig(name, content string) string {
	e.t.Helper()
	path := filepath.Join(e.confDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.t.Fatalf("Failed to write config %s: %v", name, err)
	}
	return path
}

// SetBackendValue sets a key-value pair in the backend.
func (e *E2ETestEnv) SetBackendValue(ctx context.Context, key, value string) error {
	return e.container.SetValue(ctx, key, value)
}

// DeleteBackendValue deletes a key from the backend.
func (e *E2ETestEnv) DeleteBackendValue(ctx context.Context, key string) error {
	return e.container.DeleteValue(ctx, key)
}

// RestartBackend restarts the backend container to test reconnection behavior.
// Note: Backend data may be lost on restart depending on the backend type.
func (e *E2ETestEnv) RestartBackend(ctx context.Context) error {
	return e.container.Restart(ctx)
}

// ReadDest reads the content of a destination file.
func (e *E2ETestEnv) ReadDest(name string) (string, error) {
	e.t.Helper()
	path := filepath.Join(e.destDir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// DestPath returns the full path to a destination file.
func (e *E2ETestEnv) DestPath(name string) string {
	return filepath.Join(e.destDir, name)
}

// CreateConfig creates a template.Config for the test environment.
func (e *E2ETestEnv) CreateConfig(ctx context.Context) (template.Config, error) {
	endpoint, err := e.container.Endpoint(ctx)
	if err != nil {
		return template.Config{}, fmt.Errorf("failed to get backend endpoint: %w", err)
	}

	storeClient, err := backends.New(backends.Config{
		Backend:      e.container.BackendName(),
		BackendNodes: []string{endpoint},
		DialTimeout:  5 * time.Second,
	})
	if err != nil {
		return template.Config{}, fmt.Errorf("failed to create store client: %w", err)
	}

	return template.Config{
		Ctx:               ctx,
		ConfDir:           filepath.Dir(e.confDir), // parent of conf.d
		ConfigDir:         e.confDir,
		TemplateDir:       e.templDir,
		StoreClient:       storeClient,
		Noop:              false,
		WatchErrorBackoff: 2 * time.Second,
	}, nil
}

// CreateWatchProcessor creates and returns a WatchProcessor for the test.
func (e *E2ETestEnv) CreateWatchProcessor(ctx context.Context) (template.Processor, chan bool, chan bool, chan error, error) {
	config, err := e.CreateConfig(ctx)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)
	reloadChan := make(chan struct{})

	processor := template.WatchProcessor(config, stopChan, doneChan, errChan, reloadChan)

	return processor, stopChan, doneChan, errChan, nil
}

// CreateConfigWithBatch creates a template.Config with batch interval for batch processor tests.
func (e *E2ETestEnv) CreateConfigWithBatch(ctx context.Context, batchInterval time.Duration) (template.Config, error) {
	config, err := e.CreateConfig(ctx)
	if err != nil {
		return template.Config{}, err
	}
	config.BatchInterval = batchInterval
	return config, nil
}

// CreateBatchWatchProcessor creates and returns a BatchWatchProcessor for the test.
func (e *E2ETestEnv) CreateBatchWatchProcessor(ctx context.Context, batchInterval time.Duration) (template.Processor, chan bool, chan bool, chan error, error) {
	config, err := e.CreateConfigWithBatch(ctx, batchInterval)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)
	reloadChan := make(chan struct{})

	processor := template.BatchWatchProcessor(config, stopChan, doneChan, errChan, reloadChan)

	return processor, stopChan, doneChan, errChan, nil
}

// WaitForFile waits for a file to exist and optionally contain expected content.
func WaitForFile(t *testing.T, path string, timeout time.Duration, expectedContent string) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil {
			if expectedContent == "" || strings.TrimSpace(string(content)) == strings.TrimSpace(expectedContent) {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for file %s with content %q", path, expectedContent)
}

// WaitForFileUpdate waits for a file to be updated with new content (different from notContent).
func WaitForFileUpdate(t *testing.T, path string, timeout time.Duration, notContent string) (string, error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil {
			trimmed := strings.TrimSpace(string(content))
			if trimmed != "" && trimmed != strings.TrimSpace(notContent) {
				return trimmed, nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", fmt.Errorf("timeout waiting for file %s to update from %q", path, notContent)
}
