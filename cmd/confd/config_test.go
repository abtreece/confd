package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
)

func TestLoadConfigFile_InvalidDurations(t *testing.T) {
	tests := []struct {
		name       string
		configTOML string
		wantErrMsg string
		wantHint   string
	}{
		{
			name: "invalid stat_cache_ttl",
			configTOML: `
stat_cache_ttl = "5seconds"
`,
			wantErrMsg: "invalid stat_cache_ttl",
			wantHint:   `"1m", "5m"`,
		},
		{
			name: "invalid dial_timeout",
			configTOML: `
dial_timeout = "5seconds"
`,
			wantErrMsg: "invalid dial_timeout",
			wantHint:   `"5s", "30s"`,
		},
		{
			name: "invalid read_timeout",
			configTOML: `
read_timeout = "1second"
`,
			wantErrMsg: "invalid read_timeout",
			wantHint:   `"1s", "5s"`,
		},
		{
			name: "invalid write_timeout",
			configTOML: `
write_timeout = "invalid"
`,
			wantErrMsg: "invalid write_timeout",
			wantHint:   `"1s", "5s"`,
		},
		{
			name: "invalid retry_base_delay",
			configTOML: `
retry_base_delay = "100milliseconds"
`,
			wantErrMsg: "invalid retry_base_delay",
			wantHint:   `"100ms", "1s"`,
		},
		{
			name: "invalid retry_max_delay",
			configTOML: `
retry_max_delay = "5secs"
`,
			wantErrMsg: "invalid retry_max_delay",
			wantHint:   `"5s", "30s"`,
		},
		{
			name: "invalid watch_error_backoff",
			configTOML: `
watch_error_backoff = "2sec"
`,
			wantErrMsg: "invalid watch_error_backoff",
			wantHint:   `"2s", "5s"`,
		},
		{
			name: "invalid preflight_timeout",
			configTOML: `
preflight_timeout = "10sec"
`,
			wantErrMsg: "invalid preflight_timeout",
			wantHint:   `"10s", "30s"`,
		},
		{
			name: "numeric without unit",
			configTOML: `
dial_timeout = "5"
`,
			wantErrMsg: "invalid dial_timeout",
			wantHint:   `"5s", "30s"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp config file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "confd.toml")
			if err := os.WriteFile(configFile, []byte(tt.configTOML), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			// Create CLI with defaults
			cli := &CLI{
				ConfigFile:        configFile,
				StatCacheTTL:      DefaultStatCacheTTL,
				DialTimeout:       DefaultDialTimeout,
				ReadTimeout:       DefaultReadTimeout,
				WriteTimeout:      DefaultWriteTimeout,
				RetryBaseDelay:    DefaultRetryBaseDelay,
				RetryMaxDelay:     DefaultRetryMaxDelay,
				WatchErrorBackoff: DefaultWatchErrorBackoff,
				PreflightTimeout:  DefaultPreflightTimeout,
				RetryMaxAttempts:  DefaultRetryMaxAttempts,
			}

			backendCfg := &backends.Config{}

			err := loadConfigFile(cli, backendCfg)

			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}

			errStr := err.Error()

			// Check error message contains field name
			if !strings.Contains(errStr, tt.wantErrMsg) {
				t.Errorf("expected error to contain %q, got %q", tt.wantErrMsg, errStr)
			}

			// Check error includes helpful hint
			if !strings.Contains(errStr, tt.wantHint) {
				t.Errorf("expected error to contain hint %q, got %q", tt.wantHint, errStr)
			}

			// Check error includes the invalid value
			if !strings.Contains(errStr, "invalid") {
				t.Errorf("expected error to contain 'invalid', got %q", errStr)
			}
		})
	}
}

func TestLoadConfigFile_ValidDurations(t *testing.T) {
	configTOML := `
stat_cache_ttl = "2m"
dial_timeout = "10s"
read_timeout = "3s"
write_timeout = "3s"
retry_base_delay = "200ms"
retry_max_delay = "10s"
watch_error_backoff = "5s"
preflight_timeout = "20s"
`

	// Create temp config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "confd.toml")
	if err := os.WriteFile(configFile, []byte(configTOML), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Create CLI with defaults
	cli := &CLI{
		ConfigFile:        configFile,
		StatCacheTTL:      DefaultStatCacheTTL,
		DialTimeout:       DefaultDialTimeout,
		ReadTimeout:       DefaultReadTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		RetryBaseDelay:    DefaultRetryBaseDelay,
		RetryMaxDelay:     DefaultRetryMaxDelay,
		WatchErrorBackoff: DefaultWatchErrorBackoff,
		PreflightTimeout:  DefaultPreflightTimeout,
		RetryMaxAttempts:  DefaultRetryMaxAttempts,
	}

	backendCfg := &backends.Config{}

	err := loadConfigFile(cli, backendCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify values were applied
	if cli.StatCacheTTL.String() != "2m0s" {
		t.Errorf("StatCacheTTL = %v, want 2m0s", cli.StatCacheTTL)
	}
	if cli.DialTimeout.String() != "10s" {
		t.Errorf("DialTimeout = %v, want 10s", cli.DialTimeout)
	}
	if cli.ReadTimeout.String() != "3s" {
		t.Errorf("ReadTimeout = %v, want 3s", cli.ReadTimeout)
	}
	if cli.WriteTimeout.String() != "3s" {
		t.Errorf("WriteTimeout = %v, want 3s", cli.WriteTimeout)
	}
	if cli.RetryBaseDelay.String() != "200ms" {
		t.Errorf("RetryBaseDelay = %v, want 200ms", cli.RetryBaseDelay)
	}
	if cli.RetryMaxDelay.String() != "10s" {
		t.Errorf("RetryMaxDelay = %v, want 10s", cli.RetryMaxDelay)
	}
	if cli.WatchErrorBackoff.String() != "5s" {
		t.Errorf("WatchErrorBackoff = %v, want 5s", cli.WatchErrorBackoff)
	}
	if cli.PreflightTimeout.String() != "20s" {
		t.Errorf("PreflightTimeout = %v, want 20s", cli.PreflightTimeout)
	}
}

func TestLoadConfigFile_MissingFile(t *testing.T) {
	cli := &CLI{
		ConfigFile: "/nonexistent/path/confd.toml",
	}
	backendCfg := &backends.Config{}

	// Should not error on missing file (just skip)
	err := loadConfigFile(cli, backendCfg)
	if err != nil {
		t.Fatalf("unexpected error for missing config file: %v", err)
	}
}

func TestLoadConfigFile_ExplicitDefaultSourcesOverrideTOML(t *testing.T) {
	clearCONFDEnvVars(t)

	configFile := writeConfigFile(t, `
interval = 300
failure_mode = "fail-fast"
dial_timeout = "10s"
`)

	tests := []struct {
		name  string
		env   map[string]string
		args  []string
		check func(*testing.T, *CLI)
	}{
		{
			name: "env interval default beats toml",
			env:  map[string]string{"CONFD_INTERVAL": "600"},
			args: []string{"--config-file", configFile, "env"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.Interval != 600 {
					t.Fatalf("Interval = %d, want explicit env default 600", cli.Interval)
				}
			},
		},
		{
			name: "cli interval default beats toml",
			args: []string{"--config-file", configFile, "--interval", "600", "env"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.Interval != 600 {
					t.Fatalf("Interval = %d, want explicit CLI default 600", cli.Interval)
				}
			},
		},
		{
			name: "env failure mode default beats toml",
			env:  map[string]string{"CONFD_FAILURE_MODE": "best-effort"},
			args: []string{"--config-file", configFile, "env"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.FailureMode != "best-effort" {
					t.Fatalf("FailureMode = %q, want explicit env default best-effort", cli.FailureMode)
				}
			},
		},
		{
			name: "env duration default beats toml",
			env:  map[string]string{"CONFD_DIAL_TIMEOUT": "5s"},
			args: []string{"--config-file", configFile, "env"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.DialTimeout != 5*time.Second {
					t.Fatalf("DialTimeout = %v, want explicit env default 5s", cli.DialTimeout)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearCONFDEnvVars(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			cli, _ := parseCLI(t, tt.args)
			backendCfg := &backends.Config{}

			if err := loadConfigFile(cli, backendCfg); err != nil {
				t.Fatalf("loadConfigFile failed: %v", err)
			}
			tt.check(t, cli)
		})
	}
}

func TestLoadConfigFile_BackendPrecedenceWithCommandDefaults(t *testing.T) {
	clearCONFDEnvVars(t)

	configFile := writeConfigFile(t, `
nodes = ["toml:8500"]
scheme = "https"
`)

	cli, _ := parseCLI(t, []string{"--config-file", configFile, "consul"})
	backendCfg := &backends.Config{
		Backend:      "consul",
		BackendNodes: cli.Consul.Node,
		Scheme:       cli.Consul.Scheme,
	}
	if err := loadConfigFile(cli, backendCfg); err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}
	if got := []string(backendCfg.BackendNodes); len(got) != 1 || got[0] != "toml:8500" {
		t.Fatalf("BackendNodes = %v, want TOML nodes", backendCfg.BackendNodes)
	}
	if backendCfg.Scheme != "https" {
		t.Fatalf("Scheme = %q, want TOML scheme", backendCfg.Scheme)
	}

	cli, _ = parseCLI(t, []string{"--config-file", configFile, "consul", "--node", "cli:8500", "--scheme", "http"})
	backendCfg = &backends.Config{
		Backend:      "consul",
		BackendNodes: cli.Consul.Node,
		Scheme:       cli.Consul.Scheme,
	}
	if err := loadConfigFile(cli, backendCfg); err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}
	if got := []string(backendCfg.BackendNodes); len(got) != 1 || got[0] != "cli:8500" {
		t.Fatalf("BackendNodes = %v, want CLI nodes", backendCfg.BackendNodes)
	}
	if backendCfg.Scheme != "http" {
		t.Fatalf("Scheme = %q, want CLI scheme", backendCfg.Scheme)
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	configFile := filepath.Join(t.TempDir(), "confd.toml")
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	return configFile
}
