package main

import (
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
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
	File           []string `toml:"file"`
	Filter         string   `toml:"filter"`
	Path           string   `toml:"path"`

	ACMExportPrivateKey          bool   `toml:"acm_export_private_key"`
	SecretsManagerVersionStage   string `toml:"secretsmanager_version_stage"`
	SecretsManagerNoFlatten      bool   `toml:"secretsmanager_no_flatten"`

	// Graceful shutdown settings
	ShutdownTimeout int    `toml:"shutdown_timeout"`
	ShutdownCleanup string `toml:"shutdown_cleanup"`
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
	if _, err = toml.Decode(string(configBytes), &tomlCfg); err != nil {
		return err
	}

	// Apply TOML settings as defaults (CLI flags take precedence)
	// Global settings
	if cli.ConfDir == "/etc/confd" && tomlCfg.ConfDir != "" {
		cli.ConfDir = tomlCfg.ConfDir
	}
	if cli.Interval == 600 && tomlCfg.Interval != 0 {
		cli.Interval = tomlCfg.Interval
	}
	if !cli.Noop && tomlCfg.Noop {
		cli.Noop = true
	}
	if cli.Prefix == "" && tomlCfg.Prefix != "" {
		cli.Prefix = tomlCfg.Prefix
	}
	if !cli.SyncOnly && tomlCfg.SyncOnly {
		cli.SyncOnly = true
	}
	if cli.LogLevel == "" && tomlCfg.LogLevel != "" {
		cli.LogLevel = tomlCfg.LogLevel
	}
	if cli.LogFormat == "" && tomlCfg.LogFormat != "" {
		cli.LogFormat = tomlCfg.LogFormat
	}
	if !cli.Watch && tomlCfg.Watch {
		cli.Watch = true
	}
	if !cli.KeepStageFile && tomlCfg.KeepStageFile {
		cli.KeepStageFile = true
	}
	if cli.SRVDomain == "" && tomlCfg.SRVDomain != "" {
		cli.SRVDomain = tomlCfg.SRVDomain
	}
	if cli.SRVRecord == "" && tomlCfg.SRVRecord != "" {
		cli.SRVRecord = tomlCfg.SRVRecord
	}
	if cli.ShutdownTimeout == 15 && tomlCfg.ShutdownTimeout != 0 {
		cli.ShutdownTimeout = tomlCfg.ShutdownTimeout
	}
	if cli.ShutdownCleanup == "" && tomlCfg.ShutdownCleanup != "" {
		cli.ShutdownCleanup = tomlCfg.ShutdownCleanup
	}

	// Backend settings (only apply if not already set via CLI)
	if len(backendCfg.BackendNodes) == 0 && len(tomlCfg.Nodes) > 0 {
		backendCfg.BackendNodes = tomlCfg.Nodes
	}
	if backendCfg.AuthToken == "" && tomlCfg.AuthToken != "" {
		backendCfg.AuthToken = tomlCfg.AuthToken
	}
	if backendCfg.AuthType == "" && tomlCfg.AuthType != "" {
		backendCfg.AuthType = tomlCfg.AuthType
	}
	if !backendCfg.BasicAuth && tomlCfg.BasicAuth {
		backendCfg.BasicAuth = true
	}
	if backendCfg.ClientCaKeys == "" && tomlCfg.ClientCaKeys != "" {
		backendCfg.ClientCaKeys = tomlCfg.ClientCaKeys
	}
	if backendCfg.ClientCert == "" && tomlCfg.ClientCert != "" {
		backendCfg.ClientCert = tomlCfg.ClientCert
	}
	if backendCfg.ClientKey == "" && tomlCfg.ClientKey != "" {
		backendCfg.ClientKey = tomlCfg.ClientKey
	}
	if !backendCfg.ClientInsecure && tomlCfg.ClientInsecure {
		backendCfg.ClientInsecure = true
	}
	if backendCfg.Password == "" && tomlCfg.Password != "" {
		backendCfg.Password = tomlCfg.Password
	}
	if backendCfg.Scheme == "" && tomlCfg.Scheme != "" {
		backendCfg.Scheme = tomlCfg.Scheme
	}
	if backendCfg.Table == "" && tomlCfg.Table != "" {
		backendCfg.Table = tomlCfg.Table
	}
	if backendCfg.Separator == "" && tomlCfg.Separator != "" {
		backendCfg.Separator = tomlCfg.Separator
	}
	if backendCfg.Username == "" && tomlCfg.Username != "" {
		backendCfg.Username = tomlCfg.Username
	}
	if backendCfg.AppID == "" && tomlCfg.AppID != "" {
		backendCfg.AppID = tomlCfg.AppID
	}
	if backendCfg.UserID == "" && tomlCfg.UserID != "" {
		backendCfg.UserID = tomlCfg.UserID
	}
	if backendCfg.RoleID == "" && tomlCfg.RoleID != "" {
		backendCfg.RoleID = tomlCfg.RoleID
	}
	if backendCfg.SecretID == "" && tomlCfg.SecretID != "" {
		backendCfg.SecretID = tomlCfg.SecretID
	}
	if len(backendCfg.YAMLFile) == 0 && len(tomlCfg.File) > 0 {
		backendCfg.YAMLFile = tomlCfg.File
	}
	if backendCfg.Filter == "" && tomlCfg.Filter != "" {
		backendCfg.Filter = tomlCfg.Filter
	}
	if backendCfg.Path == "" && tomlCfg.Path != "" {
		backendCfg.Path = tomlCfg.Path
	}
	if !backendCfg.ACMExportPrivateKey && tomlCfg.ACMExportPrivateKey {
		backendCfg.ACMExportPrivateKey = true
	}
	if backendCfg.SecretsManagerVersionStage == "" && tomlCfg.SecretsManagerVersionStage != "" {
		backendCfg.SecretsManagerVersionStage = tomlCfg.SecretsManagerVersionStage
	}
	if !backendCfg.SecretsManagerNoFlatten && tomlCfg.SecretsManagerNoFlatten {
		backendCfg.SecretsManagerNoFlatten = true
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
