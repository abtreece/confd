package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
	"github.com/alecthomas/kong"
)

func init() {
	log.SetLevel("error")
}

// parseCLI is a test helper that parses CLI args and returns the parsed CLI and context
func parseCLI(t *testing.T, args []string) (*CLI, *kong.Context) {
	t.Helper()
	var cli CLI
	// The exit option stops a parse error from exiting the test binary.
	parser, err := kong.New(&cli, append(parserOptions(), kong.Exit(func(int) {}))...)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		t.Fatalf("Failed to parse args %v: %v", args, err)
	}
	return &cli, ctx
}

func TestCLIParseSubcommands(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedBackend string
	}{
		{"consul", []string{"consul"}, "consul"},
		{"etcd", []string{"etcd"}, "etcd"},
		{"vault", []string{"vault"}, "vault"},
		{"redis", []string{"redis"}, "redis"},
		{"zookeeper", []string{"zookeeper"}, "zookeeper"},
		{"dynamodb", []string{"dynamodb", "--table", "test"}, "dynamodb"},
		{"ssm", []string{"ssm"}, "ssm"},
		{"acm", []string{"acm"}, "acm"},
		{"secretsmanager", []string{"secretsmanager"}, "secretsmanager"},
		{"env", []string{"env"}, "env"},
		{"file", []string{"file", "--file", "test.yaml"}, "file"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ctx := parseCLI(t, tc.args)
			if ctx.Command() != tc.expectedBackend {
				t.Errorf("Expected command %q, got %q", tc.expectedBackend, ctx.Command())
			}
		})
	}
}

func TestCLIGlobalFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		validate func(*CLI) error
	}{
		{
			name: "default values",
			args: []string{"env"},
			validate: func(cli *CLI) error {
				if cli.ConfDir != "/etc/confd" {
					t.Errorf("Expected confdir '/etc/confd', got %q", cli.ConfDir)
				}
				if cli.ConfigFile != "/etc/confd/confd.toml" {
					t.Errorf("Expected config-file '/etc/confd/confd.toml', got %q", cli.ConfigFile)
				}
				if cli.Interval != 600 {
					t.Errorf("Expected interval 600, got %d", cli.Interval)
				}
				return nil
			},
		},
		{
			name: "custom confdir",
			args: []string{"--confdir=/custom/path", "env"},
			validate: func(cli *CLI) error {
				if cli.ConfDir != "/custom/path" {
					t.Errorf("Expected confdir '/custom/path', got %q", cli.ConfDir)
				}
				return nil
			},
		},
		{
			name: "custom interval",
			args: []string{"--interval=120", "env"},
			validate: func(cli *CLI) error {
				if cli.Interval != 120 {
					t.Errorf("Expected interval 120, got %d", cli.Interval)
				}
				return nil
			},
		},
		{
			name: "onetime flag",
			args: []string{"--onetime", "env"},
			validate: func(cli *CLI) error {
				if !cli.Onetime {
					t.Error("Expected onetime to be true")
				}
				return nil
			},
		},
		{
			name: "noop flag",
			args: []string{"--noop", "env"},
			validate: func(cli *CLI) error {
				if !cli.Noop {
					t.Error("Expected noop to be true")
				}
				return nil
			},
		},
		{
			name: "watch flag",
			args: []string{"--watch", "env"},
			validate: func(cli *CLI) error {
				if !cli.Watch {
					t.Error("Expected watch to be true")
				}
				return nil
			},
		},
		{
			name: "prefix flag",
			args: []string{"--prefix=/myapp", "env"},
			validate: func(cli *CLI) error {
				if cli.Prefix != "/myapp" {
					t.Errorf("Expected prefix '/myapp', got %q", cli.Prefix)
				}
				return nil
			},
		},
		{
			name: "sync-only flag",
			args: []string{"--sync-only", "env"},
			validate: func(cli *CLI) error {
				if !cli.SyncOnly {
					t.Error("Expected sync-only to be true")
				}
				return nil
			},
		},
		{
			name: "log-level flag",
			args: []string{"--log-level=debug", "env"},
			validate: func(cli *CLI) error {
				if cli.LogLevel != "debug" {
					t.Errorf("Expected log-level 'debug', got %q", cli.LogLevel)
				}
				return nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cli, _ := parseCLI(t, tc.args)
			tc.validate(cli)
		})
	}
}

func TestNodeFlagsMultipleNodes(t *testing.T) {
	cli, _ := parseCLI(t, []string{"etcd", "-n", "http://node1:2379", "-n", "http://node2:2379"})
	cmd := &cli.Etcd

	expected := []string{"http://node1:2379", "http://node2:2379"}
	if len(cmd.Node) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(cmd.Node))
	}
	for i, node := range expected {
		if cmd.Node[i] != node {
			t.Errorf("Expected node[%d] = %q, got %q", i, node, cmd.Node[i])
		}
	}
}

func TestTLSFlags(t *testing.T) {
	cli, _ := parseCLI(t, []string{
		"etcd",
		"--client-cert=/path/to/cert",
		"--client-key=/path/to/key",
		"--client-ca-keys=/path/to/ca",
	})
	cmd := &cli.Etcd

	if cmd.ClientCert != "/path/to/cert" {
		t.Errorf("Expected client-cert '/path/to/cert', got %q", cmd.ClientCert)
	}
	if cmd.ClientKey != "/path/to/key" {
		t.Errorf("Expected client-key '/path/to/key', got %q", cmd.ClientKey)
	}
	if cmd.ClientCaKeys != "/path/to/ca" {
		t.Errorf("Expected client-ca-keys '/path/to/ca', got %q", cmd.ClientCaKeys)
	}
}

func TestAuthFlags(t *testing.T) {
	cli, _ := parseCLI(t, []string{
		"consul",
		"--basic-auth",
		"--username=admin",
		"--password=secret",
		"--auth-token=mytoken",
	})
	cmd := &cli.Consul

	if !cmd.BasicAuth {
		t.Error("Expected basic-auth to be true")
	}
	if cmd.Username != "admin" {
		t.Errorf("Expected username 'admin', got %q", cmd.Username)
	}
	if cmd.Password != "secret" {
		t.Errorf("Expected password 'secret', got %q", cmd.Password)
	}
	if cmd.AuthToken != "mytoken" {
		t.Errorf("Expected auth-token 'mytoken', got %q", cmd.AuthToken)
	}
}

func TestVaultAuthFlags(t *testing.T) {
	cli, _ := parseCLI(t, []string{
		"vault",
		"--auth-type=app-role",
		"--role-id=my-role",
		"--secret-id=my-secret",
		"--path=/auth/approle",
	})
	cmd := &cli.Vault

	if cmd.AuthType != "app-role" {
		t.Errorf("Expected auth-type 'app-role', got %q", cmd.AuthType)
	}
	if cmd.RoleID != "my-role" {
		t.Errorf("Expected role-id 'my-role', got %q", cmd.RoleID)
	}
	if cmd.SecretID != "my-secret" {
		t.Errorf("Expected secret-id 'my-secret', got %q", cmd.SecretID)
	}
	if cmd.Path != "/auth/approle" {
		t.Errorf("Expected path '/auth/approle', got %q", cmd.Path)
	}
}

func TestVaultAuthTokenFlag(t *testing.T) {
	cli, _ := parseCLI(t, []string{
		"vault",
		"--auth-type=token",
		"--auth-token=my-vault-token",
	})
	cmd := &cli.Vault

	if cmd.AuthType != "token" {
		t.Errorf("Expected auth-type 'token', got %q", cmd.AuthType)
	}
	if cmd.AuthToken != "my-vault-token" {
		t.Errorf("Expected auth-token 'my-vault-token', got %q", cmd.AuthToken)
	}
}

func TestDynamoDBRequiredTable(t *testing.T) {
	var cli CLI
	parser, _ := kong.New(&cli, kong.Exit(func(int) {}))
	_, err := parser.Parse([]string{"dynamodb"})
	if err == nil {
		t.Error("Expected error for missing required --table flag")
	}
}

func TestFileRequiredFile(t *testing.T) {
	var cli CLI
	parser, _ := kong.New(&cli, kong.Exit(func(int) {}))
	_, err := parser.Parse([]string{"file"})
	if err == nil {
		t.Error("Expected error for missing required --file flag")
	}
}

func TestSecretsManagerFlags(t *testing.T) {
	cli, _ := parseCLI(t, []string{
		"secretsmanager",
		"--secretsmanager-version-stage=AWSPREVIOUS",
		"--secretsmanager-no-flatten",
	})
	cmd := &cli.SecretsManager

	if cmd.VersionStage != "AWSPREVIOUS" {
		t.Errorf("Expected version-stage 'AWSPREVIOUS', got %q", cmd.VersionStage)
	}
	if !cmd.NoFlatten {
		t.Error("Expected no-flatten to be true")
	}
}

func TestACMExportPrivateKeyFlag(t *testing.T) {
	cli, _ := parseCLI(t, []string{"acm", "--acm-export-private-key"})
	cmd := &cli.ACM

	if !cmd.ExportPrivateKey {
		t.Error("Expected acm-export-private-key to be true")
	}
}

func TestFileMultipleFiles(t *testing.T) {
	cli, _ := parseCLI(t, []string{"file", "--file", "a.yaml", "--file", "b.yaml"})
	cmd := &cli.File

	expected := []string{"a.yaml", "b.yaml"}
	if len(cmd.File) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(cmd.File))
	}
	for i, f := range expected {
		if cmd.File[i] != f {
			t.Errorf("Expected file[%d] = %q, got %q", i, f, cmd.File[i])
		}
	}
}

func TestFileFilterFlag(t *testing.T) {
	cli, _ := parseCLI(t, []string{"file", "--file", "dir/", "--filter", "*.yaml"})
	cmd := &cli.File

	if cmd.Filter != "*.yaml" {
		t.Errorf("Expected filter '*.yaml', got %q", cmd.Filter)
	}
}

func TestRedisSeperator(t *testing.T) {
	cli, _ := parseCLI(t, []string{"redis", "--separator", ":"})
	cmd := &cli.Redis

	if cmd.Separator != ":" {
		t.Errorf("Expected separator ':', got %q", cmd.Separator)
	}
}

func TestEtcdSchemeAndInsecure(t *testing.T) {
	cli, _ := parseCLI(t, []string{"etcd", "--scheme=https", "--client-insecure"})
	cmd := &cli.Etcd

	if cmd.Scheme != "https" {
		t.Errorf("Expected scheme 'https', got %q", cmd.Scheme)
	}
	if !cmd.ClientInsecure {
		t.Error("Expected client-insecure to be true")
	}
}

// clearCONFDEnvVars clears all CONFD_* and backend-specific env vars that could
// interfere with Kong parsing, saving original values for restoration via t.Cleanup.
func clearCONFDEnvVars(t *testing.T) {
	t.Helper()

	// List of env vars that Kong parses and could interfere with tests
	envVars := []string{
		"ACM_EXPORT_PRIVATE_KEY", "VAULT_TOKEN",
		"CONFD_CONFDIR", "CONFD_CONFIG_FILE", "CONFD_INTERVAL",
		"CONFD_LOG_LEVEL", "CONFD_LOG_FORMAT", "CONFD_PREFIX",
		"CONFD_WATCH", "CONFD_ONETIME", "CONFD_NOOP", "CONFD_SYNC_ONLY",
		"CONFD_FAILURE_MODE", "CONFD_KEEP_STAGE_FILE",
		"CONFD_BACKEND_TIMEOUT", "CONFD_CHECK_CMD_TIMEOUT", "CONFD_RELOAD_CMD_TIMEOUT",
		"CONFD_DIAL_TIMEOUT", "CONFD_READ_TIMEOUT", "CONFD_WRITE_TIMEOUT",
		"CONFD_PREFLIGHT_TIMEOUT", "CONFD_SHUTDOWN_TIMEOUT",
		"CONFD_RETRY_MAX_ATTEMPTS", "CONFD_RETRY_BASE_DELAY", "CONFD_RETRY_MAX_DELAY",
		"CONFD_DEBOUNCE", "CONFD_BATCH_INTERVAL", "CONFD_WATCH_ERROR_BACKOFF",
		"CONFD_TEMPLATE_CACHE", "CONFD_STAT_CACHE_TTL",
		"CONFD_METRICS_ADDR", "CONFD_SYSTEMD_NOTIFY",
		"CONFD_CLIENT_CERT", "CONFD_CLIENT_KEY", "CONFD_CLIENT_CAKEYS",
		"CONFD_SRV_DOMAIN", "CONFD_SRV_RECORD", "CONFD_WATCHDOG_INTERVAL",
	}

	// Save original values and clear
	origValues := make(map[string]string)
	for _, env := range envVars {
		if val, exists := os.LookupEnv(env); exists {
			origValues[env] = val
		}
		os.Unsetenv(env)
	}

	// Restore original values on cleanup
	t.Cleanup(func() {
		for _, env := range envVars {
			if val, exists := origValues[env]; exists {
				os.Setenv(env, val)
			} else {
				os.Unsetenv(env)
			}
		}
	})
}

func TestCLIEnvVars(t *testing.T) {
	// Clear all CONFD_* env vars to ensure hermetic tests
	clearCONFDEnvVars(t)

	tests := []struct {
		name     string
		envVar   string
		envValue string
		check    func(*CLI) bool
		desc     string
	}{
		{
			name:     "CONFD_CONFDIR",
			envVar:   "CONFD_CONFDIR",
			envValue: "/custom/confd",
			check:    func(cli *CLI) bool { return cli.ConfDir == "/custom/confd" },
			desc:     "ConfDir",
		},
		{
			name:     "CONFD_INTERVAL",
			envVar:   "CONFD_INTERVAL",
			envValue: "300",
			check:    func(cli *CLI) bool { return cli.Interval == 300 },
			desc:     "Interval",
		},
		{
			name:     "CONFD_LOG_LEVEL",
			envVar:   "CONFD_LOG_LEVEL",
			envValue: "debug",
			check:    func(cli *CLI) bool { return cli.LogLevel == "debug" },
			desc:     "LogLevel",
		},
		{
			name:     "CONFD_LOG_FORMAT",
			envVar:   "CONFD_LOG_FORMAT",
			envValue: "json",
			check:    func(cli *CLI) bool { return cli.LogFormat == "json" },
			desc:     "LogFormat",
		},
		{
			name:     "CONFD_PREFIX",
			envVar:   "CONFD_PREFIX",
			envValue: "/production",
			check:    func(cli *CLI) bool { return cli.Prefix == "/production" },
			desc:     "Prefix",
		},
		{
			name:     "CONFD_WATCH",
			envVar:   "CONFD_WATCH",
			envValue: "true",
			check:    func(cli *CLI) bool { return cli.Watch == true },
			desc:     "Watch",
		},
		{
			name:     "CONFD_ONETIME",
			envVar:   "CONFD_ONETIME",
			envValue: "true",
			check:    func(cli *CLI) bool { return cli.Onetime == true },
			desc:     "Onetime",
		},
		{
			name:     "CONFD_NOOP",
			envVar:   "CONFD_NOOP",
			envValue: "true",
			check:    func(cli *CLI) bool { return cli.Noop == true },
			desc:     "Noop",
		},
		{
			name:     "CONFD_SYNC_ONLY",
			envVar:   "CONFD_SYNC_ONLY",
			envValue: "true",
			check:    func(cli *CLI) bool { return cli.SyncOnly == true },
			desc:     "SyncOnly",
		},
		{
			name:     "CONFD_BACKEND_TIMEOUT",
			envVar:   "CONFD_BACKEND_TIMEOUT",
			envValue: "45s",
			check:    func(cli *CLI) bool { return cli.BackendTimeout == 45*time.Second },
			desc:     "BackendTimeout",
		},
		{
			name:     "CONFD_METRICS_ADDR",
			envVar:   "CONFD_METRICS_ADDR",
			envValue: ":9100",
			check:    func(cli *CLI) bool { return cli.MetricsAddr == ":9100" },
			desc:     "MetricsAddr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use t.Setenv for automatic cleanup
			t.Setenv(tt.envVar, tt.envValue)

			cli := &CLI{}
			parser, err := kong.New(cli, kong.NoDefaultHelp())
			if err != nil {
				t.Fatalf("Failed to create kong parser: %v", err)
			}

			// Parse with minimal args (just the env subcommand to satisfy required cmd)
			_, err = parser.Parse([]string{"env"})
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			if !tt.check(cli) {
				t.Errorf("Environment variable %s=%s did not set %s correctly", tt.envVar, tt.envValue, tt.desc)
			}
		})
	}
}

func TestCLIEnvVarPrecedence(t *testing.T) {
	// Clear all CONFD_* env vars to ensure hermetic tests
	clearCONFDEnvVars(t)

	// Test that CLI flags take precedence over env vars
	t.Setenv("CONFD_INTERVAL", "300")

	cli := &CLI{}
	parser, err := kong.New(cli, kong.NoDefaultHelp())
	if err != nil {
		t.Fatalf("Failed to create kong parser: %v", err)
	}

	// CLI flag should override env var
	_, err = parser.Parse([]string{"--interval", "600", "env"})
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if cli.Interval != 600 {
		t.Errorf("CLI flag should override env var: expected 600, got %d", cli.Interval)
	}
}

func makeCheckConfigCLI(t *testing.T) *CLI {
	t.Helper()

	confDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(confDir, "conf.d"), 0755); err != nil {
		t.Fatalf("failed to create conf.d: %v", err)
	}

	return &CLI{
		ConfDir:          confDir,
		ConfigFile:       filepath.Join(confDir, "missing-confd.toml"),
		CheckConfig:      true,
		TemplateCache:    true,
		StatCacheTTL:     DefaultStatCacheTTL,
		DialTimeout:      DefaultDialTimeout,
		ReadTimeout:      DefaultReadTimeout,
		WriteTimeout:     DefaultWriteTimeout,
		RetryMaxAttempts: DefaultRetryMaxAttempts,
		RetryBaseDelay:   DefaultRetryBaseDelay,
		RetryMaxDelay:    DefaultRetryMaxDelay,
		FailureMode:      "best-effort",
	}
}

func makeValidateCLI(t *testing.T) *CLI {
	t.Helper()

	confDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(confDir, "conf.d"), 0755); err != nil {
		t.Fatalf("failed to create conf.d: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(confDir, "templates"), 0755); err != nil {
		t.Fatalf("failed to create templates: %v", err)
	}

	return &CLI{
		ConfDir:          confDir,
		ConfigFile:       filepath.Join(confDir, "missing-confd.toml"),
		Validate:         true,
		TemplateCache:    true,
		StatCacheTTL:     DefaultStatCacheTTL,
		DialTimeout:      DefaultDialTimeout,
		ReadTimeout:      DefaultReadTimeout,
		WriteTimeout:     DefaultWriteTimeout,
		RetryMaxAttempts: DefaultRetryMaxAttempts,
		RetryBaseDelay:   DefaultRetryBaseDelay,
		RetryMaxDelay:    DefaultRetryMaxDelay,
		FailureMode:      "best-effort",
	}
}

func TestBackendRunMethods_DefaultNodesWithCheckConfig(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T, *CLI) error
	}{
		{
			name: "consul",
			run: func(t *testing.T, cli *CLI) error {
				cmd := &ConsulCmd{}
				return cmd.Run(cli)
			},
		},
		{
			name: "etcd",
			run: func(t *testing.T, cli *CLI) error {
				cmd := &EtcdCmd{}
				return cmd.Run(cli)
			},
		},
		{
			name: "vault",
			run: func(t *testing.T, cli *CLI) error {
				cmd := &VaultCmd{}
				return cmd.Run(cli)
			},
		},
		{
			name: "redis",
			run: func(t *testing.T, cli *CLI) error {
				cmd := &RedisCmd{}
				return cmd.Run(cli)
			},
		},
		{
			name: "zookeeper",
			run: func(t *testing.T, cli *CLI) error {
				cmd := &ZookeeperCmd{}
				return cmd.Run(cli)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := makeCheckConfigCLI(t)
			if err := tt.run(t, cli); err != nil {
				t.Fatalf("Run() returned error: %v", err)
			}
		})
	}
}

func TestApplyBackendDefaults(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		nodes   []string
		want    []string
	}{
		{name: "consul default", backend: "consul", want: []string{"127.0.0.1:8500"}},
		{name: "etcd default", backend: "etcd", want: []string{"http://127.0.0.1:2379"}},
		{name: "vault default", backend: "vault", want: []string{"http://127.0.0.1:8200"}},
		{name: "redis default", backend: "redis", want: []string{"127.0.0.1:6379"}},
		{name: "zookeeper default", backend: "zookeeper", want: []string{"127.0.0.1:2181"}},

		{name: "consul keeps configured node", backend: "consul", nodes: []string{"consul.example:8500"}, want: []string{"consul.example:8500"}},
		{name: "etcd keeps configured nodes", backend: "etcd", nodes: []string{"http://a:2379", "http://b:2379"}, want: []string{"http://a:2379", "http://b:2379"}},
		{name: "vault keeps configured node", backend: "vault", nodes: []string{"https://vault.example:8200"}, want: []string{"https://vault.example:8200"}},
		{name: "redis keeps configured node", backend: "redis", nodes: []string{"redis.example:6379"}, want: []string{"redis.example:6379"}},
		{name: "zookeeper keeps configured node", backend: "zookeeper", nodes: []string{"zk.example:2181"}, want: []string{"zk.example:2181"}},

		{name: "acm has no default node", backend: "acm"},
		{name: "dynamodb has no default node", backend: "dynamodb"},
		{name: "env has no default node", backend: "env"},
		{name: "file has no default node", backend: "file"},
		{name: "imds has no default node", backend: "imds"},
		{name: "secretsmanager has no default node", backend: "secretsmanager"},
		{name: "ssm has no default node", backend: "ssm"},
		{name: "unknown backend has no default node", backend: "not-a-backend"},
		{name: "empty backend has no default node", backend: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &backends.Config{Backend: tt.backend, BackendNodes: tt.nodes}
			applyBackendDefaults(cfg)
			if got := []string(cfg.BackendNodes); !slices.Equal(got, tt.want) {
				t.Fatalf("BackendNodes = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdditionalRunMethods_WithValidateMode(t *testing.T) {
	cli := makeValidateCLI(t)

	if err := (&EnvCmd{}).Run(cli); err != nil {
		t.Fatalf("env Run() returned error: %v", err)
	}

	fileCmd := &FileCmd{
		File:   []string{"test.yaml"},
		Filter: "*.yaml",
	}
	if err := fileCmd.Run(cli); err != nil {
		t.Fatalf("file Run() returned error: %v", err)
	}

	secretsCmd := &SecretsManagerCmd{
		VersionStage: "AWSCURRENT",
		NoFlatten:    true,
	}
	if err := secretsCmd.Run(cli); err != nil {
		t.Fatalf("secretsmanager Run() returned error: %v", err)
	}

	acmCmd := &ACMCmd{ExportPrivateKey: true}
	if err := acmCmd.Run(cli); err != nil {
		t.Fatalf("acm Run() returned error: %v", err)
	}

	ssmCmd := &SSMCmd{}
	if err := ssmCmd.Run(cli); err != nil {
		t.Fatalf("ssm Run() returned error: %v", err)
	}

	dynamoCmd := &DynamoDBCmd{Table: "test-table"}
	if err := dynamoCmd.Run(cli); err != nil {
		t.Fatalf("dynamodb Run() returned error: %v", err)
	}
}

func TestIMDSCmdRun(t *testing.T) {
	t.Run("invalid cache ttl", func(t *testing.T) {
		cli := makeCheckConfigCLI(t)
		cmd := &IMDSCmd{CacheTTL: "bad-duration"}
		err := cmd.Run(cli)
		if err == nil {
			t.Fatal("expected error for invalid cache ttl, got nil")
		}
		if !strings.Contains(err.Error(), "invalid imds-cache-ttl") {
			t.Fatalf("expected invalid ttl error, got %v", err)
		}
	})

	t.Run("valid cache ttl with check config", func(t *testing.T) {
		cli := makeCheckConfigCLI(t)
		cmd := &IMDSCmd{CacheTTL: "90s"}
		if err := cmd.Run(cli); err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	})
}

func TestGetBackendNodesFromSRV_Error(t *testing.T) {
	record := fmt.Sprintf("_confd-test._tcp.%d.invalid.", time.Now().UnixNano())
	nodes, err := getBackendNodesFromSRV(record)
	if err == nil {
		t.Fatalf("expected error for invalid SRV record, got nil with nodes=%v", nodes)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected no nodes on lookup error, got %v", nodes)
	}
}
