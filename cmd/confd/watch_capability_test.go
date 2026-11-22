package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runWithTimeout runs fn and fails the test if it does not return: a
// polling-only backend that slips past the capability check would start the
// watch processor and block until a signal arrives.
func runWithTimeout(t *testing.T, fn func() error) error {
	t.Helper()

	errCh := make(chan error, 1)
	go func() { errCh <- fn() }()

	select {
	case err := <-errCh:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("confd kept running in watch mode with a backend that cannot watch")
		return nil
	}
}

// TestWatchCapability_GlobalBackendRejected verifies that watch mode with a
// polling-only global backend fails at startup, whether watch was enabled by
// flag or by confd.toml, and also when combined with --onetime or
// --preflight. The TOML case used to bypass the per-command checks because
// they ran before the config file was loaded.
func TestWatchCapability_GlobalBackendRejected(t *testing.T) {
	tests := []struct {
		name       string
		watchFlag  bool
		configTOML string
		onetime    bool
		preflight  bool
	}{
		{name: "watch flag", watchFlag: true},
		{name: "watch in confd.toml", configTOML: "watch = true\n"},
		{name: "watch flag with onetime", watchFlag: true, onetime: true},
		{name: "watch in confd.toml with onetime", configTOML: "watch = true\n", onetime: true},
		{name: "watch flag with preflight", watchFlag: true, preflight: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := makeCheckConfigCLI(t)
			cli.CheckConfig = false
			cli.Watch = tt.watchFlag
			cli.Onetime = tt.onetime
			cli.Preflight = tt.preflight
			if tt.configTOML != "" {
				cli.ConfigFile = filepath.Join(cli.ConfDir, "confd.toml")
				if err := os.WriteFile(cli.ConfigFile, []byte(tt.configTOML), 0644); err != nil {
					t.Fatalf("failed to write config file: %v", err)
				}
			}

			err := runWithTimeout(t, func() error { return (&EnvCmd{}).Run(cli) })
			if err == nil {
				t.Fatal("expected watch mode with the env backend to fail at startup, got nil")
			}
			if !strings.Contains(err.Error(), "watch mode not supported for env backend") {
				t.Errorf("error = %q, want it to contain %q", err, "watch mode not supported for env backend")
			}
		})
	}
}
