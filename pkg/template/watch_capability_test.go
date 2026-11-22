package template

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
)

// pollingOnlyClient is a StoreClient that does not implement
// backends.Watcher, like the polling-only backends.
type pollingOnlyClient struct{}

func (pollingOnlyClient) GetValues(_ context.Context, _ []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (pollingOnlyClient) HealthCheck(_ context.Context) error { return nil }
func (pollingOnlyClient) Close() error                        { return nil }

var _ backends.StoreClient = pollingOnlyClient{}

func watchCapabilityConfig(env *processorTestEnv, client backends.StoreClient, backend string) Config {
	return Config{
		ConfigDir:         env.confDir,
		TemplateDir:       env.templDir,
		StoreClient:       client,
		Backend:           backend,
		Ctx:               context.Background(),
		WatchErrorBackoff: 10 * time.Millisecond,
	}
}

func TestCheckWatchSupport_AcceptsWatchCapableBackends(t *testing.T) {
	env := newProcessorTestEnv(t)
	writeWatchResource(env, "watched", "/key")

	config := watchCapabilityConfig(env, newFakeWatchClient(map[string]string{}), "etcd")
	if err := CheckWatchSupport(config); err != nil {
		t.Errorf("CheckWatchSupport() = %v, want nil", err)
	}
}

func TestCheckWatchSupport_GlobalBackendRejected(t *testing.T) {
	env := newProcessorTestEnv(t)
	writeWatchResource(env, "app", "/key")

	err := CheckWatchSupport(watchCapabilityConfig(env, pollingOnlyClient{}, "vault"))
	if err == nil {
		t.Fatal("CheckWatchSupport() = nil, want an error for a polling-only global backend")
	}
	if want := "watch mode not supported for vault backend"; err.Error() != want {
		t.Errorf("CheckWatchSupport() = %q, want %q", err, want)
	}
}

// TestWatchCapability_PerResourceBackendRejected verifies that watch mode is
// rejected when a template resource selects a polling-only backend in its own
// [backend] section, even though the global backend can watch, and that the
// error names the backend and the offending resource only.
func TestWatchCapability_PerResourceBackendRejected(t *testing.T) {
	t.Cleanup(clearClientCache)
	env := newProcessorTestEnv(t)

	writeWatchResource(env, "watched", "/key")

	env.writeTemplate("polling.tmpl", `value={{ getv "/key" }}`)
	env.writeConfig("polling.toml", fmt.Sprintf(`[template]
src = "polling.tmpl"
dest = "%s"
keys = ["/key"]

[backend]
backend = "env"
`, env.destPath("polling.txt")))

	err := CheckWatchSupport(watchCapabilityConfig(env, newFakeWatchClient(map[string]string{}), "etcd"))
	if err == nil {
		t.Fatal("CheckWatchSupport() = nil, want an error for the resource with a polling-only backend")
	}

	msg := err.Error()
	for _, want := range []string{"watch mode not supported", "env", "polling.toml"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not contain %q", msg, want)
		}
	}
	if strings.Contains(msg, "watched.toml") {
		t.Errorf("error %q names watched.toml, whose backend can watch", msg)
	}
}

// TestWatchMonitor_ReportsBackendThatCannotWatch covers a resource whose
// backend cannot watch reaching a watch processor without the startup check,
// as happens when a reload switches a resource to a polling-only backend. The
// processor must report it instead of blocking silently.
func TestWatchMonitor_ReportsBackendThatCannotWatch(t *testing.T) {
	processors := map[string]newWatchModeProcessor{
		"watch":       WatchProcessor,
		"batch watch": BatchWatchProcessor,
	}

	for name, newProcessor := range processors {
		t.Run(name, func(t *testing.T) {
			env := newProcessorTestEnv(t)
			writeWatchResource(env, "app", "/key")

			config := watchCapabilityConfig(env, pollingOnlyClient{}, "vault")
			config.BatchInterval = 10 * time.Millisecond

			stopChan := make(chan bool)
			doneChan := make(chan bool)
			errChan := make(chan error, 10)

			go newProcessor(config, stopChan, doneChan, errChan, nil).Process()

			select {
			case err := <-errChan:
				for _, want := range []string{"watch mode not supported for vault backend", "app.toml"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err, want)
					}
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no error reported for a backend that cannot watch")
			}

			close(stopChan)
			waitForDone(t, doneChan, 5*time.Second)
		})
	}
}
