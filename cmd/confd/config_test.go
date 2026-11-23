package main

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestLoadConfigFile_BothLogKeySpellings(t *testing.T) {
	tests := []struct {
		name           string
		underscoreLine string
		hyphenatedLine string
		wantLogLevel   string
		wantLogFormat  string
	}{
		{
			name:           "log_level",
			underscoreLine: `log_level = "error"`,
			hyphenatedLine: `log-level = "debug"`,
			wantLogLevel:   "error",
		},
		{
			name:           "log_format",
			underscoreLine: `log_format = "json"`,
			hyphenatedLine: `log-format = "text"`,
			wantLogFormat:  "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearCONFDEnvVars(t)

			// The underscore spelling wins wherever it sits in the file.
			fileContents := []string{
				tt.underscoreLine + "\n" + tt.hyphenatedLine + "\n",
				tt.hyphenatedLine + "\n" + tt.underscoreLine + "\n",
			}
			for _, fileContent := range fileContents {
				configFile := writeConfigFile(t, fileContent)
				cli, _ := parseCLI(t, []string{"--config-file", configFile, "env"})
				if err := loadConfigFile(cli, &backends.Config{}); err != nil {
					t.Fatalf("loadConfigFile failed for %q: %v", fileContent, err)
				}
				if cli.LogLevel != tt.wantLogLevel {
					t.Errorf("file %q: LogLevel = %q, want %q", fileContent, cli.LogLevel, tt.wantLogLevel)
				}
				if cli.LogFormat != tt.wantLogFormat {
					t.Errorf("file %q: LogFormat = %q, want %q", fileContent, cli.LogFormat, tt.wantLogFormat)
				}
			}
		})
	}
}

func TestLoadConfigFile_HyphenatedLogKeysOverridden(t *testing.T) {
	tests := []struct {
		name          string
		fileContent   string
		args          []string
		env           map[string]string
		wantLogLevel  string
		wantLogFormat string
	}{
		{
			name:         "flag_log_level",
			fileContent:  `log-level = "debug"`,
			args:         []string{"--log-level", "error"},
			wantLogLevel: "error",
		},
		{
			name:          "flag_log_format",
			fileContent:   `log-format = "text"`,
			args:          []string{"--log-format", "json"},
			wantLogFormat: "json",
		},
		{
			name:         "env_log_level",
			fileContent:  `log-level = "debug"`,
			env:          map[string]string{"CONFD_LOG_LEVEL": "error"},
			wantLogLevel: "error",
		},
		{
			name:          "env_log_format",
			fileContent:   `log-format = "text"`,
			env:           map[string]string{"CONFD_LOG_FORMAT": "json"},
			wantLogFormat: "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearCONFDEnvVars(t)
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			configFile := writeConfigFile(t, tt.fileContent)
			args := append([]string{"--config-file", configFile}, tt.args...)
			cli, _ := parseCLI(t, append(args, "env"))
			if err := loadConfigFile(cli, &backends.Config{}); err != nil {
				t.Fatalf("loadConfigFile failed: %v", err)
			}
			if cli.LogLevel != tt.wantLogLevel {
				t.Errorf("LogLevel = %q, want %q", cli.LogLevel, tt.wantLogLevel)
			}
			if cli.LogFormat != tt.wantLogFormat {
				t.Errorf("LogFormat = %q, want %q", cli.LogFormat, tt.wantLogFormat)
			}
		})
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

func TestLoadConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "confd.toml")
	configContent := `
confdir = "/custom/confd"
interval = 300
prefix = "/myprefix"
log-level = "debug"
noop = true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cli := &CLI{
		ConfDir:    "/etc/confd", // default
		ConfigFile: configPath,
		Interval:   600, // default
	}
	backendCfg := &backends.Config{}

	if err := loadConfigFile(cli, backendCfg); err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}

	// Verify TOML values are applied
	if cli.ConfDir != "/custom/confd" {
		t.Errorf("Expected confdir '/custom/confd', got %q", cli.ConfDir)
	}
	if cli.Interval != 300 {
		t.Errorf("Expected interval 300, got %d", cli.Interval)
	}
	if cli.Prefix != "/myprefix" {
		t.Errorf("Expected prefix '/myprefix', got %q", cli.Prefix)
	}
	if cli.LogLevel != "debug" {
		t.Errorf("Expected log-level 'debug', got %q", cli.LogLevel)
	}
	if !cli.Noop {
		t.Error("Expected noop to be true")
	}
}

func TestLoadConfigFileCLIPrecedence(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "confd.toml")
	configContent := `
confdir = "/toml/confd"
interval = 300
prefix = "/tomlprefix"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// CLI values should take precedence over TOML
	cli := &CLI{
		ConfDir:    "/cli/confd", // Non-default CLI value
		ConfigFile: configPath,
		Interval:   120,          // Non-default CLI value
		Prefix:     "/cliprefix", // CLI value
		configSources: &configSources{cli: map[string]bool{
			"confdir":  true,
			"interval": true,
			"prefix":   true,
		}},
	}
	backendCfg := &backends.Config{}

	if err := loadConfigFile(cli, backendCfg); err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}

	// CLI values should be preserved (not overwritten by TOML)
	if cli.ConfDir != "/cli/confd" {
		t.Errorf("Expected confdir '/cli/confd', got %q", cli.ConfDir)
	}
	if cli.Interval != 120 {
		t.Errorf("Expected interval 120, got %d", cli.Interval)
	}
	if cli.Prefix != "/cliprefix" {
		t.Errorf("Expected prefix '/cliprefix', got %q", cli.Prefix)
	}
}

func TestLoadConfigFileBackendSettings(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "confd.toml")
	configContent := `
nodes = ["node1:2379", "node2:2379"]
client_cert = "/path/to/cert"
client_key = "/path/to/key"
username = "admin"
password = "secret"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cli := &CLI{
		ConfDir:    "/etc/confd",
		ConfigFile: configPath,
		Interval:   600,
	}
	backendCfg := &backends.Config{}

	if err := loadConfigFile(cli, backendCfg); err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}

	if len(backendCfg.BackendNodes) != 2 {
		t.Fatalf("Expected 2 backend nodes, got %d", len(backendCfg.BackendNodes))
	}
	if backendCfg.BackendNodes[0] != "node1:2379" {
		t.Errorf("Expected first node 'node1:2379', got %q", backendCfg.BackendNodes[0])
	}
	if backendCfg.ClientCert != "/path/to/cert" {
		t.Errorf("Expected client_cert '/path/to/cert', got %q", backendCfg.ClientCert)
	}
	if backendCfg.Username != "admin" {
		t.Errorf("Expected username 'admin', got %q", backendCfg.Username)
	}
}
func TestProcessEnv(t *testing.T) {
	t.Setenv("CONFD_CLIENT_CERT", "/env/cert")
	t.Setenv("CONFD_CLIENT_KEY", "/env/key")
	t.Setenv("CONFD_CLIENT_CAKEYS", "/env/ca")
	// ACM_EXPORT_PRIVATE_KEY is parsed by Kong; processEnv must leave it alone.
	t.Setenv("ACM_EXPORT_PRIVATE_KEY", "true")

	cfg := &backends.Config{}
	processEnv(cfg)

	if cfg.ClientCert != "/env/cert" {
		t.Errorf("Expected ClientCert '/env/cert', got %q", cfg.ClientCert)
	}
	if cfg.ClientKey != "/env/key" {
		t.Errorf("Expected ClientKey '/env/key', got %q", cfg.ClientKey)
	}
	if cfg.ClientCaKeys != "/env/ca" {
		t.Errorf("Expected ClientCaKeys '/env/ca', got %q", cfg.ClientCaKeys)
	}
	if cfg.ACMExportPrivateKey {
		t.Error("Expected processEnv to leave ACMExportPrivateKey false")
	}
}

func TestProcessEnvDoesNotOverride(t *testing.T) {
	// Save and restore environment
	origCert := os.Getenv("CONFD_CLIENT_CERT")
	defer os.Setenv("CONFD_CLIENT_CERT", origCert)

	os.Setenv("CONFD_CLIENT_CERT", "/env/cert")

	// Pre-set value should not be overridden
	cfg := &backends.Config{
		ClientCert: "/cli/cert",
	}
	processEnv(cfg)

	if cfg.ClientCert != "/cli/cert" {
		t.Errorf("Expected ClientCert '/cli/cert' (not overridden), got %q", cfg.ClientCert)
	}
}

// resolveConfig merges confd.toml, backend defaults and the environment into
// cli and backendCfg in the same order as run().
func resolveConfig(t *testing.T, cli *CLI, backendCfg *backends.Config) {
	t.Helper()

	if err := loadConfigFile(cli, backendCfg); err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}
	applyBackendDefaults(backendCfg)
	processEnv(backendCfg)
}

// TestConfigPrecedence_Representative walks one option of each kind through
// the precedence tiers: confd.toml alone, an env var over confd.toml, and a
// flag over an env var. Slice options have no env vars, so they cover
// confd.toml, a flag over confd.toml, and the built-in default.
func TestConfigPrecedence_Representative(t *testing.T) {
	consulConfig := func(cli *CLI) backends.Config {
		return backends.Config{Backend: "consul", BackendNodes: cli.Consul.Node, Scheme: cli.Consul.Scheme}
	}
	vaultConfig := func(cli *CLI) backends.Config {
		return backends.Config{Backend: "vault", BackendNodes: cli.Vault.Node, AuthToken: cli.Vault.AuthToken}
	}
	acmConfig := func(cli *CLI) backends.Config {
		return backends.Config{Backend: "acm", ACMExportPrivateKey: cli.ACM.ExportPrivateKey}
	}
	secretsManagerConfig := func(cli *CLI) backends.Config {
		return backends.Config{
			Backend:                    "secretsmanager",
			SecretsManagerVersionStage: cli.SecretsManager.VersionStage,
			SecretsManagerNoFlatten:    cli.SecretsManager.NoFlatten,
		}
	}

	noop := func(cli *CLI, _ *backends.Config) any { return cli.Noop }
	templateCache := func(cli *CLI, _ *backends.Config) any { return cli.TemplateCache }
	prefix := func(cli *CLI, _ *backends.Config) any { return cli.Prefix }
	retryMaxDelay := func(cli *CLI, _ *backends.Config) any { return cli.RetryMaxDelay }
	nodes := func(_ *CLI, cfg *backends.Config) any { return []string(cfg.BackendNodes) }
	scheme := func(_ *CLI, cfg *backends.Config) any { return cfg.Scheme }
	authToken := func(_ *CLI, cfg *backends.Config) any { return cfg.AuthToken }
	acmExport := func(_ *CLI, cfg *backends.Config) any { return cfg.ACMExportPrivateKey }
	versionStage := func(_ *CLI, cfg *backends.Config) any { return cfg.SecretsManagerVersionStage }
	noFlatten := func(_ *CLI, cfg *backends.Config) any { return cfg.SecretsManagerNoFlatten }

	tests := []struct {
		name string
		toml string
		env  map[string]string
		// args follow --config-file and include the backend subcommand
		args []string
		// backend builds the backend config from the parsed command the way
		// that command's Run method does; nil means the env backend.
		backend func(*CLI) backends.Config
		got     func(*CLI, *backends.Config) any
		want    any
	}{
		// bool, resolving to true
		{name: "bool true/toml only", toml: `noop = true`, args: []string{"env"}, got: noop, want: true},
		{name: "bool true/env over toml", toml: `noop = false`, env: map[string]string{"CONFD_NOOP": "true"}, args: []string{"env"}, got: noop, want: true},
		{name: "bool true/flag over env", toml: `noop = false`, env: map[string]string{"CONFD_NOOP": "false"}, args: []string{"--noop", "env"}, got: noop, want: true},

		// bool, resolving to false
		{name: "bool false/toml only", toml: `noop = false`, args: []string{"env"}, got: noop, want: false},
		{name: "bool false/env over toml", toml: `noop = true`, env: map[string]string{"CONFD_NOOP": "false"}, args: []string{"env"}, got: noop, want: false},
		{name: "bool false/flag over env", toml: `noop = true`, env: map[string]string{"CONFD_NOOP": "true"}, args: []string{"--noop=false", "env"}, got: noop, want: false},

		// bool that defaults to true
		{name: "default-true bool/unset", args: []string{"env"}, got: templateCache, want: true},
		{name: "default-true bool/toml only", toml: `template_cache = false`, args: []string{"env"}, got: templateCache, want: false},
		{name: "default-true bool/env over toml", toml: `template_cache = false`, env: map[string]string{"CONFD_TEMPLATE_CACHE": "true"}, args: []string{"env"}, got: templateCache, want: true},
		{name: "default-true bool/flag over env", toml: `template_cache = true`, env: map[string]string{"CONFD_TEMPLATE_CACHE": "true"}, args: []string{"--no-template-cache", "env"}, got: templateCache, want: false},

		// string
		{name: "string/toml only", toml: `prefix = "/toml"`, args: []string{"env"}, got: prefix, want: "/toml"},
		{name: "string/env over toml", toml: `prefix = "/toml"`, env: map[string]string{"CONFD_PREFIX": "/env"}, args: []string{"env"}, got: prefix, want: "/env"},
		{name: "string/flag over env", toml: `prefix = "/toml"`, env: map[string]string{"CONFD_PREFIX": "/env"}, args: []string{"--prefix", "/flag", "env"}, got: prefix, want: "/flag"},

		// duration
		{name: "duration/unset", args: []string{"env"}, got: retryMaxDelay, want: DefaultRetryMaxDelay},
		{name: "duration/toml only", toml: `retry_max_delay = "10s"`, args: []string{"env"}, got: retryMaxDelay, want: 10 * time.Second},
		{name: "duration/env over toml", toml: `retry_max_delay = "10s"`, env: map[string]string{"CONFD_RETRY_MAX_DELAY": "20s"}, args: []string{"env"}, got: retryMaxDelay, want: 20 * time.Second},
		{name: "duration/flag over env", toml: `retry_max_delay = "10s"`, env: map[string]string{"CONFD_RETRY_MAX_DELAY": "20s"}, args: []string{"--retry-max-delay", "40s", "env"}, got: retryMaxDelay, want: 40 * time.Second},

		// slice
		{name: "slice/built-in default", args: []string{"consul"}, backend: consulConfig, got: nodes, want: []string{"127.0.0.1:8500"}},
		{name: "slice/toml only", toml: `nodes = ["toml-a:8500", "toml-b:8500"]`, args: []string{"consul"}, backend: consulConfig, got: nodes, want: []string{"toml-a:8500", "toml-b:8500"}},
		{name: "slice/flag over toml", toml: `nodes = ["toml-a:8500", "toml-b:8500"]`, args: []string{"consul", "--node", "flag:8500"}, backend: consulConfig, got: nodes, want: []string{"flag:8500"}},

		// backend-specific string with an env var
		{name: "vault auth token/toml only", toml: `auth_token = "toml-token"`, args: []string{"vault"}, backend: vaultConfig, got: authToken, want: "toml-token"},
		{name: "vault auth token/env over toml", toml: `auth_token = "toml-token"`, env: map[string]string{"VAULT_TOKEN": "env-token"}, args: []string{"vault"}, backend: vaultConfig, got: authToken, want: "env-token"},
		{name: "vault auth token/flag over env", toml: `auth_token = "toml-token"`, env: map[string]string{"VAULT_TOKEN": "env-token"}, args: []string{"vault", "--auth-token", "flag-token"}, backend: vaultConfig, got: authToken, want: "flag-token"},

		// backend-specific string whose flag has a default
		{name: "scheme/command default", args: []string{"consul"}, backend: consulConfig, got: scheme, want: "http"},
		{name: "scheme/toml over command default", toml: `scheme = "https"`, args: []string{"consul"}, backend: consulConfig, got: scheme, want: "https"},
		{name: "scheme/flag set to default over toml", toml: `scheme = "https"`, args: []string{"consul", "--scheme", "http"}, backend: consulConfig, got: scheme, want: "http"},
		{name: "secrets manager version stage/toml over command default", toml: `secretsmanager_version_stage = "AWSPREVIOUS"`, args: []string{"secretsmanager"}, backend: secretsManagerConfig, got: versionStage, want: "AWSPREVIOUS"},
		{name: "secrets manager version stage/flag set to default over toml", toml: `secretsmanager_version_stage = "AWSPREVIOUS"`, args: []string{"secretsmanager", "--secretsmanager-version-stage", "AWSCURRENT"}, backend: secretsManagerConfig, got: versionStage, want: "AWSCURRENT"},

		// backend-specific bool
		{name: "secrets manager no-flatten/toml only", toml: `secretsmanager_no_flatten = true`, args: []string{"secretsmanager"}, backend: secretsManagerConfig, got: noFlatten, want: true},
		{name: "secrets manager no-flatten/flag false over toml", toml: `secretsmanager_no_flatten = true`, args: []string{"secretsmanager", "--secretsmanager-no-flatten=false"}, backend: secretsManagerConfig, got: noFlatten, want: false},
		{name: "acm export private key/toml only", toml: `acm_export_private_key = true`, args: []string{"acm"}, backend: acmConfig, got: acmExport, want: true},
		{name: "acm export private key/env true over toml", toml: `acm_export_private_key = false`, env: map[string]string{"ACM_EXPORT_PRIVATE_KEY": "true"}, args: []string{"acm"}, backend: acmConfig, got: acmExport, want: true},
		{name: "acm export private key/flag over env", toml: `acm_export_private_key = false`, env: map[string]string{"ACM_EXPORT_PRIVATE_KEY": "false"}, args: []string{"acm", "--acm-export-private-key"}, backend: acmConfig, got: acmExport, want: true},
		{name: "acm export private key/env false over toml", toml: `acm_export_private_key = true`, env: map[string]string{"ACM_EXPORT_PRIVATE_KEY": "false"}, args: []string{"acm"}, backend: acmConfig, got: acmExport, want: false},
		{name: "acm export private key/flag false over env", toml: `acm_export_private_key = true`, env: map[string]string{"ACM_EXPORT_PRIVATE_KEY": "true"}, args: []string{"acm", "--acm-export-private-key=false"}, backend: acmConfig, got: acmExport, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearCONFDEnvVars(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			configFile := writeConfigFile(t, tt.toml)
			cli, _ := parseCLI(t, append([]string{"--config-file", configFile}, tt.args...))

			backendCfg := backends.Config{Backend: "env"}
			if tt.backend != nil {
				backendCfg = tt.backend(cli)
			}
			resolveConfig(t, cli, &backendCfg)

			if got := tt.got(cli, &backendCfg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestACMExportPrivateKeyEnvValues checks that ACM_EXPORT_PRIVATE_KEY is read
// as a boolean rather than as "set means true", and that an explicit flag wins
// over it in both directions.
func TestACMExportPrivateKeyEnvValues(t *testing.T) {
	tests := []struct {
		name string
		// env is the value of ACM_EXPORT_PRIVATE_KEY; nil leaves it unset
		env  *string
		flag []string
		want bool
	}{
		{name: "env_false", env: new("false"), want: false},
		{name: "env_0", env: new("0"), want: false},
		{name: "env_true", env: new("true"), want: true},
		{name: "env_unset", want: false},
		{name: "env_true_flag_false", env: new("true"), flag: []string{"--acm-export-private-key=false"}, want: false},
		{name: "env_false_flag_true", env: new("false"), flag: []string{"--acm-export-private-key"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearCONFDEnvVars(t)
			if tt.env != nil {
				t.Setenv("ACM_EXPORT_PRIVATE_KEY", *tt.env)
			}
			configFile := writeConfigFile(t, "")
			cli, _ := parseCLI(t, append([]string{"--config-file", configFile, "acm"}, tt.flag...))

			backendCfg := backends.Config{Backend: "acm", ACMExportPrivateKey: cli.ACM.ExportPrivateKey}
			resolveConfig(t, cli, &backendCfg)

			if backendCfg.ACMExportPrivateKey != tt.want {
				t.Errorf("ACMExportPrivateKey = %v, want %v", backendCfg.ACMExportPrivateKey, tt.want)
			}
		})
	}
}
