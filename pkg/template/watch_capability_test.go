package template

import (
	"context"
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

// TestCheckWatchSupport_AllowsPerResourceBackendThatCannotWatch verifies
// that a template resource selecting a backend that cannot watch in its own
// [backend] section does not fail the startup check: watch mode polls it.
func TestCheckWatchSupport_AllowsPerResourceBackendThatCannotWatch(t *testing.T) {
	t.Cleanup(clearClientCache)
	env := newProcessorTestEnv(t)

	writeWatchResource(env, "watched", "/key")
	writePolledResource(env, "polled")

	config := watchCapabilityConfig(env, newFakeWatchClient(map[string]string{}), "etcd")
	if err := CheckWatchSupport(config); err != nil {
		t.Errorf("CheckWatchSupport() = %v, want nil", err)
	}
}
