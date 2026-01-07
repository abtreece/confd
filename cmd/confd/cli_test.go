package main

import (
	"os"
	"path/filepath"
	"testing"

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
	parser, err := kong.New(&cli,
		kong.Name("confd"),
		kong.Exit(func(int) {}), // Don't exit on error
	)
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

func TestConsulCmdDefaultNodes(t *testing.T) {
	cli, _ := parseCLI(t, []string{"consul"})
	cmd := &cli.Consul

	if len(cmd.Node) != 0 {
		t.Errorf("Expected no nodes before Run, got %v", cmd.Node)
	}

	// Simulate what Run does for default nodes
	if len(cmd.Node) == 0 {
		cmd.Node = []string{"127.0.0.1:8500"}
	}

	if len(cmd.Node) != 1 || cmd.Node[0] != "127.0.0.1:8500" {
		t.Errorf("Expected default node '127.0.0.1:8500', got %v", cmd.Node)
	}
}

func TestEtcdCmdDefaultNodes(t *testing.T) {
	cli, _ := parseCLI(t, []string{"etcd"})
	cmd := &cli.Etcd

	if len(cmd.Node) == 0 {
		cmd.Node = []string{"http://127.0.0.1:2379"}
	}

	if len(cmd.Node) != 1 || cmd.Node[0] != "http://127.0.0.1:2379" {
		t.Errorf("Expected default node 'http://127.0.0.1:2379', got %v", cmd.Node)
	}
}

func TestVaultCmdDefaultNodes(t *testing.T) {
	cli, _ := parseCLI(t, []string{"vault"})
	cmd := &cli.Vault

	if len(cmd.Node) == 0 {
		cmd.Node = []string{"http://127.0.0.1:8200"}
	}

	if len(cmd.Node) != 1 || cmd.Node[0] != "http://127.0.0.1:8200" {
		t.Errorf("Expected default node 'http://127.0.0.1:8200', got %v", cmd.Node)
	}
}

func TestRedisCmdDefaultNodes(t *testing.T) {
	cli, _ := parseCLI(t, []string{"redis"})
	cmd := &cli.Redis

	if len(cmd.Node) == 0 {
		cmd.Node = []string{"127.0.0.1:6379"}
	}

	if len(cmd.Node) != 1 || cmd.Node[0] != "127.0.0.1:6379" {
		t.Errorf("Expected default node '127.0.0.1:6379', got %v", cmd.Node)
	}
}

func TestZookeeperCmdDefaultNodes(t *testing.T) {
	cli, _ := parseCLI(t, []string{"zookeeper"})
	cmd := &cli.Zookeeper

	if len(cmd.Node) == 0 {
		cmd.Node = []string{"127.0.0.1:2181"}
	}

	if len(cmd.Node) != 1 || cmd.Node[0] != "127.0.0.1:2181" {
		t.Errorf("Expected default node '127.0.0.1:2181', got %v", cmd.Node)
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

func TestDynamoDBWatchModeError(t *testing.T) {
	cli, _ := parseCLI(t, []string{"--watch", "dynamodb", "--table", "test"})
	cmd := &cli.DynamoDB
	err := cmd.Run(cli)
	if err == nil {
		t.Error("Expected error for watch mode with dynamodb")
	}
	if err.Error() != "watch mode not supported for dynamodb backend" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestSSMWatchModeError(t *testing.T) {
	cli, _ := parseCLI(t, []string{"--watch", "ssm"})
	cmd := &cli.SSM
	err := cmd.Run(cli)
	if err == nil {
		t.Error("Expected error for watch mode with ssm")
	}
	if err.Error() != "watch mode not supported for ssm backend" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestACMWatchModeError(t *testing.T) {
	cli, _ := parseCLI(t, []string{"--watch", "acm"})
	cmd := &cli.ACM
	err := cmd.Run(cli)
	if err == nil {
		t.Error("Expected error for watch mode with acm")
	}
	if err.Error() != "watch mode not supported for acm backend" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestSecretsManagerWatchModeError(t *testing.T) {
	cli, _ := parseCLI(t, []string{"--watch", "secretsmanager"})
	cmd := &cli.SecretsManager
	err := cmd.Run(cli)
	if err == nil {
		t.Error("Expected error for watch mode with secretsmanager")
	}
	if err.Error() != "watch mode not supported for secretsmanager backend" {
		t.Errorf("Unexpected error message: %v", err)
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
		ConfDir:    "/cli/confd",    // Non-default CLI value
		ConfigFile: configPath,
		Interval:   120,             // Non-default CLI value
		Prefix:     "/cliprefix",    // CLI value
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

func TestLoadConfigFileNotFound(t *testing.T) {
	cli := &CLI{
		ConfDir:    "/etc/confd",
		ConfigFile: "/nonexistent/confd.toml",
		Interval:   600,
	}
	backendCfg := &backends.Config{}

	// Should not error when config file doesn't exist
	if err := loadConfigFile(cli, backendCfg); err != nil {
		t.Errorf("loadConfigFile should not error for missing file: %v", err)
	}
}

func TestProcessEnv(t *testing.T) {
	// Save and restore environment
	origCert := os.Getenv("CONFD_CLIENT_CERT")
	origKey := os.Getenv("CONFD_CLIENT_KEY")
	origCaKeys := os.Getenv("CONFD_CLIENT_CAKEYS")
	origACM := os.Getenv("ACM_EXPORT_PRIVATE_KEY")
	defer func() {
		os.Setenv("CONFD_CLIENT_CERT", origCert)
		os.Setenv("CONFD_CLIENT_KEY", origKey)
		os.Setenv("CONFD_CLIENT_CAKEYS", origCaKeys)
		os.Setenv("ACM_EXPORT_PRIVATE_KEY", origACM)
	}()

	os.Setenv("CONFD_CLIENT_CERT", "/env/cert")
	os.Setenv("CONFD_CLIENT_KEY", "/env/key")
	os.Setenv("CONFD_CLIENT_CAKEYS", "/env/ca")
	os.Setenv("ACM_EXPORT_PRIVATE_KEY", "true")

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
	if !cfg.ACMExportPrivateKey {
		t.Error("Expected ACMExportPrivateKey to be true")
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
