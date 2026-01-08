package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/template"
	"github.com/alecthomas/kong"
)

// CLI is the root command structure
type CLI struct {
	// Global flags
	ConfDir       string `name:"confdir" help:"confd conf directory" default:"/etc/confd"`
	ConfigFile    string `name:"config-file" help:"confd config file" default:"/etc/confd/confd.toml"`
	Interval      int    `help:"backend polling interval" default:"600"`
	LogLevel      string `name:"log-level" help:"log level (debug, info, warn, error)" default:""`
	LogFormat     string `name:"log-format" help:"log format (text, json)" default:""`
	Noop          bool   `help:"only show pending changes"`
	Onetime       bool   `help:"run once and exit"`
	Prefix        string `help:"key path prefix"`
	SyncOnly      bool   `name:"sync-only" help:"sync without check_cmd and reload_cmd"`
	Watch         bool   `help:"enable watch support"`
	KeepStageFile bool   `name:"keep-stage-file" help:"keep staged files"`
	SRVDomain     string `name:"srv-domain" help:"DNS SRV domain"`
	SRVRecord     string `name:"srv-record" help:"SRV record for backend node discovery"`

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
	DebounceStr      string `name:"debounce" help:"debounce duration for watch mode (e.g., 2s, 500ms)"`
	BatchIntervalStr string `name:"batch-interval" help:"batch processing interval for watch mode (e.g., 5s)"`

	// Template cache flags
	TemplateCacheEnabled *bool  `name:"template-cache" help:"enable template caching (default: true)"`
	TemplateCacheSize    int    `name:"template-cache-size" help:"maximum cached templates" default:"100"`
	TemplateCachePolicy  string `name:"template-cache-policy" help:"eviction policy: lru, lfu, fifo" default:"lru"`

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
	Env            EnvCmd            `cmd:"" name:"env" help:"Use environment variables backend"`
	File           FileCmd           `cmd:"" name:"file" help:"Use file backend"`
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

type ConsulCmd struct {
	NodeFlags
	TLSFlags
	AuthFlags
	Scheme string `help:"URI scheme (http or https)" default:"http"`
}

func (c *ConsulCmd) Run(cli *CLI) error {
	if len(c.Node) == 0 {
		c.Node = []string{"127.0.0.1:8500"}
	}
	cfg := backends.Config{
		Backend:      "consul",
		BackendNodes: c.Node,
		Scheme:       c.Scheme,
		ClientCert:   c.ClientCert,
		ClientKey:    c.ClientKey,
		ClientCaKeys: c.ClientCaKeys,
		BasicAuth:    c.BasicAuth,
		Username:     c.Username,
		Password:     c.Password,
	}
	return run(cli, cfg)
}

type EtcdCmd struct {
	NodeFlags
	TLSFlags
	AuthFlags
	Scheme         string `help:"URI scheme (http or https)" default:"http"`
	ClientInsecure bool   `name:"client-insecure" help:"skip TLS certificate verification"`
}

func (e *EtcdCmd) Run(cli *CLI) error {
	if len(e.Node) == 0 {
		e.Node = []string{"http://127.0.0.1:2379"}
	}
	cfg := backends.Config{
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
	}
	return run(cli, cfg)
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
	if len(v.Node) == 0 {
		v.Node = []string{"http://127.0.0.1:8200"}
	}
	cfg := backends.Config{
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
	}
	return run(cli, cfg)
}

type RedisCmd struct {
	NodeFlags
	TLSFlags
	AuthFlags
	Separator string `help:"separator to replace '/' in keys"`
}

func (r *RedisCmd) Run(cli *CLI) error {
	if len(r.Node) == 0 {
		r.Node = []string{"127.0.0.1:6379"}
	}
	cfg := backends.Config{
		Backend:      "redis",
		BackendNodes: r.Node,
		ClientCert:   r.ClientCert,
		ClientKey:    r.ClientKey,
		ClientCaKeys: r.ClientCaKeys,
		Username:     r.Username,
		Password:     r.Password,
		Separator:    r.Separator,
	}
	return run(cli, cfg)
}

type ZookeeperCmd struct {
	NodeFlags
}

func (z *ZookeeperCmd) Run(cli *CLI) error {
	if len(z.Node) == 0 {
		z.Node = []string{"127.0.0.1:2181"}
	}
	cfg := backends.Config{
		Backend:      "zookeeper",
		BackendNodes: z.Node,
	}
	return run(cli, cfg)
}

type DynamoDBCmd struct {
	Table string `help:"DynamoDB table name" required:""`
}

func (d *DynamoDBCmd) Run(cli *CLI) error {
	if cli.Watch {
		return fmt.Errorf("watch mode not supported for dynamodb backend")
	}
	cfg := backends.Config{
		Backend: "dynamodb",
		Table:   d.Table,
	}
	return run(cli, cfg)
}

type SSMCmd struct{}

func (s *SSMCmd) Run(cli *CLI) error {
	if cli.Watch {
		return fmt.Errorf("watch mode not supported for ssm backend")
	}
	cfg := backends.Config{
		Backend: "ssm",
	}
	return run(cli, cfg)
}

type ACMCmd struct {
	ExportPrivateKey bool `name:"acm-export-private-key" help:"export private key from certificates" env:"ACM_EXPORT_PRIVATE_KEY"`
}

func (a *ACMCmd) Run(cli *CLI) error {
	if cli.Watch {
		return fmt.Errorf("watch mode not supported for acm backend")
	}
	cfg := backends.Config{
		Backend:             "acm",
		ACMExportPrivateKey: a.ExportPrivateKey,
	}
	return run(cli, cfg)
}

type SecretsManagerCmd struct {
	VersionStage string `name:"secretsmanager-version-stage" help:"version stage (AWSCURRENT, AWSPREVIOUS, or custom)" default:"AWSCURRENT"`
	NoFlatten    bool   `name:"secretsmanager-no-flatten" help:"disable JSON flattening"`
}

func (s *SecretsManagerCmd) Run(cli *CLI) error {
	if cli.Watch {
		return fmt.Errorf("watch mode not supported for secretsmanager backend")
	}
	cfg := backends.Config{
		Backend:                    "secretsmanager",
		SecretsManagerVersionStage: s.VersionStage,
		SecretsManagerNoFlatten:    s.NoFlatten,
	}
	return run(cli, cfg)
}

type EnvCmd struct{}

func (e *EnvCmd) Run(cli *CLI) error {
	cfg := backends.Config{
		Backend: "env",
	}
	return run(cli, cfg)
}

type FileCmd struct {
	File   []string `help:"YAML/JSON files to watch" required:"" sep:"none"`
	Filter string   `help:"file filter pattern" default:"*"`
}

func (f *FileCmd) Run(cli *CLI) error {
	cfg := backends.Config{
		Backend: "file",
		Filter:  f.Filter,
	}
	// YAMLFile is a util.Nodes type ([]string)
	cfg.YAMLFile = f.File
	return run(cli, cfg)
}

// run is the shared execution function for all backends
func run(cli *CLI, backendCfg backends.Config) error {
	// Load TOML config file if it exists (for defaults)
	if err := loadConfigFile(cli, &backendCfg); err != nil {
		return err
	}

	// Process environment variables
	processEnv(&backendCfg)

	// Set up logging
	if cli.LogLevel != "" {
		log.SetLevel(cli.LogLevel)
	}
	if cli.LogFormat != "" {
		log.SetFormat(cli.LogFormat)
	}

	// Check-config mode: validate configuration and exit (no backend needed)
	if cli.CheckConfig {
		return template.ValidateConfig(cli.ConfDir, cli.Resource)
	}

	// Validate mode: validate templates and exit (no backend needed)
	if cli.Validate {
		return template.ValidateTemplates(cli.ConfDir, cli.Resource, cli.MockData)
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

	// Create store client
	storeClient, err := backends.New(backendCfg)
	if err != nil {
		return err
	}

	// Initialize template cache
	templateCacheEnabled := true
	if cli.TemplateCacheEnabled != nil {
		templateCacheEnabled = *cli.TemplateCacheEnabled
	}
	template.InitGlobalTemplateCache(templateCacheEnabled, cli.TemplateCacheSize, cli.TemplateCachePolicy)

	// Build template config
	tmplCfg := template.Config{
		ConfDir:              cli.ConfDir,
		ConfigDir:            filepath.Join(cli.ConfDir, "conf.d"),
		TemplateDir:          filepath.Join(cli.ConfDir, "templates"),
		StoreClient:          storeClient,
		Noop:                 cli.Noop,
		Prefix:               cli.Prefix,
		SyncOnly:             cli.SyncOnly,
		KeepStageFile:        cli.KeepStageFile,
		ShowDiff:             cli.Diff,
		DiffContext:          cli.DiffContext,
		ColorDiff:            cli.Color,
		TemplateCacheEnabled: templateCacheEnabled,
		TemplateCacheSize:    cli.TemplateCacheSize,
		TemplateCachePolicy:  cli.TemplateCachePolicy,
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

	// Preflight mode: run connectivity checks and exit
	if cli.Preflight {
		return template.Preflight(tmplCfg)
	}

	// One-time mode
	if cli.Onetime {
		if err := template.Process(tmplCfg); err != nil {
			return err
		}
		return nil
	}

	// Continuous mode with processor
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	var processor template.Processor
	if cli.Watch {
		if tmplCfg.BatchInterval > 0 {
			// Use batch processor when --batch-interval is specified
			log.Info("Batch processing enabled with interval %v", tmplCfg.BatchInterval)
			processor = template.BatchWatchProcessor(tmplCfg, stopChan, doneChan, errChan)
		} else {
			processor = template.WatchProcessor(tmplCfg, stopChan, doneChan, errChan)
		}
	} else {
		processor = template.IntervalProcessor(tmplCfg, stopChan, doneChan, errChan, cli.Interval)
	}

	go processor.Process()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case err := <-errChan:
			log.Error("%s", err.Error())
		case s := <-signalChan:
			log.Info("Captured %v. Exiting...", s)
			close(doneChan)
		case <-doneChan:
			return nil
		}
	}
}
