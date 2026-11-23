package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/metrics"
	"github.com/abtreece/confd/pkg/service"
	"github.com/abtreece/confd/pkg/template"
	"github.com/alecthomas/kong"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Default timeout and retry values
const (
	DefaultDialTimeout       = 5 * time.Second
	DefaultReadTimeout       = 1 * time.Second
	DefaultWriteTimeout      = 1 * time.Second
	DefaultRetryMaxAttempts  = 3
	DefaultRetryBaseDelay    = 100 * time.Millisecond
	DefaultRetryMaxDelay     = 5 * time.Second
	DefaultWatchErrorBackoff = 2 * time.Second
	DefaultPreflightTimeout  = 10 * time.Second
	DefaultStatCacheTTL      = 1 * time.Second
)

// CLI is the root command structure
type CLI struct {
	configSources *configSources

	// Global flags
	ConfDir       string `name:"confdir" help:"confd conf directory" default:"/etc/confd" env:"CONFD_CONFDIR"`
	ConfigFile    string `name:"config-file" help:"confd config file" default:"/etc/confd/confd.toml" env:"CONFD_CONFIG_FILE"`
	Interval      int    `help:"backend polling interval" default:"600" env:"CONFD_INTERVAL"`
	LogLevel      string `name:"log-level" help:"log level (debug, info, warn, error)" default:"" env:"CONFD_LOG_LEVEL"`
	LogFormat     string `name:"log-format" help:"log format (text, json)" default:"" env:"CONFD_LOG_FORMAT"`
	Noop          bool   `help:"only show pending changes" env:"CONFD_NOOP"`
	Onetime       bool   `help:"run once and exit" env:"CONFD_ONETIME"`
	Prefix        string `help:"key path prefix" env:"CONFD_PREFIX"`
	SyncOnly      bool   `name:"sync-only" help:"sync without check_cmd and reload_cmd" env:"CONFD_SYNC_ONLY"`
	Watch         bool   `help:"enable watch support" env:"CONFD_WATCH"`
	FailureMode   string `name:"failure-mode" help:"error handling mode: 'best-effort' or 'fail-fast'" default:"best-effort" enum:"best-effort,fail-fast" env:"CONFD_FAILURE_MODE"`
	KeepStageFile bool   `name:"keep-stage-file" help:"keep staged files" env:"CONFD_KEEP_STAGE_FILE"`
	SRVDomain     string `name:"srv-domain" help:"DNS SRV domain" env:"CONFD_SRV_DOMAIN"`
	SRVRecord     string `name:"srv-record" help:"SRV record for backend node discovery" env:"CONFD_SRV_RECORD"`

	// Validation flags
	CheckConfig bool   `name:"check-config" help:"validate configuration files and exit"`
	Preflight   bool   `help:"run connectivity checks and exit"`
	Validate    bool   `help:"validate templates without processing (syntax check)"`
	MockData    string `name:"mock-data" help:"JSON file with mock data for template validation"`
	Resource    string `help:"specific resource file to validate (used with --check-config or --validate)"`

	// Diff flags (for use with --noop)
	Diff        bool `help:"show diff output in noop mode"`
	DiffContext int  `name:"diff-context" help:"lines of context for diff" default:"3"`
	Color       bool `help:"colorize diff output"`

	// Watch mode flags
	DebounceStr      string `name:"debounce" help:"debounce duration for watch mode (e.g., 2s, 500ms)" env:"CONFD_DEBOUNCE"`
	BatchIntervalStr string `name:"batch-interval" help:"batch processing interval for watch mode (e.g., 5s)" env:"CONFD_BATCH_INTERVAL"`

	// Performance flags
	TemplateCache    bool          `name:"template-cache" help:"enable template compilation caching" default:"true" negatable:"" env:"CONFD_TEMPLATE_CACHE"`
	StatCacheTTL     time.Duration `name:"stat-cache-ttl" help:"TTL for template file stat cache (e.g., 1s, 500ms)" default:"1s" env:"CONFD_STAT_CACHE_TTL"`
	BackendTimeout   time.Duration `name:"backend-timeout" help:"timeout for backend operations (e.g., 30s, 1m)" default:"30s" env:"CONFD_BACKEND_TIMEOUT"`
	CheckCmdTimeout  time.Duration `name:"check-cmd-timeout" help:"default timeout for check commands (e.g., 30s)" default:"30s" env:"CONFD_CHECK_CMD_TIMEOUT"`
	ReloadCmdTimeout time.Duration `name:"reload-cmd-timeout" help:"default timeout for reload commands (e.g., 60s)" default:"60s" env:"CONFD_RELOAD_CMD_TIMEOUT"`

	// Connection timeouts
	// Which backends use each of these is recorded in backendFlagScope
	// (config.go); keep the help text in step with it.
	DialTimeout  time.Duration `name:"dial-timeout" help:"connection timeout (not used by consul, env, file or vault)" default:"5s" env:"CONFD_DIAL_TIMEOUT"`
	ReadTimeout  time.Duration `name:"read-timeout" help:"redis only: read timeout" default:"1s" env:"CONFD_READ_TIMEOUT"`
	WriteTimeout time.Duration `name:"write-timeout" help:"redis only: write timeout" default:"1s" env:"CONFD_WRITE_TIMEOUT"`

	// Retry configuration (redis only, see backendFlagScope)
	RetryMaxAttempts int           `name:"retry-max-attempts" help:"redis only: max connection retry attempts" default:"3" env:"CONFD_RETRY_MAX_ATTEMPTS"`
	RetryBaseDelay   time.Duration `name:"retry-base-delay" help:"redis only: initial retry backoff delay" default:"100ms" env:"CONFD_RETRY_BASE_DELAY"`
	RetryMaxDelay    time.Duration `name:"retry-max-delay" help:"redis only: maximum retry backoff delay" default:"5s" env:"CONFD_RETRY_MAX_DELAY"`

	// Watch mode timeouts
	WatchErrorBackoff time.Duration `name:"watch-error-backoff" help:"backoff after watch errors" default:"2s" env:"CONFD_WATCH_ERROR_BACKOFF"`

	// Preflight timeout
	PreflightTimeout time.Duration `name:"preflight-timeout" help:"preflight check timeout" default:"10s" env:"CONFD_PREFLIGHT_TIMEOUT"`

	// Shutdown timeout
	ShutdownTimeout time.Duration `name:"shutdown-timeout" help:"graceful shutdown timeout" default:"30s" env:"CONFD_SHUTDOWN_TIMEOUT"`

	// Systemd integration
	SystemdNotify    bool          `name:"systemd-notify" help:"enable systemd sd_notify support" env:"CONFD_SYSTEMD_NOTIFY"`
	WatchdogInterval time.Duration `name:"watchdog-interval" help:"systemd watchdog ping interval (0=disabled)" default:"0" env:"CONFD_WATCHDOG_INTERVAL"`

	// Metrics and observability
	MetricsAddr string `name:"metrics-addr" help:"Address for metrics endpoint (e.g., :9100). Disabled if empty." env:"CONFD_METRICS_ADDR"`

	Version VersionFlag `help:"print version and exit"`

	// Backend subcommands
	Consul         ConsulCmd         `cmd:"" name:"consul" help:"Use Consul backend"`
	Etcd           EtcdCmd           `cmd:"" name:"etcd" help:"Use etcd backend"`
	Vault          VaultCmd          `cmd:"" name:"vault" help:"Use Vault backend"`
	Redis          RedisCmd          `cmd:"" name:"redis" help:"Use Redis backend"`
	Zookeeper      ZookeeperCmd      `cmd:"" name:"zookeeper" help:"Use Zookeeper backend"`
	DynamoDB       DynamoDBCmd       `cmd:"" name:"dynamodb" help:"Use DynamoDB backend"`
	SSM            SSMCmd            `cmd:"" name:"ssm" help:"Use AWS SSM Parameter Store backend"`
	ACM            ACMCmd            `cmd:"" name:"acm" help:"Use AWS ACM backend"`
	SecretsManager SecretsManagerCmd `cmd:"" name:"secretsmanager" help:"Use AWS Secrets Manager backend"`
	IMDS           IMDSCmd           `cmd:"" name:"imds" help:"Use AWS EC2 IMDS backend"`
	Env            EnvCmd            `cmd:"" name:"env" help:"Use environment variables backend"`
	File           FileCmd           `cmd:"" name:"file" help:"Use file backend"`
}

// AfterApply captures which options came from CLI arguments or environment
// variables before the config file is merged in run().
func (c *CLI) AfterApply(ctx *kong.Context) error {
	c.configSources = newConfigSources(ctx)
	return nil
}

// parserOptions returns the Kong options the confd command line is parsed
// with. main and the tests both use them, so that they parse the same way.
func parserOptions() []kong.Option {
	return []kong.Option{
		kong.Name("confd"),
		kong.Description("Manage local config files using templates and data from backends"),
		kong.UsageOnError(),
		kong.PostBuild(ignoreEmptyStringEnvs),
	}
}

// ignoreEmptyStringEnvs makes an environment variable that is set to the
// empty string behave exactly as if it were unset, for every flag whose type
// is string. Kong would otherwise apply the empty value over the flag's
// default and validate it against the flag's enum, and newConfigSources would
// record the setting as explicit, which stops its confd.toml value from
// applying.
//
// The variable is detached from the flag before parsing, so everything that
// follows (defaults, enum validation, source tracking) sees a flag with no
// variable set. The process environment is not changed. Flags of other types
// are left alone: an empty value there is a parse error.
func ignoreEmptyStringEnvs(k *kong.Kong) error {
	var visit func(node *kong.Node)
	visit = func(node *kong.Node) {
		for _, flag := range node.Flags {
			if !flag.Target.IsValid() || flag.Target.Kind() != reflect.String {
				continue
			}
			envs := slices.DeleteFunc(slices.Clone(flag.Tag.Envs), func(env string) bool {
				value, isSet := os.LookupEnv(env)
				return isSet && value == ""
			})
			if len(envs) == len(flag.Tag.Envs) {
				continue
			}
			if len(envs) == 0 {
				// Keep the variable's name in the help text, which Kong
				// builds from the list being emptied here.
				flag.Help = kong.DefaultHelpValueFormatter(flag.Value)
			}
			flag.Tag.Envs = envs
			flag.Envs = envs
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(k.Model.Node)
	return nil
}

// VersionFlag is a custom flag type that prints version and exits
type VersionFlag bool

func (v VersionFlag) BeforeApply(app *kong.Kong) error {
	fmt.Printf("confd %s (Git SHA: %s, Go Version: %s)\n", Version, GitSHA, runtime.Version())
	os.Exit(0)
	return nil
}

// Shared flag groups

type TLSFlags struct {
	ClientCert   string `name:"client-cert" help:"client certificate file" env:"CONFD_CLIENT_CERT"`
	ClientKey    string `name:"client-key" help:"client key file" env:"CONFD_CLIENT_KEY"`
	ClientCaKeys string `name:"client-ca-keys" help:"client CA keys" env:"CONFD_CLIENT_CAKEYS"`
}

type AuthFlags struct {
	BasicAuth bool   `name:"basic-auth" help:"use basic authentication"`
	Username  string `help:"authentication username"`
	Password  string `help:"authentication password"`
	AuthToken string `name:"auth-token" help:"bearer token for authentication"`
}

type NodeFlags struct {
	Node []string `help:"backend node addresses" short:"n" sep:"none"`
}

// Backend commands

// backendCommand is a backend subcommand. backendConfig builds the backend
// config from the command's parsed flags; Run and the tests both call it, so
// that the tests check the mapping production uses.
type backendCommand interface {
	backendConfig(cli *CLI) (backends.Config, error)
}

// runBackend is the Run body shared by every backend command.
func runBackend(cli *CLI, cmd backendCommand) error {
	cfg, err := cmd.backendConfig(cli)
	if err != nil {
		return err
	}
	return run(cli, cfg)
}

type ConsulCmd struct {
	NodeFlags
	TLSFlags
	AuthFlags
	Scheme string `help:"URI scheme (http or https)" default:"http"`
}

func (c *ConsulCmd) Run(cli *CLI) error {
	return runBackend(cli, c)
}

func (c *ConsulCmd) backendConfig(_ *CLI) (backends.Config, error) {
	return backends.Config{
		Backend:      "consul",
		BackendNodes: c.Node,
		Scheme:       c.Scheme,
		ClientCert:   c.ClientCert,
		ClientKey:    c.ClientKey,
		ClientCaKeys: c.ClientCaKeys,
		BasicAuth:    c.BasicAuth,
		Username:     c.Username,
		Password:     c.Password,
		AuthToken:    c.AuthToken,
	}, nil
}

type EtcdCmd struct {
	NodeFlags
	TLSFlags
	AuthFlags
	Scheme         string `help:"URI scheme (http or https)" default:"http"`
	ClientInsecure bool   `name:"client-insecure" help:"skip TLS certificate verification"`
}

func (e *EtcdCmd) Run(cli *CLI) error {
	return runBackend(cli, e)
}

func (e *EtcdCmd) backendConfig(_ *CLI) (backends.Config, error) {
	return backends.Config{
		Backend:        "etcd",
		BackendNodes:   e.Node,
		Scheme:         e.Scheme,
		ClientCert:     e.ClientCert,
		ClientKey:      e.ClientKey,
		ClientCaKeys:   e.ClientCaKeys,
		ClientInsecure: e.ClientInsecure,
		BasicAuth:      e.BasicAuth,
		Username:       e.Username,
		Password:       e.Password,
	}, nil
}

type VaultCmd struct {
	NodeFlags
	TLSFlags
	AuthType  string `name:"auth-type" help:"auth backend type (token, app-id, app-role, kubernetes)" default:""`
	AuthToken string `name:"auth-token" help:"Vault auth token" env:"VAULT_TOKEN"`
	AppID     string `name:"app-id" help:"app-id for app-id auth"`
	UserID    string `name:"user-id" help:"user-id for app-id auth"`
	RoleID    string `name:"role-id" help:"role-id for app-role/kubernetes auth"`
	SecretID  string `name:"secret-id" help:"secret-id for app-role auth"`
	Path      string `help:"auth mount path"`
	Username  string `help:"username for userpass auth"`
	Password  string `help:"password for userpass auth"`
}

func (v *VaultCmd) Run(cli *CLI) error {
	return runBackend(cli, v)
}

func (v *VaultCmd) backendConfig(_ *CLI) (backends.Config, error) {
	return backends.Config{
		Backend:      "vault",
		BackendNodes: v.Node,
		ClientCert:   v.ClientCert,
		ClientKey:    v.ClientKey,
		ClientCaKeys: v.ClientCaKeys,
		AuthType:     v.AuthType,
		AuthToken:    v.AuthToken,
		AppID:        v.AppID,
		UserID:       v.UserID,
		RoleID:       v.RoleID,
		SecretID:     v.SecretID,
		Path:         v.Path,
		Username:     v.Username,
		Password:     v.Password,
	}, nil
}

// redisPasswordEnv is the environment variable the redis command reads its
// password from.
const redisPasswordEnv = "REDIS_PASSWORD"

// RedisCmd declares the AuthFlags fields itself rather than embedding
// AuthFlags, so that REDIS_PASSWORD is bound to the redis password flag only
// and no other backend reads it.
type RedisCmd struct {
	NodeFlags
	TLSFlags
	BasicAuth bool   `name:"basic-auth" help:"use basic authentication"`
	Username  string `help:"authentication username"`
	Password  string `help:"Redis password (legacy: --client-key is used when no password is set)" env:"REDIS_PASSWORD"`
	AuthToken string `name:"auth-token" help:"bearer token for authentication"`
	Separator string `help:"separator to replace '/' in keys"`
}

func (r *RedisCmd) Run(cli *CLI) error {
	return runBackend(cli, r)
}

// resolvePassword treats an empty --password as unset, so that the first
// non-empty source wins: flag, then REDIS_PASSWORD, then password in
// confd.toml. An empty REDIS_PASSWORD is already unset by the time the command
// line is parsed (see ignoreEmptyStringEnvs).
//
// An empty --password stops the parser from reading REDIS_PASSWORD, so the
// variable is read here. When that gives no password either, the flag is
// still recorded as an explicit source, which would stop the confd.toml
// password from applying, so that record is dropped.
func (r *RedisCmd) resolvePassword(cli *CLI) {
	if r.Password == "" {
		r.Password = os.Getenv(redisPasswordEnv)
	}
	if r.Password == "" && cli.configSources != nil {
		delete(cli.configSources.cli, "password")
	}
}

// backendConfig resolves the password (see resolvePassword) and builds the
// redis backend config from the parsed command.
func (r *RedisCmd) backendConfig(cli *CLI) (backends.Config, error) {
	r.resolvePassword(cli)
	return backends.Config{
		Backend:      "redis",
		BackendNodes: r.Node,
		ClientCert:   r.ClientCert,
		ClientKey:    r.ClientKey,
		ClientCaKeys: r.ClientCaKeys,
		Username:     r.Username,
		Password:     r.Password,
		Separator:    r.Separator,
	}, nil
}

type ZookeeperCmd struct {
	NodeFlags
}

func (z *ZookeeperCmd) Run(cli *CLI) error {
	return runBackend(cli, z)
}

func (z *ZookeeperCmd) backendConfig(_ *CLI) (backends.Config, error) {
	return backends.Config{
		Backend:      "zookeeper",
		BackendNodes: z.Node,
	}, nil
}

type DynamoDBCmd struct {
	Table string `help:"DynamoDB table name" required:""`
}

func (d *DynamoDBCmd) Run(cli *CLI) error {
	return runBackend(cli, d)
}

func (d *DynamoDBCmd) backendConfig(_ *CLI) (backends.Config, error) {
	return backends.Config{
		Backend: "dynamodb",
		Table:   d.Table,
	}, nil
}

type SSMCmd struct{}

func (s *SSMCmd) Run(cli *CLI) error {
	return runBackend(cli, s)
}

func (s *SSMCmd) backendConfig(_ *CLI) (backends.Config, error) {
	return backends.Config{Backend: "ssm"}, nil
}

type ACMCmd struct {
	ExportPrivateKey bool `name:"acm-export-private-key" help:"export private key from certificates" env:"ACM_EXPORT_PRIVATE_KEY"`
}

func (a *ACMCmd) Run(cli *CLI) error {
	return runBackend(cli, a)
}

func (a *ACMCmd) backendConfig(_ *CLI) (backends.Config, error) {
	return backends.Config{
		Backend:             "acm",
		ACMExportPrivateKey: a.ExportPrivateKey,
	}, nil
}

type SecretsManagerCmd struct {
	VersionStage string `name:"secretsmanager-version-stage" help:"version stage (AWSCURRENT, AWSPREVIOUS, or custom)" default:"AWSCURRENT"`
	NoFlatten    bool   `name:"secretsmanager-no-flatten" help:"disable JSON flattening"`
}

func (s *SecretsManagerCmd) Run(cli *CLI) error {
	return runBackend(cli, s)
}

func (s *SecretsManagerCmd) backendConfig(_ *CLI) (backends.Config, error) {
	return backends.Config{
		Backend:                    "secretsmanager",
		SecretsManagerVersionStage: s.VersionStage,
		SecretsManagerNoFlatten:    s.NoFlatten,
	}, nil
}

type EnvCmd struct{}

func (e *EnvCmd) Run(cli *CLI) error {
	return runBackend(cli, e)
}

func (e *EnvCmd) backendConfig(_ *CLI) (backends.Config, error) {
	return backends.Config{Backend: "env"}, nil
}

type FileCmd struct {
	File   []string `help:"YAML/JSON files to watch" required:"" sep:"none"`
	Filter string   `help:"file filter pattern" default:"*"`
}

func (f *FileCmd) Run(cli *CLI) error {
	return runBackend(cli, f)
}

func (f *FileCmd) backendConfig(_ *CLI) (backends.Config, error) {
	return backends.Config{
		Backend:  "file",
		Filter:   f.Filter,
		YAMLFile: f.File,
	}, nil
}

type IMDSCmd struct {
	CacheTTL string `name:"imds-cache-ttl" help:"Cache TTL for metadata (e.g., 60s, 5m)" default:"60s"`
}

func (i *IMDSCmd) Run(cli *CLI) error {
	return runBackend(cli, i)
}

func (i *IMDSCmd) backendConfig(_ *CLI) (backends.Config, error) {
	cacheTTL, err := time.ParseDuration(i.CacheTTL)
	if err != nil {
		return backends.Config{}, fmt.Errorf("invalid imds-cache-ttl: %w", err)
	}
	return backends.Config{
		Backend:      "imds",
		IMDSCacheTTL: cacheTTL,
	}, nil
}

// run is the shared execution function for all backends
func run(cli *CLI, backendCfg backends.Config) error {
	// Load TOML config file if it exists (for defaults)
	if err := loadConfigFile(cli, &backendCfg); err != nil {
		return err
	}
	applyBackendDefaults(&backendCfg)

	// Apply CLI timeout/retry config to backend config
	backendCfg.DialTimeout = cli.DialTimeout
	backendCfg.ReadTimeout = cli.ReadTimeout
	backendCfg.WriteTimeout = cli.WriteTimeout
	backendCfg.RetryMaxAttempts = cli.RetryMaxAttempts
	backendCfg.RetryBaseDelay = cli.RetryBaseDelay
	backendCfg.RetryMaxDelay = cli.RetryMaxDelay

	// Set up logging
	if cli.LogLevel != "" {
		log.SetLevel(cli.LogLevel)
	}
	if cli.LogFormat != "" {
		log.SetFormat(cli.LogFormat)
	}

	// Warn about timeout and retry flags the backend does not use. This runs
	// before the backend client is created so that it is logged even when the
	// backend then fails to connect.
	warnIgnoredBackendFlags(cli.configSources, backendCfg.Backend)

	// Check-config mode: validate configuration and exit (no backend needed)
	if cli.CheckConfig {
		if err := template.ValidateConfig(cli.ConfDir, cli.Resource); err != nil {
			return fmt.Errorf("validate config: %w", err)
		}
		return nil
	}

	// Validate mode: validate templates and exit (no backend needed)
	if cli.Validate {
		if err := template.ValidateTemplates(cli.ConfDir, cli.Resource, cli.MockData); err != nil {
			return fmt.Errorf("validate templates: %w", err)
		}
		return nil
	}

	// Handle SRV record discovery
	if cli.SRVDomain != "" && cli.SRVRecord == "" {
		cli.SRVRecord = fmt.Sprintf("_%s._tcp.%s.", backendCfg.Backend, cli.SRVDomain)
	}
	if backendCfg.Backend != "env" && cli.SRVRecord != "" {
		log.Info("SRV record set to %s", cli.SRVRecord)
		srvNodes, err := getBackendNodesFromSRV(cli.SRVRecord)
		if err != nil {
			return fmt.Errorf("cannot get nodes from SRV records: %w", err)
		}
		if backendCfg.Backend == "etcd" {
			for i, v := range srvNodes {
				srvNodes[i] = backendCfg.Scheme + "://" + v
			}
		}
		backendCfg.BackendNodes = srvNodes
	}

	log.Info("Starting confd")
	log.Info("Backend set to %s", backendCfg.Backend)

	// Initialize template cache
	template.InitTemplateCache(cli.TemplateCache, cli.StatCacheTTL)

	// Create root context for cancellation. Until endStartup is called, a
	// shutdown signal cancels it and backend construction is abandoned.
	ctx, cancel, endStartup := newRootContext()
	defer cancel()
	defer endStartup()

	// Create store client
	storeClient, err := awaitStoreClient(ctx, func() (backends.StoreClient, error) {
		return backends.New(ctx, backendCfg)
	})
	endStartup()
	if ctx.Err() != nil {
		if err == nil {
			closeAbandonedStoreClient(storeClient)
		}
		return startupInterruptedError(ctx)
	}
	if err != nil {
		return fmt.Errorf("create store client: %w", err)
	}

	// Start metrics server if configured
	var metricsServer *http.Server
	if cli.MetricsAddr != "" {
		metrics.Initialize()
		storeClient = metrics.WrapStoreClient(storeClient, backendCfg.Backend)
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))
		mux.HandleFunc("/health", metrics.HealthHandler(storeClient))
		mux.HandleFunc("/ready", metrics.ReadyHandler(storeClient))
		mux.HandleFunc("/ready/detailed", metrics.ReadyDetailedHandler(storeClient))
		metricsServer = &http.Server{
			Addr:              cli.MetricsAddr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			log.Info("Starting metrics server on %s", cli.MetricsAddr)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("Metrics server error: %v", err)
			}
		}()
	}

	// Parse failure mode
	failureMode, err := template.ParseFailureMode(cli.FailureMode)
	if err != nil {
		return fmt.Errorf("parse failure mode: %w", err)
	}

	// Build template config
	tmplCfg := template.Config{
		ConfDir:           cli.ConfDir,
		ConfigDir:         filepath.Join(cli.ConfDir, "conf.d"),
		TemplateDir:       filepath.Join(cli.ConfDir, "templates"),
		StoreClient:       storeClient,
		Backend:           backendCfg.Backend,
		Noop:              cli.Noop,
		Prefix:            cli.Prefix,
		SyncOnly:          cli.SyncOnly,
		KeepStageFile:     cli.KeepStageFile,
		ShowDiff:          cli.Diff,
		DiffContext:       cli.DiffContext,
		ColorDiff:         cli.Color,
		Ctx:               ctx,
		BackendTimeout:    cli.BackendTimeout,
		CheckCmdTimeout:   cli.CheckCmdTimeout,
		ReloadCmdTimeout:  cli.ReloadCmdTimeout,
		WatchErrorBackoff: cli.WatchErrorBackoff,
		PreflightTimeout:  cli.PreflightTimeout,
		FailureMode:       failureMode,
	}

	// Parse watch mode duration flags
	if cli.DebounceStr != "" {
		d, err := time.ParseDuration(cli.DebounceStr)
		if err != nil {
			return fmt.Errorf("invalid debounce duration %q: %w", cli.DebounceStr, err)
		}
		tmplCfg.Debounce = d
	}
	if cli.BatchIntervalStr != "" {
		d, err := time.ParseDuration(cli.BatchIntervalStr)
		if err != nil {
			return fmt.Errorf("invalid batch-interval duration %q: %w", cli.BatchIntervalStr, err)
		}
		tmplCfg.BatchInterval = d
	}

	// Watch mode needs every backend in use to support watching. This is
	// checked whenever watch is enabled, including with --preflight and
	// --onetime, so a configuration that cannot run in watch mode is never
	// reported as working.
	if cli.Watch {
		if err := template.CheckWatchSupport(tmplCfg); err != nil {
			return fmt.Errorf("check watch support: %w", err)
		}
	}

	// Preflight mode: run connectivity checks and exit
	if cli.Preflight {
		if err := template.Preflight(tmplCfg); err != nil {
			return fmt.Errorf("preflight checks: %w", err)
		}
		return nil
	}

	// One-time mode
	if cli.Onetime {
		if err := template.Process(tmplCfg); err != nil {
			return fmt.Errorf("process templates: %w", err)
		}
		return nil
	}

	// Continuous mode with processor
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	// Create reload manager for SIGHUP handling
	reloadMgr := service.NewReloadManager()
	reloadChan := reloadMgr.Subscribe()

	var processor template.Processor
	if cli.Watch {
		if tmplCfg.BatchInterval > 0 {
			// Use batch processor when --batch-interval is specified
			log.Info("Batch processing enabled with interval %v", tmplCfg.BatchInterval)
			processor = template.BatchWatchProcessor(tmplCfg, stopChan, doneChan, errChan, reloadChan)
		} else {
			processor = template.WatchProcessor(tmplCfg, stopChan, doneChan, errChan, reloadChan)
		}
	} else {
		processor = template.IntervalProcessor(tmplCfg, stopChan, doneChan, errChan, cli.Interval, reloadChan)
	}

	go processor.Process()

	// Create shutdown manager for graceful shutdown coordination
	shutdownMgr := service.NewShutdownManager(cli.ShutdownTimeout, metricsServer, storeClient)

	// Create systemd notifier and start watchdog if enabled
	systemdNotifier := service.NewSystemdNotifier(cli.SystemdNotify, cli.WatchdogInterval)
	systemdNotifier.StartWatchdog(ctx)

	// Notify systemd that we're ready
	if err := systemdNotifier.NotifyReady(); err != nil {
		log.Warning("Failed to notify systemd ready: %v", err)
	}

	shutdown := func() error {
		if err := shutdownMgr.Shutdown(context.Background()); err != nil {
			log.Error("Shutdown error: %v", err)
			return fmt.Errorf("shutdown: %w", err)
		}
		if err := template.CloseAllCachedClients(); err != nil {
			log.Warning("Error closing per-resource backend clients: %v", err)
		}
		return nil
	}

	procErrs := &processorErrors{errChan: errChan, doneChan: doneChan}
	// finish shuts down once the processor has stopped. A processor that
	// stopped because no template resource loaded makes confd exit non-zero.
	finish := func() error {
		procErrs.waitDone()
		if err := shutdown(); err != nil {
			return err
		}
		return procErrs.fatal
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	var stopOnce sync.Once
	for {
		select {
		case err := <-errChan:
			procErrs.handle(err)
		case s := <-signalChan:
			switch s {
			case syscall.SIGHUP:
				log.Info("Captured SIGHUP. Reloading configuration...")
				// Notify systemd we're reloading
				if err := systemdNotifier.NotifyReloading(); err != nil {
					log.Warning("Failed to notify systemd reloading: %v", err)
				}
				reloadMgr.TriggerReload()
				// Notify systemd we're ready again
				if err := systemdNotifier.NotifyReady(); err != nil {
					log.Warning("Failed to notify systemd ready: %v", err)
				}
			case syscall.SIGINT, syscall.SIGTERM:
				log.Info("Captured %v. Initiating shutdown...", s)
				// Notify systemd we're stopping
				if err := systemdNotifier.NotifyStopping(); err != nil {
					log.Warning("Failed to notify systemd stopping: %v", err)
				}
				cancel() // Cancel context to signal all goroutines
				stopOnce.Do(func() { close(stopChan) })
				return finish()
			}
		case <-doneChan:
			return finish()
		}
	}
}

// processorErrors receives the errors a long-running processor reports.
// Errors are logged and the processor carries on, except for
// template.ErrNoTemplateResources: the processor has stopped, and the error
// is kept in fatal for run to return once shutdown has completed.
type processorErrors struct {
	errChan  <-chan error
	doneChan <-chan bool
	fatal    error
}

func (p *processorErrors) handle(err error) {
	if errors.Is(err, template.ErrNoTemplateResources) {
		// Not logged here: run returns it and it is reported on exit.
		p.fatal = err
		return
	}
	log.Error("%s", err.Error())
}

// waitDone handles errors until the processor has stopped, then the ones it
// reported just before stopping. A processor sends its last error before it
// closes doneChan, so that error may still be unread when doneChan closes.
func (p *processorErrors) waitDone() {
	for {
		select {
		case err := <-p.errChan:
			p.handle(err)
		case <-p.doneChan:
			for {
				select {
				case err := <-p.errChan:
					p.handle(err)
				default:
					return
				}
			}
		}
	}
}

// newRootContext returns the root context, its cancel function, and
// endStartup. Until endStartup is called, SIGINT or SIGTERM cancels the
// context with a cause naming the signal, so startup work that honours the
// context can be interrupted. endStartup stops that handling and returns only
// once any signal already delivered has been applied to the context; after it
// returns, signals are left to whatever handling the caller installs next.
func newRootContext() (ctx context.Context, cancel context.CancelFunc, endStartup func()) {
	ctx, cancelCause := context.WithCancelCause(context.Background())
	interrupt := func(s os.Signal) {
		cancelCause(fmt.Errorf("received signal %v: %w", s, context.Canceled))
	}

	startupSignals := make(chan os.Signal, 1)
	signal.Notify(startupSignals, syscall.SIGINT, syscall.SIGTERM)
	stopWatching := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case s := <-startupSignals:
			interrupt(s)
		case <-stopWatching:
		}
	}()

	var endOnce sync.Once
	endStartup = func() {
		endOnce.Do(func() {
			signal.Stop(startupSignals)
			close(stopWatching)
			<-watcherDone
			select {
			case s := <-startupSignals:
				interrupt(s)
			default:
			}
		})
	}
	return ctx, func() { cancelCause(nil) }, endStartup
}

// awaitStoreClient runs construct and returns its result, unless ctx is
// cancelled first, in which case it returns a startup interrupted error
// without waiting. Not every backend constructor honours a context, so the wait is
// abandoned here rather than relying on construct to return; a client that an
// abandoned construct produces later is closed.
func awaitStoreClient(ctx context.Context, construct func() (backends.StoreClient, error)) (backends.StoreClient, error) {
	type constructResult struct {
		client backends.StoreClient
		err    error
	}
	resultChan := make(chan constructResult, 1)
	go func() {
		client, err := construct()
		resultChan <- constructResult{client: client, err: err}
	}()

	select {
	case result := <-resultChan:
		return result.client, result.err
	case <-ctx.Done():
		go func() {
			if result := <-resultChan; result.err == nil {
				closeAbandonedStoreClient(result.client)
			}
		}()
		return nil, startupInterruptedError(ctx)
	}
}

// startupInterruptedError reports why ctx was cancelled during startup.
func startupInterruptedError(ctx context.Context) error {
	return fmt.Errorf("startup interrupted: %w", context.Cause(ctx))
}

// closeAbandonedStoreClient closes a store client that was constructed but
// will not be used because startup was interrupted.
func closeAbandonedStoreClient(client backends.StoreClient) {
	if client == nil {
		return
	}
	if err := client.Close(); err != nil {
		log.Warning("Error closing store client: %v", err)
	}
}

func applyBackendDefaults(cfg *backends.Config) {
	if len(cfg.BackendNodes) > 0 {
		return
	}
	switch cfg.Backend {
	case "consul":
		cfg.BackendNodes = []string{"127.0.0.1:8500"}
	case "etcd":
		cfg.BackendNodes = []string{"http://127.0.0.1:2379"}
	case "vault":
		cfg.BackendNodes = []string{"http://127.0.0.1:8200"}
	case "redis":
		cfg.BackendNodes = []string{"127.0.0.1:6379"}
	case "zookeeper":
		cfg.BackendNodes = []string{"127.0.0.1:2181"}
	}
}
