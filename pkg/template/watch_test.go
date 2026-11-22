package template

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
)

// newWatchModeProcessor matches the signature shared by WatchProcessor and
// BatchWatchProcessor so lifecycle tests can run against both modes.
type newWatchModeProcessor func(config Config, stopChan, doneChan chan bool, errChan chan error, reloadChan <-chan struct{}) Processor

// fakeWatchClient is an in-memory StoreClient whose WatchPrefix blocks until
// a value is set, the stop channel is closed, or the context is cancelled.
type fakeWatchClient struct {
	mu      sync.Mutex
	values  map[string]string
	index   uint64
	changed chan struct{}
}

func newFakeWatchClient(values map[string]string) *fakeWatchClient {
	return &fakeWatchClient{values: values, index: 1, changed: make(chan struct{})}
}

func (c *fakeWatchClient) set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = value
	c.index++
	close(c.changed)
	c.changed = make(chan struct{})
}

func (c *fakeWatchClient) GetValues(_ context.Context, keys []string) (map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	values := make(map[string]string)
	for storedKey, value := range c.values {
		for _, key := range keys {
			if strings.HasPrefix(storedKey, key) {
				values[storedKey] = value
			}
		}
	}
	return values, nil
}

func (c *fakeWatchClient) WatchPrefix(ctx context.Context, _ string, _ []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	c.mu.Lock()
	index, changed := c.index, c.changed
	c.mu.Unlock()

	if waitIndex < index {
		return index, nil
	}
	select {
	case <-changed:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.index, nil
	case <-stopChan:
		return waitIndex, nil
	case <-ctx.Done():
		return waitIndex, fmt.Errorf("watch cancelled: %w", ctx.Err())
	}
}

func (c *fakeWatchClient) HealthCheck(_ context.Context) error { return nil }
func (c *fakeWatchClient) Close() error                        { return nil }

var _ backends.StoreClient = (*fakeWatchClient)(nil)

// writeWatchResource writes a template and its template resource config,
// returning the destination path the resource renders to.
func writeWatchResource(env *processorTestEnv, name, key string) string {
	env.t.Helper()
	env.writeTemplate(name+".tmpl", fmt.Sprintf(`value={{ getv "%s" }}`, key))
	destPath := env.destPath(name + ".txt")
	env.writeConfig(name+".toml", fmt.Sprintf(`[template]
src = "%s.tmpl"
dest = "%s"
keys = ["%s"]
`, name, destPath, key))
	return destPath
}

func waitForDone(t *testing.T, doneChan chan bool, timeout time.Duration) {
	t.Helper()
	select {
	case <-doneChan:
	case <-time.After(timeout):
		t.Fatalf("Process did not return within %v", timeout)
	}
}

// testBackoffCancelledOnStop verifies that a stop signal arriving while a
// monitor is backing off after a WatchPrefix error ends Process immediately
// instead of after WatchErrorBackoff.
func testBackoffCancelledOnStop(t *testing.T, newProcessor newWatchModeProcessor) {
	// The erroring backend logs an expected error; keep test output quiet.
	log.SetLevel("error")
	t.Cleanup(func() { log.SetLevel("info") })

	env := newProcessorTestEnv(t)
	writeWatchResource(env, "backoff", "/test/value")

	config := Config{
		Ctx:               context.Background(),
		ConfigDir:         env.confDir,
		TemplateDir:       env.templDir,
		StoreClient:       &erroringStoreClient{},
		WatchErrorBackoff: time.Minute,
		BatchInterval:     10 * time.Millisecond,
	}
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, testErrorChannelBuffer)

	go newProcessor(config, stopChan, doneChan, errChan, make(chan struct{})).Process()

	// The monitor reports the watch error just before it starts backing off.
	select {
	case <-errChan:
	case <-time.After(testShutdownTimeout):
		t.Fatal("no watch error reported; monitor never reached backoff")
	}

	close(stopChan)
	waitForDone(t, doneChan, testShutdownTimeout)
}

func TestWatchProcessor_BackoffCancelledOnStop(t *testing.T) {
	testBackoffCancelledOnStop(t, WatchProcessor)
}

func TestBatchWatchProcessor_BackoffCancelledOnStop(t *testing.T) {
	testBackoffCancelledOnStop(t, BatchWatchProcessor)
}

// testReloadRestartsMonitors verifies that a reload signal reloads the
// template resources (picking up one added after startup) and that the
// restarted monitors still react to backend changes.
func testReloadRestartsMonitors(t *testing.T, newProcessor newWatchModeProcessor) {
	env := newProcessorTestEnv(t)
	firstDest := writeWatchResource(env, "first", "/first/value")

	client := newFakeWatchClient(map[string]string{
		"/first/value":  "one",
		"/second/value": "two",
	})
	config := Config{
		Ctx:               context.Background(),
		ConfigDir:         env.confDir,
		TemplateDir:       env.templDir,
		StoreClient:       client,
		WatchErrorBackoff: time.Minute,
		BatchInterval:     10 * time.Millisecond,
	}
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, testErrorChannelBuffer)
	reloadChan := make(chan struct{})

	go newProcessor(config, stopChan, doneChan, errChan, reloadChan).Process()

	if err := waitForFile(t, firstDest, 5*time.Second, "value=one"); err != nil {
		t.Fatalf("initial output not created: %v", err)
	}

	// A resource added after startup is only picked up by a reload.
	secondDest := writeWatchResource(env, "second", "/second/value")
	select {
	case reloadChan <- struct{}{}:
	case <-time.After(testShutdownTimeout):
		t.Fatal("processor did not accept the reload signal")
	}
	if err := waitForFile(t, secondDest, 5*time.Second, "value=two"); err != nil {
		t.Fatalf("resource added before reload was not processed: %v", err)
	}

	// Watching has resumed if a backend change reaches the reloaded resources.
	client.set("/first/value", "uno")
	if err := waitForFile(t, firstDest, 5*time.Second, "value=uno"); err != nil {
		t.Fatalf("change after reload was not processed: %v", err)
	}

	close(stopChan)
	waitForDone(t, doneChan, testShutdownTimeout)

	select {
	case err := <-errChan:
		t.Errorf("unexpected processor error: %v", err)
	default:
	}
}

func TestWatchProcessor_ReloadRestartsMonitors(t *testing.T) {
	testReloadRestartsMonitors(t, WatchProcessor)
}

func TestBatchWatchProcessor_ReloadRestartsMonitors(t *testing.T) {
	testReloadRestartsMonitors(t, BatchWatchProcessor)
}
