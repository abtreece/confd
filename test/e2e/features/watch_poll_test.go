//go:build e2e

package features

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/operations"
)

// ticksTemplate renders the current time, so the destination of a resource
// using it changes every time the resource is processed. The env backend
// reads confd's own environment, which cannot change while confd runs, so
// this is how a test observes that a resource is being polled.
const ticksTemplate = "{{ (datetime).UnixNano }}\n"

// watchPollEnv is a confdir with two resources:
//   - app.toml renders /app/name from the global file backend (watched).
//   - ticks.toml renders the current time; its backend is set by writeTicks.
type watchPollEnv struct {
	*operations.TestEnv
	yamlPath  string
	appDest   string
	ticksDest string
}

func newWatchPollEnv(t *testing.T) *watchPollEnv {
	t.Helper()

	env := operations.NewTestEnv(t)

	// Keep the YAML in its own directory and pass confd the file path, so
	// the file backend's watch fires only when this file is written, not on
	// destination or TOML writes under BaseDir.
	fileBackendDir := filepath.Join(env.BaseDir, "file-backend")
	if err := os.MkdirAll(fileBackendDir, 0755); err != nil {
		t.Fatalf("Failed to create file backend directory: %v", err)
	}

	wp := &watchPollEnv{
		TestEnv:   env,
		yamlPath:  filepath.Join(fileBackendDir, "data.yaml"),
		appDest:   env.DestPath("app.conf"),
		ticksDest: env.DestPath("ticks.conf"),
	}
	wp.writeAppName(t, "one")

	env.WriteTemplate("app.tmpl", "name={{ getv \"/app/name\" }}\n")
	env.WriteTemplate("ticks.tmpl", ticksTemplate)
	env.WriteConfig("app.toml", fmt.Sprintf(`[template]
src = "app.tmpl"
dest = "%s"
keys = ["/app"]
`, wp.appDest))

	return wp
}

// writeAppName writes the YAML data file in place with /app/name = name.
func (wp *watchPollEnv) writeAppName(t *testing.T, name string) {
	t.Helper()
	content := fmt.Sprintf("app:\n  name: %s\n", name)
	if err := os.WriteFile(wp.yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}
}

// writeTicks writes ticks.toml. An empty backend leaves out the [backend]
// table, so the resource uses the global backend.
func (wp *watchPollEnv) writeTicks(backend string) {
	config := fmt.Sprintf(`[template]
src = "ticks.tmpl"
dest = "%s"
keys = ["/ticks"]
`, wp.ticksDest)
	if backend != "" {
		config += fmt.Sprintf("\n[backend]\nbackend = %q\n", backend)
	}
	wp.WriteConfig("ticks.toml", config)
}

// startWatch starts confd with the global file backend in watch mode at
// --interval 1 and stops it when the test ends.
func (wp *watchPollEnv) startWatch(t *testing.T) *operations.ConfdBinary {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	confd := operations.NewConfdBinary(t)
	// Nothing reads confd's output pipes, so keep logging quiet enough that
	// they never fill and block confd.
	err := confd.Start(ctx, "file",
		"--file", wp.yamlPath,
		"--confdir", wp.ConfDir,
		"--watch",
		"--interval", "1",
		"--log-level", "error",
	)
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	t.Cleanup(func() {
		if err := confd.Stop(); err != nil {
			t.Logf("Failed to stop confd: %v", err)
		}
	})
	return confd
}

// readFileOrEmpty returns the file's contents, or "" if it cannot be read.
func readFileOrEmpty(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

// waitForChange waits for path to hold non-empty contents different from
// old, and returns them. It fails the test with what if that does not happen
// within timeout.
func waitForChange(t *testing.T, path, old string, timeout time.Duration, what string) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		current := readFileOrEmpty(path)
		if current != "" && current != old {
			return current
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s: %s did not change from %q within %s", what, path, old, timeout)
	return ""
}

// TestWatchMode_PollsPerResourceBackendThatCannotWatch verifies that in
// watch mode a resource on the watch-capable global backend follows changes,
// while a resource whose per-resource backend cannot watch is processed
// every --interval.
func TestWatchMode_PollsPerResourceBackendThatCannotWatch(t *testing.T) {
	t.Parallel()

	wp := newWatchPollEnv(t)
	wp.writeTicks("env")

	wp.startWatch(t)

	if err := operations.WaitForFile(t, wp.appDest, 15*time.Second, "name=one\n"); err != nil {
		t.Fatalf("Watched resource was not rendered: %v", err)
	}

	// Only a running confd writes the polled destination, so each change
	// below also proves confd is still alive. ConfdBinary.IsRunning cannot:
	// nothing reaps an exited confd before cleanup, and signal 0 to the
	// zombie succeeds.
	ticks1 := waitForChange(t, wp.ticksDest, "", 5*time.Second, "polled resource was never rendered")
	ticks2 := waitForChange(t, wp.ticksDest, ticks1, 4*time.Second,
		"polled resource was not processed again within 4s at --interval 1")
	waitForChange(t, wp.ticksDest, ticks2, 4*time.Second,
		"polled resource was not processed a third time within 4s at --interval 1")

	// More than a second has passed since the first render, so the watch on
	// the YAML file is in place.
	wp.writeAppName(t, "two")
	if err := operations.WaitForFile(t, wp.appDest, 10*time.Second, "name=two\n"); err != nil {
		t.Fatalf("Watched resource did not follow the YAML change: %v (content %q)",
			err, readFileOrEmpty(wp.appDest))
	}

	// confd must still be running after processing the YAML change: the
	// polled destination changes again.
	waitForChange(t, wp.ticksDest, readFileOrEmpty(wp.ticksDest), 4*time.Second,
		"confd stopped polling after the YAML change; it is no longer running")
}

// TestWatchMode_ReloadMovesResourceToPoller verifies that a SIGHUP reload
// after a resource's TOML gains a per-resource backend that cannot watch
// moves that resource from the watcher to the poller.
func TestWatchMode_ReloadMovesResourceToPoller(t *testing.T) {
	t.Parallel()

	wp := newWatchPollEnv(t)
	wp.writeTicks("")

	confd := wp.startWatch(t)

	if err := operations.WaitForFile(t, wp.appDest, 15*time.Second, "name=one\n"); err != nil {
		t.Fatalf("Watched resource was not rendered: %v", err)
	}
	ticks1 := waitForChange(t, wp.ticksDest, "", 5*time.Second,
		"ticks resource was not rendered on the global file backend")

	// The ticks resource is watched and its keys do not change, so it must
	// not be processed again.
	time.Sleep(3 * time.Second)
	if current := readFileOrEmpty(wp.ticksDest); current != ticks1 {
		t.Fatalf("Watched resource whose keys did not change was processed again: %q -> %q", ticks1, current)
	}

	wp.writeTicks("env")
	if err := confd.SendSignal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Only a running confd writes the destination, so these changes also
	// prove confd survived the reload (ConfdBinary.IsRunning cannot see an
	// exited, unreaped confd).
	ticks2 := waitForChange(t, wp.ticksDest, ticks1, 6*time.Second,
		"after SIGHUP, the ticks resource on the env backend was not polled")
	waitForChange(t, wp.ticksDest, ticks2, 4*time.Second,
		"after SIGHUP, the ticks resource was not polled again within 4s")
}
