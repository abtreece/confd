package template

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends/env"
)

// reloadRetryFixture is a template resource whose reload_cmd appends the
// rendered {{.src}} path to reloadLog and fails while failMarker exists.
type reloadRetryFixture struct {
	resource   *TemplateResource
	dest       string
	reloadLog  string
	failMarker string
}

// newReloadRetryFixture renders the RELOAD_RETRY_VALUE environment variable to
// a destination file. extraTOML is appended to the [template] section.
func newReloadRetryFixture(t *testing.T, config Config, extraTOML string) *reloadRetryFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("reload_cmd in this test is a POSIX shell command")
	}
	t.Setenv("RELOAD_RETRY_VALUE", "v1")

	workDir := t.TempDir()
	confDir := filepath.Join(workDir, "conf.d")
	templateDir := filepath.Join(workDir, "templates")
	for _, dir := range []string{confDir, templateDir} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	fixture := &reloadRetryFixture{
		dest:       filepath.Join(workDir, "app.conf"),
		reloadLog:  filepath.Join(workDir, "reload.log"),
		failMarker: filepath.Join(workDir, "reload.fail"),
	}

	if err := os.WriteFile(filepath.Join(templateDir, "app.tmpl"), []byte(`{{getv "/reload/retry/value"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	resourceContent := fmt.Sprintf(`[template]
src = "app.tmpl"
dest = "%s"
keys = ["/reload/retry/value"]
reload_cmd = "echo {{.src}} >> %s; test ! -e %s"
%s
`, fixture.dest, fixture.reloadLog, fixture.failMarker, extraTOML)
	resourcePath := filepath.Join(confDir, "app.toml")
	if err := os.WriteFile(resourcePath, []byte(resourceContent), 0644); err != nil {
		t.Fatal(err)
	}

	storeClient, err := env.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	config.ConfDir = workDir
	config.ConfigDir = confDir
	config.TemplateDir = templateDir
	config.StoreClient = storeClient

	fixture.resource, err = NewTemplateResource(resourcePath, config)
	if err != nil {
		t.Fatalf("NewTemplateResource failed: %v", err)
	}
	return fixture
}

func (f *reloadRetryFixture) failReloads(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(f.failMarker, nil, 0644); err != nil {
		t.Fatal(err)
	}
}

func (f *reloadRetryFixture) allowReloads(t *testing.T) {
	t.Helper()
	if err := os.Remove(f.failMarker); err != nil {
		t.Fatal(err)
	}
}

// reloadSrcPaths returns the {{.src}} value of every reload_cmd invocation.
func (f *reloadRetryFixture) reloadSrcPaths(t *testing.T) []string {
	t.Helper()
	content, err := os.ReadFile(f.reloadLog)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(content))
}

func (f *reloadRetryFixture) assertReloadCount(t *testing.T, want int) {
	t.Helper()
	if got := len(f.reloadSrcPaths(t)); got != want {
		t.Fatalf("reload_cmd ran %d time(s), want %d", got, want)
	}
}

func (f *reloadRetryFixture) assertDestContent(t *testing.T, want string) {
	t.Helper()
	content, err := os.ReadFile(f.dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("destination content = %q, want %q", content, want)
	}
}

func TestTemplateResource_ReloadRetriedAfterFailure(t *testing.T) {
	fixture := newReloadRetryFixture(t, Config{}, "")
	fixture.failReloads(t)

	if err := fixture.resource.process(); err == nil {
		t.Fatal("first process() returned nil, want the reload failure")
	}
	fixture.assertReloadCount(t, 1)
	fixture.assertDestContent(t, "v1")

	// Unchanged data, still failing: retried, reported as an error again.
	if err := fixture.resource.process(); err == nil {
		t.Fatal("process() with a failing retry returned nil, want the reload failure")
	}
	fixture.assertReloadCount(t, 2)

	fixture.allowReloads(t)
	if err := fixture.resource.process(); err != nil {
		t.Fatalf("process() with a succeeding retry returned %v", err)
	}
	fixture.assertReloadCount(t, 3)

	// The stage file is moved onto the destination before the first reload
	// and there is none on a retry: {{.src}} is the destination every time.
	for run, srcPath := range fixture.reloadSrcPaths(t) {
		if srcPath != fixture.dest {
			t.Errorf("reload %d ran with {{.src}} = %s, want the destination %s", run+1, srcPath, fixture.dest)
		}
	}

	// The retry succeeded, so nothing is pending any more.
	if err := fixture.resource.process(); err != nil {
		t.Fatalf("process() after a successful retry returned %v", err)
	}
	fixture.assertReloadCount(t, 3)
}

func TestTemplateResource_ReloadNotRetriedAfterSuccess(t *testing.T) {
	fixture := newReloadRetryFixture(t, Config{}, "")

	for pass := 1; pass <= 3; pass++ {
		if err := fixture.resource.process(); err != nil {
			t.Fatalf("process() pass %d returned %v", pass, err)
		}
		fixture.assertReloadCount(t, 1)
	}
}

func TestTemplateResource_ReloadRetryReplacedByUpdate(t *testing.T) {
	fixture := newReloadRetryFixture(t, Config{}, "")
	fixture.failReloads(t)

	if err := fixture.resource.process(); err == nil {
		t.Fatal("first process() returned nil, want the reload failure")
	}

	// The data changes while a reload is pending: the normal update path runs
	// and its successful reload settles the pending one.
	fixture.allowReloads(t)
	t.Setenv("RELOAD_RETRY_VALUE", "v2")
	if err := fixture.resource.process(); err != nil {
		t.Fatalf("process() after a data change returned %v", err)
	}
	fixture.assertReloadCount(t, 2)
	fixture.assertDestContent(t, "v2")
	if srcPath := fixture.reloadSrcPaths(t)[1]; srcPath != fixture.dest {
		t.Errorf("update ran reload_cmd with {{.src}} = %s, want the destination %s", srcPath, fixture.dest)
	}

	if err := fixture.resource.process(); err != nil {
		t.Fatalf("process() after the update returned %v", err)
	}
	fixture.assertReloadCount(t, 2)
}

func TestTemplateResource_ReloadRetryNotThrottledAfterFailure(t *testing.T) {
	fixture := newReloadRetryFixture(t, Config{}, `min_reload_interval = "1h"`)
	fixture.failReloads(t)

	if err := fixture.resource.process(); err == nil {
		t.Fatal("first process() returned nil, want the reload failure")
	}
	if !fixture.resource.lastReloadTime.IsZero() {
		t.Fatal("a failed reload started min_reload_interval")
	}

	fixture.allowReloads(t)
	if err := fixture.resource.process(); err != nil {
		t.Fatalf("process() with a succeeding retry returned %v", err)
	}
	fixture.assertReloadCount(t, 2)

	if err := fixture.resource.process(); err != nil {
		t.Fatalf("process() after a successful retry returned %v", err)
	}
	fixture.assertReloadCount(t, 2)
}

func TestTemplateResource_ThrottledReloadRetryStaysPending(t *testing.T) {
	fixture := newReloadRetryFixture(t, Config{}, `min_reload_interval = "1h"`)
	fixture.failReloads(t)

	if err := fixture.resource.process(); err == nil {
		t.Fatal("first process() returned nil, want the reload failure")
	}
	fixture.allowReloads(t)

	// A successful reload less than min_reload_interval ago throttles the retry.
	fixture.resource.lastReloadTime = time.Now()
	if err := fixture.resource.process(); err != nil {
		t.Fatalf("process() with a throttled retry returned %v", err)
	}
	fixture.assertReloadCount(t, 1)
	if !fixture.resource.reloadPending {
		t.Fatal("a throttled retry cleared the pending reload")
	}

	// Once the interval has elapsed the retry runs.
	fixture.resource.lastReloadTime = time.Now().Add(-2 * time.Hour)
	if err := fixture.resource.process(); err != nil {
		t.Fatalf("process() after min_reload_interval returned %v", err)
	}
	fixture.assertReloadCount(t, 2)
	if fixture.resource.reloadPending {
		t.Fatal("a successful retry left the reload pending")
	}
}

func TestTemplateResource_ReloadRetrySkippedInNoopAndSyncOnly(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{name: "noop", config: Config{Noop: true}},
		{name: "sync-only", config: Config{SyncOnly: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newReloadRetryFixture(t, tt.config, "")
			if err := os.WriteFile(fixture.dest, []byte("v1"), 0644); err != nil {
				t.Fatal(err)
			}
			fixture.resource.reloadPending = true

			if err := fixture.resource.process(); err != nil {
				t.Fatalf("process() returned %v", err)
			}
			fixture.assertReloadCount(t, 0)
		})
	}
}
