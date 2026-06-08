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
	LogLevel      string   `toml:"log-level"`
	LogFormat     string   `toml:"log-format"`
	Watch         bool     `toml:"watch"`
	FailureMode   string   `toml:"failure_mode"`
	KeepStageFile bool     `toml:"keep_stage_file"`
	SRVDomain     string   `toml:"srv_domain"`
	SRVRecord     string   `toml:"srv_record"`
	Nodes         []string `toml:"nodes"`

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
	Database       string   `toml:"database"`
	File           []string `toml:"file"`
	Filter         string   `toml:"filter"`
	Path           string   `toml:"path"`

	ACMExportPrivateKey        bool   `toml:"acm_export_private_key"`
	SecretsManagerVersionStage string `toml:"secretsmanager_version_stage"`
	SecretsManagerNoFlatten    bool   `toml:"secretsmanager_no_flatten"`

	// Performance settings
	TemplateCache *bool  `toml:"template_cache"` // Pointer to distinguish unset from false
	StatCacheTTL  string `toml:"stat_cache_ttl"`

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

	// Preflight timeout
	PreflightTimeout string `toml:"preflight_timeout"`

	// Metrics and observability
	MetricsAddr string `toml:"metrics_addr"`
}

type configSources struct {
	cli map[string]bool
	env map[string]bool
}

func newConfigSources(ctx *kong.Context) *configSources {
	sources := &configSources{
		cli: make(map[string]bool),
		env: make(map[string]bool),
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
		log.Debug("Skipping confd config file.")
		return nil
	}

	log.Debug("Loading %s", cli.ConfigFile)
	configBytes, err := os.ReadFile(cli.ConfigFile)
	if err != nil {
		return err
	}

	var tomlCfg TOMLConfig
	md, err := toml.Decode(string(configBytes), &tomlCfg)
	if err != nil {
		return err
	}

	sources := cli.configSources

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
	if shouldApplyTOML(sources, md, "log_level", "log-level") && tomlCfg.LogLevel != "" {
		cli.LogLevel = tomlCfg.LogLevel
	}
	if shouldApplyTOML(sources, md, "log_format", "log-format") && tomlCfg.LogFormat != "" {
		cli.LogFormat = tomlCfg.LogFormat
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
	if shouldApplyTOML(sources, md, "database") && tomlCfg.Database != "" {
		backendCfg.Database = tomlCfg.Database
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
		return nodes, err
	}
	for _, srv := range addrs {
		host := strings.TrimRight(srv.Target, ".")
		port := strconv.FormatUint(uint64(srv.Port), 10)
		nodes = append(nodes, net.JoinHostPort(host, port))
	}
	return nodes, nil
}

func processEnv(cfg *backends.Config) {
	cakeys := os.Getenv("CONFD_CLIENT_CAKEYS")
	if len(cakeys) > 0 && cfg.ClientCaKeys == "" {
		cfg.ClientCaKeys = cakeys
	}

	cert := os.Getenv("CONFD_CLIENT_CERT")
	if len(cert) > 0 && cfg.ClientCert == "" {
		cfg.ClientCert = cert
	}

	key := os.Getenv("CONFD_CLIENT_KEY")
	if len(key) > 0 && cfg.ClientKey == "" {
		cfg.ClientKey = key
	}

	if os.Getenv("ACM_EXPORT_PRIVATE_KEY") != "" && !cfg.ACMExportPrivateKey {
		cfg.ACMExportPrivateKey = true
	}
}
