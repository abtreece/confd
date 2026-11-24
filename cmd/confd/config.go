package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
	"github.com/alecthomas/kong"
)

// TOMLConfig represents the structure of the confd TOML config file
type TOMLConfig struct {
	ConfDir       string   `toml:"confdir"`
	Interval      int      `toml:"interval"`
	Noop          bool     `toml:"noop"`
	Prefix        string   `toml:"prefix"`
	SyncOnly      bool     `toml:"sync_only"`
	LogLevel      string   `toml:"log_level"`
	LogFormat     string   `toml:"log_format"`
	Watch         bool     `toml:"watch"`
	FailureMode   string   `toml:"failure_mode"`
	KeepStageFile bool     `toml:"keep_stage_file"`
	SRVDomain     string   `toml:"srv_domain"`
	SRVRecord     string   `toml:"srv_record"`
	Nodes         []string `toml:"nodes"`

	// Hyphenated spellings of log_level and log_format, kept for existing config files
	LogLevelHyphenated  string `toml:"log-level"`
	LogFormatHyphenated string `toml:"log-format"`

	// Backend-specific settings
	AuthToken      string   `toml:"auth_token"`
	AuthType       string   `toml:"auth_type"`
	BasicAuth      bool     `toml:"basic_auth"`
	ClientCaKeys   string   `toml:"client_cakeys"`
	ClientCert     string   `toml:"client_cert"`
	ClientKey      string   `toml:"client_key"`
	ClientInsecure bool     `toml:"client_insecure"`
	Password       string   `toml:"password"`
	Scheme         string   `toml:"scheme"`
	Table          string   `toml:"table"`
	Separator      string   `toml:"separator"`
	Username       string   `toml:"username"`
	AppID          string   `toml:"app_id"`
	UserID         string   `toml:"user_id"`
	RoleID         string   `toml:"role_id"`
	SecretID       string   `toml:"secret_id"`
	File           []string `toml:"file"`
	Filter         string   `toml:"filter"`
	Path           string   `toml:"path"`

	ACMExportPrivateKey        bool   `toml:"acm_export_private_key"`
	SecretsManagerVersionStage string `toml:"secretsmanager_version_stage"`
	SecretsManagerNoFlatten    bool   `toml:"secretsmanager_no_flatten"`
	IMDSCacheTTL               string `toml:"imds_cache_ttl"`

	// Performance settings
	TemplateCache *bool  `toml:"template_cache"` // Pointer to distinguish unset from false
	StatCacheTTL  string `toml:"stat_cache_ttl"`

	// Operation timeouts
	BackendTimeout   string `toml:"backend_timeout"`
	CheckCmdTimeout  string `toml:"check_cmd_timeout"`
	ReloadCmdTimeout string `toml:"reload_cmd_timeout"`
	ShutdownTimeout  string `toml:"shutdown_timeout"`

	// Connection timeouts
	DialTimeout  string `toml:"dial_timeout"`
	ReadTimeout  string `toml:"read_timeout"`
	WriteTimeout string `toml:"write_timeout"`

	// Retry configuration
	RetryMaxAttempts int    `toml:"retry_max_attempts"`
	RetryBaseDelay   string `toml:"retry_base_delay"`
	RetryMaxDelay    string `toml:"retry_max_delay"`

	// Watch mode timeouts
	WatchErrorBackoff string `toml:"watch_error_backoff"`

	// Watch mode debounce and batching
	Debounce      string `toml:"debounce"`
	BatchInterval string `toml:"batch_interval"`

	// Preflight timeout
	PreflightTimeout string `toml:"preflight_timeout"`

	// Systemd integration
	SystemdNotify    bool   `toml:"systemd_notify"`
	WatchdogInterval string `toml:"watchdog_interval"`

	// Metrics and observability
	MetricsAddr string `toml:"metrics_addr"`
}

// configSources records where each option was supplied. cli and env are
// filled while parsing and decide whether a confd.toml value may be applied;
// toml is filled when the config file is loaded.
type configSources struct {
	cli  map[string]bool
	env  map[string]bool
	toml map[string]bool
}

func newConfigSources(ctx *kong.Context) *configSources {
	sources := &configSources{
		cli:  make(map[string]bool),
		env:  make(map[string]bool),
		toml: make(map[string]bool),
	}
	for _, path := range ctx.Path {
		if path.Flag == nil {
			continue
		}
		name := normalizeConfigName(path.Flag.Name)
		if path.Resolved {
			sources.env[name] = true
			continue
		}
		sources.cli[name] = true
	}
	// A string flag's variable that is set to the empty string is not in
	// flag.Envs (see ignoreEmptyStringEnvs), so it is not recorded here.
	for _, flag := range ctx.Flags() {
		for _, env := range flag.Envs {
			if _, ok := os.LookupEnv(env); ok {
				sources.env[normalizeConfigName(flag.Name)] = true
				break
			}
		}
	}
	return sources
}

func normalizeConfigName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "-", "_")
}

func (s *configSources) isExplicit(name string) bool {
	if s == nil {
		return false
	}
	name = normalizeConfigName(name)
	return s.cli[name] || s.env[name]
}

// recordTOML notes every top-level key the config file defines.
func (s *configSources) recordTOML(md toml.MetaData) {
	if s == nil {
		return
	}
	if s.toml == nil {
		s.toml = make(map[string]bool)
	}
	for _, key := range md.Keys() {
		if len(key) == 1 {
			s.toml[key[0]] = true
		}
	}
}

// isSupplied reports whether the operator supplied the option at all: on the
// command line, through its environment variable, or as a key in confd.toml.
// An option supplied with its default value still counts.
func (s *configSources) isSupplied(name string) bool {
	if s == nil {
		return false
	}
	return s.isExplicit(name) || s.toml[normalizeConfigName(name)]
}

// warnIgnoredBackendFlags logs a warning for each setting supplied on the
// command line, through its environment variable or in confd.toml that the
// backend does not use (backends.IgnoredSettingWarnings). cfg must be the
// resolved config, after confd.toml and flags.
func warnIgnoredBackendFlags(sources *configSources, cfg backends.Config) {
	for _, warning := range backends.IgnoredSettingWarnings(cfg, sources.isSupplied) {
		log.Warning("%s", warning)
	}
}

func tomlDefined(md toml.MetaData, names ...string) bool {
	for _, name := range names {
		if md.IsDefined(name) {
			return true
		}
	}
	return false
}

func shouldApplyTOML(s *configSources, md toml.MetaData, name string, aliases ...string) bool {
	names := append([]string{name}, aliases...)
	for _, candidate := range names {
		if s.isExplicit(candidate) {
			return false
		}
	}
	return tomlDefined(md, names...)
}

// loadConfigFile loads the TOML config file and applies defaults to CLI and backend config
func loadConfigFile(cli *CLI, backendCfg *backends.Config) error {
	_, err := os.Stat(cli.ConfigFile)
	if os.IsNotExist(err) {
		return nil
	}

	configBytes, err := os.ReadFile(cli.ConfigFile)
	if err != nil {
		return fmt.Errorf("read config file %s: %w", cli.ConfigFile, err)
	}

	var tomlCfg TOMLConfig
	md, err := toml.Decode(string(configBytes), &tomlCfg)
	if err != nil {
		return fmt.Errorf("decode config file %s: %w", cli.ConfigFile, err)
	}

	sources := cli.configSources
	sources.recordTOML(md)

	// Apply TOML settings as defaults (CLI flags take precedence)
	// Global settings
	if shouldApplyTOML(sources, md, "confdir") && tomlCfg.ConfDir != "" {
		cli.ConfDir = tomlCfg.ConfDir
	}
	if shouldApplyTOML(sources, md, "interval") && tomlCfg.Interval != 0 {
		cli.Interval = tomlCfg.Interval
	}
	if shouldApplyTOML(sources, md, "noop") {
		cli.Noop = tomlCfg.Noop
	}
	if shouldApplyTOML(sources, md, "prefix") && tomlCfg.Prefix != "" {
		cli.Prefix = tomlCfg.Prefix
	}
	if shouldApplyTOML(sources, md, "sync_only") {
		cli.SyncOnly = tomlCfg.SyncOnly
	}
	if shouldApplyTOML(sources, md, "log_level", "log-level") {
		if tomlCfg.LogLevel != "" {
			cli.LogLevel = tomlCfg.LogLevel
		} else if tomlCfg.LogLevelHyphenated != "" {
			cli.LogLevel = tomlCfg.LogLevelHyphenated
		}
	}
	if shouldApplyTOML(sources, md, "log_format", "log-format") {
		if tomlCfg.LogFormat != "" {
			cli.LogFormat = tomlCfg.LogFormat
		} else if tomlCfg.LogFormatHyphenated != "" {
			cli.LogFormat = tomlCfg.LogFormatHyphenated
		}
	}
	if shouldApplyTOML(sources, md, "watch") {
		cli.Watch = tomlCfg.Watch
	}
	if shouldApplyTOML(sources, md, "failure_mode") && tomlCfg.FailureMode != "" {
		cli.FailureMode = tomlCfg.FailureMode
	}
	if shouldApplyTOML(sources, md, "keep_stage_file") {
		cli.KeepStageFile = tomlCfg.KeepStageFile
	}
	if shouldApplyTOML(sources, md, "template_cache") && tomlCfg.TemplateCache != nil {
		cli.TemplateCache = *tomlCfg.TemplateCache
	}
	// Stat cache TTL
	if shouldApplyTOML(sources, md, "stat_cache_ttl") && tomlCfg.StatCacheTTL != "" {
		d, err := time.ParseDuration(tomlCfg.StatCacheTTL)
		if err != nil {
			return fmt.Errorf("invalid stat_cache_ttl %q: %w (use Go duration format, e.g., \"1m\", \"5m\")", tomlCfg.StatCacheTTL, err)
		}
		cli.StatCacheTTL = d
	}
	if shouldApplyTOML(sources, md, "srv_domain") && tomlCfg.SRVDomain != "" {
		cli.SRVDomain = tomlCfg.SRVDomain
	}
	if shouldApplyTOML(sources, md, "srv_record") && tomlCfg.SRVRecord != "" {
		cli.SRVRecord = tomlCfg.SRVRecord
	}
	if shouldApplyTOML(sources, md, "metrics_addr") && tomlCfg.MetricsAddr != "" {
		cli.MetricsAddr = tomlCfg.MetricsAddr
	}

	// Backend settings (only apply if not already set via CLI)
	if shouldApplyTOML(sources, md, "nodes", "node") && len(tomlCfg.Nodes) > 0 {
		backendCfg.BackendNodes = tomlCfg.Nodes
	}
	if shouldApplyTOML(sources, md, "auth_token") && tomlCfg.AuthToken != "" {
		backendCfg.AuthToken = tomlCfg.AuthToken
	}
	if shouldApplyTOML(sources, md, "auth_type") && tomlCfg.AuthType != "" {
		backendCfg.AuthType = tomlCfg.AuthType
	}
	if shouldApplyTOML(sources, md, "basic_auth") {
		backendCfg.BasicAuth = tomlCfg.BasicAuth
	}
	if shouldApplyTOML(sources, md, "client_cakeys", "client_ca_keys") && tomlCfg.ClientCaKeys != "" {
		backendCfg.ClientCaKeys = tomlCfg.ClientCaKeys
	}
	if shouldApplyTOML(sources, md, "client_cert") && tomlCfg.ClientCert != "" {
		backendCfg.ClientCert = tomlCfg.ClientCert
	}
	if shouldApplyTOML(sources, md, "client_key") && tomlCfg.ClientKey != "" {
		backendCfg.ClientKey = tomlCfg.ClientKey
	}
	if shouldApplyTOML(sources, md, "client_insecure") {
		backendCfg.ClientInsecure = tomlCfg.ClientInsecure
	}
	if shouldApplyTOML(sources, md, "password") && tomlCfg.Password != "" {
		backendCfg.Password = tomlCfg.Password
	}
	if shouldApplyTOML(sources, md, "scheme") && tomlCfg.Scheme != "" {
		backendCfg.Scheme = tomlCfg.Scheme
	}
	if shouldApplyTOML(sources, md, "table") && tomlCfg.Table != "" {
		backendCfg.Table = tomlCfg.Table
	}
	if shouldApplyTOML(sources, md, "separator") && tomlCfg.Separator != "" {
		backendCfg.Separator = tomlCfg.Separator
	}
	if shouldApplyTOML(sources, md, "username") && tomlCfg.Username != "" {
		backendCfg.Username = tomlCfg.Username
	}
	if shouldApplyTOML(sources, md, "app_id") && tomlCfg.AppID != "" {
		backendCfg.AppID = tomlCfg.AppID
	}
	if shouldApplyTOML(sources, md, "user_id") && tomlCfg.UserID != "" {
		backendCfg.UserID = tomlCfg.UserID
	}
	if shouldApplyTOML(sources, md, "role_id") && tomlCfg.RoleID != "" {
		backendCfg.RoleID = tomlCfg.RoleID
	}
	if shouldApplyTOML(sources, md, "secret_id") && tomlCfg.SecretID != "" {
		backendCfg.SecretID = tomlCfg.SecretID
	}
	if shouldApplyTOML(sources, md, "file") && len(tomlCfg.File) > 0 {
		backendCfg.YAMLFile = tomlCfg.File
	}
	if shouldApplyTOML(sources, md, "filter") && tomlCfg.Filter != "" {
		backendCfg.Filter = tomlCfg.Filter
	}
	if shouldApplyTOML(sources, md, "path") && tomlCfg.Path != "" {
		backendCfg.Path = tomlCfg.Path
	}
	if shouldApplyTOML(sources, md, "acm_export_private_key") {
		backendCfg.ACMExportPrivateKey = tomlCfg.ACMExportPrivateKey
	}
	if shouldApplyTOML(sources, md, "secretsmanager_version_stage") && tomlCfg.SecretsManagerVersionStage != "" {
		backendCfg.SecretsManagerVersionStage = tomlCfg.SecretsManagerVersionStage
	}
	if shouldApplyTOML(sources, md, "secretsmanager_no_flatten") {
		backendCfg.SecretsManagerNoFlatten = tomlCfg.SecretsManagerNoFlatten
	}
	if shouldApplyTOML(sources, md, "imds_cache_ttl") && tomlCfg.IMDSCacheTTL != "" {
		d, err := time.ParseDuration(tomlCfg.IMDSCacheTTL)
		if err != nil {
			return fmt.Errorf("invalid imds_cache_ttl %q: %w (use Go duration format, e.g., \"60s\", \"5m\")", tomlCfg.IMDSCacheTTL, err)
		}
		backendCfg.IMDSCacheTTL = d
	}

	// Operation timeout settings
	if shouldApplyTOML(sources, md, "backend_timeout") && tomlCfg.BackendTimeout != "" {
		d, err := time.ParseDuration(tomlCfg.BackendTimeout)
		if err != nil {
			return fmt.Errorf("invalid backend_timeout %q: %w (use Go duration format, e.g., \"30s\", \"1m\")", tomlCfg.BackendTimeout, err)
		}
		cli.BackendTimeout = d
	}
	if shouldApplyTOML(sources, md, "check_cmd_timeout") && tomlCfg.CheckCmdTimeout != "" {
		d, err := time.ParseDuration(tomlCfg.CheckCmdTimeout)
		if err != nil {
			return fmt.Errorf("invalid check_cmd_timeout %q: %w (use Go duration format, e.g., \"30s\", \"1m\")", tomlCfg.CheckCmdTimeout, err)
		}
		cli.CheckCmdTimeout = d
	}
	if shouldApplyTOML(sources, md, "reload_cmd_timeout") && tomlCfg.ReloadCmdTimeout != "" {
		d, err := time.ParseDuration(tomlCfg.ReloadCmdTimeout)
		if err != nil {
			return fmt.Errorf("invalid reload_cmd_timeout %q: %w (use Go duration format, e.g., \"60s\", \"2m\")", tomlCfg.ReloadCmdTimeout, err)
		}
		cli.ReloadCmdTimeout = d
	}
	if shouldApplyTOML(sources, md, "shutdown_timeout") && tomlCfg.ShutdownTimeout != "" {
		d, err := time.ParseDuration(tomlCfg.ShutdownTimeout)
		if err != nil {
			return fmt.Errorf("invalid shutdown_timeout %q: %w (use Go duration format, e.g., \"30s\", \"1m\")", tomlCfg.ShutdownTimeout, err)
		}
		cli.ShutdownTimeout = d
	}

	// Connection timeout settings (apply to CLI if default, then to backend config)
	if shouldApplyTOML(sources, md, "dial_timeout") && tomlCfg.DialTimeout != "" {
		d, err := time.ParseDuration(tomlCfg.DialTimeout)
		if err != nil {
			return fmt.Errorf("invalid dial_timeout %q: %w (use Go duration format, e.g., \"5s\", \"30s\")", tomlCfg.DialTimeout, err)
		}
		cli.DialTimeout = d
	}
	if shouldApplyTOML(sources, md, "read_timeout") && tomlCfg.ReadTimeout != "" {
		d, err := time.ParseDuration(tomlCfg.ReadTimeout)
		if err != nil {
			return fmt.Errorf("invalid read_timeout %q: %w (use Go duration format, e.g., \"1s\", \"5s\")", tomlCfg.ReadTimeout, err)
		}
		cli.ReadTimeout = d
	}
	if shouldApplyTOML(sources, md, "write_timeout") && tomlCfg.WriteTimeout != "" {
		d, err := time.ParseDuration(tomlCfg.WriteTimeout)
		if err != nil {
			return fmt.Errorf("invalid write_timeout %q: %w (use Go duration format, e.g., \"1s\", \"5s\")", tomlCfg.WriteTimeout, err)
		}
		cli.WriteTimeout = d
	}

	// Retry configuration (apply to CLI if default, then to backend config)
	if shouldApplyTOML(sources, md, "retry_max_attempts") && tomlCfg.RetryMaxAttempts != 0 {
		cli.RetryMaxAttempts = tomlCfg.RetryMaxAttempts
	}
	if shouldApplyTOML(sources, md, "retry_base_delay") && tomlCfg.RetryBaseDelay != "" {
		d, err := time.ParseDuration(tomlCfg.RetryBaseDelay)
		if err != nil {
			return fmt.Errorf("invalid retry_base_delay %q: %w (use Go duration format, e.g., \"100ms\", \"1s\")", tomlCfg.RetryBaseDelay, err)
		}
		cli.RetryBaseDelay = d
	}
	if shouldApplyTOML(sources, md, "retry_max_delay") && tomlCfg.RetryMaxDelay != "" {
		d, err := time.ParseDuration(tomlCfg.RetryMaxDelay)
		if err != nil {
			return fmt.Errorf("invalid retry_max_delay %q: %w (use Go duration format, e.g., \"5s\", \"30s\")", tomlCfg.RetryMaxDelay, err)
		}
		cli.RetryMaxDelay = d
	}

	// Watch mode timeouts
	if shouldApplyTOML(sources, md, "watch_error_backoff") && tomlCfg.WatchErrorBackoff != "" {
		d, err := time.ParseDuration(tomlCfg.WatchErrorBackoff)
		if err != nil {
			return fmt.Errorf("invalid watch_error_backoff %q: %w (use Go duration format, e.g., \"2s\", \"5s\")", tomlCfg.WatchErrorBackoff, err)
		}
		cli.WatchErrorBackoff = d
	}
	if shouldApplyTOML(sources, md, "debounce") && tomlCfg.Debounce != "" {
		if _, err := time.ParseDuration(tomlCfg.Debounce); err != nil {
			return fmt.Errorf("invalid debounce %q: %w (use Go duration format, e.g., \"500ms\", \"2s\")", tomlCfg.Debounce, err)
		}
		cli.DebounceStr = tomlCfg.Debounce
	}
	if shouldApplyTOML(sources, md, "batch_interval") && tomlCfg.BatchInterval != "" {
		if _, err := time.ParseDuration(tomlCfg.BatchInterval); err != nil {
			return fmt.Errorf("invalid batch_interval %q: %w (use Go duration format, e.g., \"5s\", \"30s\")", tomlCfg.BatchInterval, err)
		}
		cli.BatchIntervalStr = tomlCfg.BatchInterval
	}

	// Systemd integration
	if shouldApplyTOML(sources, md, "systemd_notify") {
		cli.SystemdNotify = tomlCfg.SystemdNotify
	}
	if shouldApplyTOML(sources, md, "watchdog_interval") && tomlCfg.WatchdogInterval != "" {
		d, err := time.ParseDuration(tomlCfg.WatchdogInterval)
		if err != nil {
			return fmt.Errorf("invalid watchdog_interval %q: %w (use Go duration format, e.g., \"30s\", \"1m\")", tomlCfg.WatchdogInterval, err)
		}
		cli.WatchdogInterval = d
	}

	// Preflight timeout
	if shouldApplyTOML(sources, md, "preflight_timeout") && tomlCfg.PreflightTimeout != "" {
		d, err := time.ParseDuration(tomlCfg.PreflightTimeout)
		if err != nil {
			return fmt.Errorf("invalid preflight_timeout %q: %w (use Go duration format, e.g., \"10s\", \"30s\")", tomlCfg.PreflightTimeout, err)
		}
		cli.PreflightTimeout = d
	}

	return nil
}

func getBackendNodesFromSRV(record string) ([]string, error) {
	nodes := make([]string, 0)

	// Ignore the CNAME as we don't need it.
	_, addrs, err := net.LookupSRV("", "", record)
	if err != nil {
		return nodes, fmt.Errorf("look up SRV record %s: %w", record, err)
	}
	for _, srv := range addrs {
		host := strings.TrimRight(srv.Target, ".")
		port := strconv.FormatUint(uint64(srv.Port), 10)
		nodes = append(nodes, net.JoinHostPort(host, port))
	}
	return nodes, nil
}
