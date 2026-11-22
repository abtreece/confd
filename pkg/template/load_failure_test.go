package template

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/log"
)

const unparseableResource = "this is not toml ["

// newLoadFailureEnv returns a test environment and a Config that reads it.
// Load failures are logged as errors; the log level keeps test output quiet.
func newLoadFailureEnv(t *testing.T) (*processorTestEnv, Config) {
	t.Helper()
	log.SetLevel("fatal")
	t.Cleanup(func() { log.SetLevel("info") })

	env := newProcessorTestEnv(t)
	config := Config{
		Ctx:           context.Background(),
		ConfigDir:     env.confDir,
		TemplateDir:   env.templDir,
		StoreClient:   newFakeWatchClient(map[string]string{"/test/value": "v1"}),
		BatchInterval: 10 * time.Millisecond,
	}
	return env, config
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestProcess_LoadErrorBestEffort(t *testing.T) {
	env, config := newLoadFailureEnv(t)
	badPath := env.writeConfig("bad.toml", unparseableResource)
	destPath := writeWatchResource(env, "good", "/test/value")
	config.FailureMode = FailModeBestEffort

	err := Process(config)
	if err == nil {
		t.Fatal("Process() = nil, want an error for the resource that failed to load")
	}
	if !strings.Contains(err.Error(), filepath.Base(badPath)) {
		t.Errorf("Process() error = %q, want it to name %s", err, filepath.Base(badPath))
	}
	got, readErr := os.ReadFile(destPath)
	if readErr != nil {
		t.Fatalf("the resource that loaded was not processed: %v", readErr)
	}
	if string(got) != "value=v1" {
		t.Errorf("dest = %q, want %q", got, "value=v1")
	}
}

func TestProcess_LoadErrorFailFast(t *testing.T) {
	env, config := newLoadFailureEnv(t)
	badPath := env.writeConfig("bad.toml", unparseableResource)
	destPath := writeWatchResource(env, "good", "/test/value")
	config.FailureMode = FailModeFast

	err := Process(config)
	if err == nil {
		t.Fatal("Process() = nil, want an error for the resource that failed to load")
	}
	if !strings.Contains(err.Error(), filepath.Base(badPath)) {
		t.Errorf("Process() error = %q, want it to name %s", err, filepath.Base(badPath))
	}
	if fileExists(destPath) {
		t.Errorf("%s was written; fail-fast must not process anything after a load error", destPath)
	}
}

// Every resource file that failed to load is named, including one whose load
// error does not mention its path, and processing errors are reported too.
func TestProcess_LoadErrorBestEffort_ReportsEveryError(t *testing.T) {
	env, config := newLoadFailureEnv(t)
	badPath := env.writeConfig("bad.toml", unparseableResource)
	noSrcPath := env.writeConfig("nosrc.toml", "[template]\ndest = \"/unused\"\n")
	missingKeyDest := writeWatchResource(env, "missingkey", "/test/absent")
	config.FailureMode = FailModeBestEffort

	err := Process(config)
	if err == nil {
		t.Fatal("Process() = nil, want an error")
	}
	for _, want := range []string{badPath, noSrcPath, missingKeyDest} {
		if !strings.Contains(err.Error(), filepath.Base(want)) {
			t.Errorf("Process() error = %q, want it to mention %s", err, filepath.Base(want))
		}
	}
	if !errors.Is(err, ErrEmptySrc) {
		t.Errorf("Process() error = %q, want it to wrap ErrEmptySrc", err)
	}
}

func TestProcess_NothingLoads(t *testing.T) {
	for _, mode := range []FailureMode{FailModeBestEffort, FailModeFast} {
		t.Run(mode.String(), func(t *testing.T) {
			env, config := newLoadFailureEnv(t)
			env.writeConfig("bad.toml", unparseableResource)
			config.FailureMode = mode
			if err := Process(config); err == nil {
				t.Error("Process() = nil, want an error")
			}
		})
	}
}

func TestProcess_EmptyConfigDirIsNotAFailure(t *testing.T) {
	_, config := newLoadFailureEnv(t)
	if err := Process(config); err != nil {
		t.Errorf("Process() = %v, want nil for an empty conf.d", err)
	}
}

type longRunningProcessor struct {
	name string
	new  func(config Config, stopChan, doneChan chan bool, errChan chan error, reloadChan <-chan struct{}) Processor
}

func longRunningProcessors() []longRunningProcessor {
	return []longRunningProcessor{
		{"interval", func(config Config, stopChan, doneChan chan bool, errChan chan error, reloadChan <-chan struct{}) Processor {
			return IntervalProcessor(config, stopChan, doneChan, errChan, 1, reloadChan)
		}},
		{"watch", WatchProcessor},
		{"batch watch", BatchWatchProcessor},
	}
}

// expectNoTemplateResources waits for the processor to stop and checks that
// it reported ErrNoTemplateResources before doing so.
func expectNoTemplateResources(t *testing.T, doneChan chan bool, errChan chan error, wantNamed string) {
	t.Helper()
	waitForDone(t, doneChan, testShutdownTimeout)
	for {
		select {
		case err := <-errChan:
			if !errors.Is(err, ErrNoTemplateResources) {
				continue
			}
			if !strings.Contains(err.Error(), filepath.Base(wantNamed)) {
				t.Errorf("error = %q, want it to name %s", err, filepath.Base(wantNamed))
			}
			return
		default:
			t.Fatal("processor stopped without reporting ErrNoTemplateResources")
		}
	}
}

func TestLongRunning_NothingLoadsReportsErrNoTemplateResources(t *testing.T) {
	for _, p := range longRunningProcessors() {
		t.Run(p.name, func(t *testing.T) {
			env, config := newLoadFailureEnv(t)
			badPath := env.writeConfig("bad.toml", unparseableResource)
			doneChan := make(chan bool)
			errChan := make(chan error, testErrorChannelBuffer)

			go p.new(config, make(chan bool), doneChan, errChan, make(chan struct{})).Process()

			expectNoTemplateResources(t, doneChan, errChan, badPath)
		})
	}
}

func TestLongRunning_ReloadAfterWhichNothingLoads(t *testing.T) {
	for _, p := range longRunningProcessors() {
		t.Run(p.name, func(t *testing.T) {
			env, config := newLoadFailureEnv(t)
			destPath := writeWatchResource(env, "good", "/test/value")
			doneChan := make(chan bool)
			errChan := make(chan error, testErrorChannelBuffer)
			reloadChan := make(chan struct{})

			go p.new(config, make(chan bool), doneChan, errChan, reloadChan).Process()

			// The resource has loaded once it has been rendered.
			if err := waitForFile(t, destPath, 5*time.Second, "value=v1"); err != nil {
				t.Fatalf("initial output not created: %v", err)
			}
			if err := os.Remove(env.confDir + "/good.toml"); err != nil {
				t.Fatal(err)
			}
			badPath := env.writeConfig("bad.toml", unparseableResource)
			select {
			case reloadChan <- struct{}{}:
			case <-time.After(testShutdownTimeout):
				t.Fatal("processor did not accept the reload signal")
			}

			expectNoTemplateResources(t, doneChan, errChan, badPath)
		})
	}
}

// A processor keeps running, without reporting ErrNoTemplateResources, when
// some resources load and when there are no resource files at all.
func TestLongRunning_KeepsRunning(t *testing.T) {
	cases := []struct {
		name  string
		setup func(env *processorTestEnv)
	}{
		{"some resources load", func(env *processorTestEnv) {
			env.writeConfig("bad.toml", unparseableResource)
			writeWatchResource(env, "good", "/test/value")
		}},
		{"empty conf.d", func(env *processorTestEnv) {}},
	}
	for _, p := range longRunningProcessors() {
		for _, tc := range cases {
			t.Run(p.name+"/"+tc.name, func(t *testing.T) {
				env, config := newLoadFailureEnv(t)
				tc.setup(env)
				stopChan := make(chan bool)
				doneChan := make(chan bool)
				errChan := make(chan error, testErrorChannelBuffer)

				go p.new(config, stopChan, doneChan, errChan, make(chan struct{})).Process()

				select {
				case <-doneChan:
					t.Fatal("processor stopped on its own")
				case <-time.After(200 * time.Millisecond):
				}
				close(stopChan)
				waitForDone(t, doneChan, testShutdownTimeout)
				close(errChan)
				for err := range errChan {
					if errors.Is(err, ErrNoTemplateResources) {
						t.Errorf("processor reported %q", err)
					}
				}
			})
		}
	}
}
