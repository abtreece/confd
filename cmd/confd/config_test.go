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

func TestLoadConfigFile_DocumentedKeys(t *testing.T) {
	configTOML := `
backend_timeout = "45s"
check_cmd_timeout = "15s"
reload_cmd_timeout = "90s"
shutdown_timeout = "20s"
debounce = "2s"
systemd_notify = true
watchdog_interval = "25s"
log_level = "debug"
log_format = "json"
`

	tests := []struct {
		name string
		env  map[string]string
		args []string
		// check receives the CLI after the config file has been loaded
		check func(*testing.T, *CLI)
	}{
		{
			name: "toml values applied",
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.BackendTimeout != 45*time.Second {
					t.Errorf("BackendTimeout = %v, want 45s", cli.BackendTimeout)
				}
				if cli.CheckCmdTimeout != 15*time.Second {
					t.Errorf("CheckCmdTimeout = %v, want 15s", cli.CheckCmdTimeout)
				}
				if cli.ReloadCmdTimeout != 90*time.Second {
					t.Errorf("ReloadCmdTimeout = %v, want 1m30s", cli.ReloadCmdTimeout)
				}
				if cli.ShutdownTimeout != 20*time.Second {
					t.Errorf("ShutdownTimeout = %v, want 20s", cli.ShutdownTimeout)
				}
				if cli.DebounceStr != "2s" {
					t.Errorf("DebounceStr = %q, want 2s", cli.DebounceStr)
				}
				if !cli.SystemdNotify {
					t.Error("SystemdNotify = false, want true")
				}
				if cli.WatchdogInterval != 25*time.Second {
					t.Errorf("WatchdogInterval = %v, want 25s", cli.WatchdogInterval)
				}
				if cli.LogLevel != "debug" {
					t.Errorf("LogLevel = %q, want debug", cli.LogLevel)
				}
				if cli.LogFormat != "json" {
					t.Errorf("LogFormat = %q, want json", cli.LogFormat)
				}
			},
		},
		{
			name: "cli backend_timeout beats toml",
			args: []string{"--backend-timeout", "30s"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.BackendTimeout != 30*time.Second {
					t.Errorf("BackendTimeout = %v, want 30s", cli.BackendTimeout)
				}
			},
		},
		{
			name: "env backend_timeout beats toml",
			env:  map[string]string{"CONFD_BACKEND_TIMEOUT": "30s"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.BackendTimeout != 30*time.Second {
					t.Errorf("BackendTimeout = %v, want 30s", cli.BackendTimeout)
				}
			},
		},
		{
			name: "cli check_cmd_timeout beats toml",
			args: []string{"--check-cmd-timeout", "30s"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.CheckCmdTimeout != 30*time.Second {
					t.Errorf("CheckCmdTimeout = %v, want 30s", cli.CheckCmdTimeout)
				}
			},
		},
		{
			name: "env check_cmd_timeout beats toml",
			env:  map[string]string{"CONFD_CHECK_CMD_TIMEOUT": "30s"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.CheckCmdTimeout != 30*time.Second {
					t.Errorf("CheckCmdTimeout = %v, want 30s", cli.CheckCmdTimeout)
				}
			},
		},
		{
			name: "cli reload_cmd_timeout beats toml",
			args: []string{"--reload-cmd-timeout", "60s"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.ReloadCmdTimeout != 60*time.Second {
					t.Errorf("ReloadCmdTimeout = %v, want 1m0s", cli.ReloadCmdTimeout)
				}
			},
		},
		{
			name: "env reload_cmd_timeout beats toml",
			env:  map[string]string{"CONFD_RELOAD_CMD_TIMEOUT": "60s"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.ReloadCmdTimeout != 60*time.Second {
					t.Errorf("ReloadCmdTimeout = %v, want 1m0s", cli.ReloadCmdTimeout)
				}
			},
		},
		{
			name: "cli shutdown_timeout beats toml",
			args: []string{"--shutdown-timeout", "30s"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.ShutdownTimeout != 30*time.Second {
					t.Errorf("ShutdownTimeout = %v, want 30s", cli.ShutdownTimeout)
				}
			},
		},
		{
			name: "env shutdown_timeout beats toml",
			env:  map[string]string{"CONFD_SHUTDOWN_TIMEOUT": "30s"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.ShutdownTimeout != 30*time.Second {
					t.Errorf("ShutdownTimeout = %v, want 30s", cli.ShutdownTimeout)
				}
			},
		},
		{
			name: "cli debounce beats toml",
			args: []string{"--debounce", "500ms"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.DebounceStr != "500ms" {
					t.Errorf("DebounceStr = %q, want 500ms", cli.DebounceStr)
				}
			},
		},
		{
			name: "env debounce beats toml",
			env:  map[string]string{"CONFD_DEBOUNCE": "500ms"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.DebounceStr != "500ms" {
					t.Errorf("DebounceStr = %q, want 500ms", cli.DebounceStr)
				}
			},
		},
		{
			name: "cli systemd_notify beats toml",
			args: []string{"--systemd-notify=false"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.SystemdNotify {
					t.Error("SystemdNotify = true, want false")
				}
			},
		},
		{
			name: "env systemd_notify beats toml",
			env:  map[string]string{"CONFD_SYSTEMD_NOTIFY": "false"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.SystemdNotify {
					t.Error("SystemdNotify = true, want false")
				}
			},
		},
		{
			name: "cli watchdog_interval beats toml",
			args: []string{"--watchdog-interval", "10s"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.WatchdogInterval != 10*time.Second {
					t.Errorf("WatchdogInterval = %v, want 10s", cli.WatchdogInterval)
				}
			},
		},
		{
			name: "env watchdog_interval beats toml",
			env:  map[string]string{"CONFD_WATCHDOG_INTERVAL": "10s"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.WatchdogInterval != 10*time.Second {
					t.Errorf("WatchdogInterval = %v, want 10s", cli.WatchdogInterval)
				}
			},
		},
		{
			name: "cli log_level beats toml",
			args: []string{"--log-level", "warn"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.LogLevel != "warn" {
					t.Errorf("LogLevel = %q, want warn", cli.LogLevel)
				}
			},
		},
		{
			name: "env log_level beats toml",
			env:  map[string]string{"CONFD_LOG_LEVEL": "warn"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.LogLevel != "warn" {
					t.Errorf("LogLevel = %q, want warn", cli.LogLevel)
				}
			},
		},
		{
			name: "cli log_format beats toml",
			args: []string{"--log-format", "text"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.LogFormat != "text" {
					t.Errorf("LogFormat = %q, want text", cli.LogFormat)
				}
			},
		},
		{
			name: "env log_format beats toml",
			env:  map[string]string{"CONFD_LOG_FORMAT": "text"},
			check: func(t *testing.T, cli *CLI) {
				t.Helper()
				if cli.LogFormat != "text" {
					t.Errorf("LogFormat = %q, want text", cli.LogFormat)
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
			configFile := writeConfigFile(t, configTOML)
			args := append([]string{"--config-file", configFile}, tt.args...)
			cli, _ := parseCLI(t, append(args, "env"))

			if err := loadConfigFile(cli, &backends.Config{}); err != nil {
				t.Fatalf("loadConfigFile failed: %v", err)
			}
			tt.check(t, cli)
		})
	}
}

func TestLoadConfigFile_HyphenatedLogKeys(t *testing.T) {
	clearCONFDEnvVars(t)

	configFile := writeConfigFile(t, `
log-level = "debug"
log-format = "json"
`)
	cli, _ := parseCLI(t, []string{"--config-file", configFile, "env"})
	if err := loadConfigFile(cli, &backends.Config{}); err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}
	if cli.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cli.LogLevel)
	}
	if cli.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want json", cli.LogFormat)
	}
}

func TestLoadConfigFile_DocumentedKeysInvalidDurations(t *testing.T) {
	keys := []string{
		"backend_timeout", "check_cmd_timeout", "reload_cmd_timeout",
		"shutdown_timeout", "debounce", "watchdog_interval",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			clearCONFDEnvVars(t)

			configFile := writeConfigFile(t, key+` = "not-a-duration"`)
			cli, _ := parseCLI(t, []string{"--config-file", configFile, "env"})

			err := loadConfigFile(cli, &backends.Config{})
			if err == nil {
				t.Fatalf("expected error for invalid %s", key)
			}
			if !strings.Contains(err.Error(), "invalid "+key) {
				t.Errorf("error = %q, want it to name %s", err, key)
			}
		})
	}
}
