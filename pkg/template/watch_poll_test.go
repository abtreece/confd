package template

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testPollInterval is the PollInterval used by tests that poll resources
// whose backend cannot watch.
const testPollInterval = 50 * time.Millisecond

// countingWatchClient is a fakeWatchClient that counts GetValues calls, which
// happen once each time a resource on it is processed.
type countingWatchClient struct {
	*fakeWatchClient
	fetches atomic.Int64
}

func (c *countingWatchClient) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	c.fetches.Add(1)
	return c.fakeWatchClient.GetValues(ctx, keys)
}

// tickingTemplate renders the current time, so every time its resource is
// processed the destination is written again with new content.
const tickingTemplate = `{{ (datetime).UnixNano }}`

// writeTickingResource writes a template resource whose template renders the
// current time, using backendSection as its [backend] section (empty for the
// global backend). It returns the destination path.
func writeTickingResource(env *processorTestEnv, name, backendSection string) string {
	env.t.Helper()
	env.writeTemplate(name+".tmpl", tickingTemplate)
	destPath := env.destPath(name + ".txt")
	env.writeConfig(name+".toml", fmt.Sprintf(`[template]
src = "%s.tmpl"
dest = "%s"
keys = ["/ticks"]
%s`, name, destPath, backendSection))
	return destPath
}

// writePolledResource writes a ticking template resource whose own [backend]
// section selects the env backend, which cannot watch. Callers must clean up
// the client cache with t.Cleanup(clearClientCache).
func writePolledResource(env *processorTestEnv, name string) string {
	env.t.Helper()
	return writeTickingResource(env, name, "\n[backend]\nbackend = \"env\"\n")
}

// waitForTicks waits for the destination at path to be written n more times
// after holding last, returning its final content.
func waitForTicks(t *testing.T, path, last string, n int) string {
	t.Helper()
	for i := range n {
		next, err := waitForFileUpdate(t, path, 5*time.Second, last)
		if err != nil {
			t.Fatalf("resource was processed %d of %d times: %v", i, n, err)
		}
		last = next
	}
	return last
}

func pollTestConfig(env *processorTestEnv, client *countingWatchClient) Config {
	return Config{
		Ctx:               context.Background(),
		ConfigDir:         env.confDir,
		TemplateDir:       env.templDir,
		StoreClient:       client,
		Backend:           "etcd",
		WatchErrorBackoff: time.Minute,
		BatchInterval:     10 * time.Millisecond,
		PollInterval:      testPollInterval,
	}
}

func requireNoProcessorError(t *testing.T, errChan chan error) {
	t.Helper()
	select {
	case err := <-errChan:
		t.Errorf("unexpected processor error: %v", err)
	default:
	}
}

// testPollsResourcesThatCannotWatch verifies that a resource whose own
// backend cannot watch is processed repeatedly at PollInterval, while a
// resource on the watch-capable global backend is only processed when the
// backend reports a change.
func testPollsResourcesThatCannotWatch(t *testing.T, newProcessor newWatchModeProcessor) {
	t.Cleanup(clearClientCache)
	env := newProcessorTestEnv(t)
	watchedDest := writeWatchResource(env, "watched", "/key")
	polledDest := writePolledResource(env, "polled")

	client := &countingWatchClient{fakeWatchClient: newFakeWatchClient(map[string]string{"/key": "one"})}
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, testErrorChannelBuffer)

	go newProcessor(pollTestConfig(env, client), stopChan, doneChan, errChan, make(chan struct{})).Process()

	if err := waitForFile(t, watchedDest, 5*time.Second, "value=one"); err != nil {
		t.Fatalf("watched resource not rendered: %v", err)
	}
	waitForTicks(t, polledDest, "", 3)

	// Polling the env resource must not process the watched resource.
	if fetches := client.fetches.Load(); fetches != 1 {
		t.Errorf("watched resource fetched %d times before any change, want 1", fetches)
	}

	client.set("/key", "two")
	if err := waitForFile(t, watchedDest, 5*time.Second, "value=two"); err != nil {
		t.Fatalf("watched resource did not react to a change: %v", err)
	}

	close(stopChan)
	waitForDone(t, doneChan, testShutdownTimeout)
	requireNoProcessorError(t, errChan)
}

func TestWatchProcessor_PollsResourcesThatCannotWatch(t *testing.T) {
	testPollsResourcesThatCannotWatch(t, WatchProcessor)
}

func TestBatchWatchProcessor_PollsResourcesThatCannotWatch(t *testing.T) {
	testPollsResourcesThatCannotWatch(t, BatchWatchProcessor)
}

// TestWatchProcessor_ReloadMovesResourceToPoller verifies that a reload that
// switches a watched resource to a backend that cannot watch polls it from
// then on, instead of reporting an error.
func TestWatchProcessor_ReloadMovesResourceToPoller(t *testing.T) {
	t.Cleanup(clearClientCache)
	env := newProcessorTestEnv(t)
	ticksDest := writeTickingResource(env, "ticks", "")

	client := &countingWatchClient{fakeWatchClient: newFakeWatchClient(map[string]string{"/ticks": "x"})}
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, testErrorChannelBuffer)
	reloadChan := make(chan struct{})

	go WatchProcessor(pollTestConfig(env, client), stopChan, doneChan, errChan, reloadChan).Process()

	first := waitForTicks(t, ticksDest, "", 1)

	// Watched on the global backend, the resource is not processed again
	// while its keys do not change.
	time.Sleep(5 * testPollInterval)
	if content, err := env.readDest("ticks.txt"); err != nil || strings.TrimSpace(content) != first {
		t.Fatalf("watched resource was processed again without a change (content %q, err %v)", content, err)
	}

	writePolledResource(env, "ticks")
	select {
	case reloadChan <- struct{}{}:
	case <-time.After(testShutdownTimeout):
		t.Fatal("processor did not accept the reload signal")
	}
	waitForTicks(t, ticksDest, first, 3)

	close(stopChan)
	waitForDone(t, doneChan, testShutdownTimeout)
	requireNoProcessorError(t, errChan)
}
