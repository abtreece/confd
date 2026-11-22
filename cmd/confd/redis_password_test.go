package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unsetEnv removes an environment variable for the duration of the test.
func unsetEnv(t *testing.T, name string) {
	t.Helper()
	original, wasSet := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unset %s: %v", name, err)
	}
	t.Cleanup(func() {
		if wasSet {
			os.Setenv(name, original)
		}
	})
}

func TestRedisPasswordPrecedence(t *testing.T) {
	tests := []struct {
		name         string
		flag         string
		env          *string
		tomlPassword string
		want         string
	}{
		{name: "flag only", flag: "from-flag", want: "from-flag"},
		{name: "env only", env: new("from-env"), want: "from-env"},
		{name: "toml only", tomlPassword: "from-toml", want: "from-toml"},
		{name: "flag wins over env", flag: "from-flag", env: new("from-env"), want: "from-flag"},
		{name: "flag wins over toml", flag: "from-flag", tomlPassword: "from-toml", want: "from-flag"},
		{name: "env wins over toml", env: new("from-env"), tomlPassword: "from-toml", want: "from-env"},
		{name: "empty env falls through to toml", env: new(""), tomlPassword: "from-toml", want: "from-toml"},
		{name: "nothing set", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetEnv(t, redisPasswordEnv)
			if tt.env != nil {
				t.Setenv(redisPasswordEnv, *tt.env)
			}

			configPath := filepath.Join(t.TempDir(), "confd.toml")
			tomlContent := ""
			if tt.tomlPassword != "" {
				tomlContent = `password = "` + tt.tomlPassword + `"` + "\n"
			}
			if err := os.WriteFile(configPath, []byte(tomlContent), 0600); err != nil {
				t.Fatalf("write config file: %v", err)
			}

			args := []string{"--config-file", configPath, "redis"}
			if tt.flag != "" {
				args = append(args, "--password", tt.flag)
			}
			cli, _ := parseCLI(t, args)

			cfg := cli.Redis.backendConfig(cli)
			if err := loadConfigFile(cli, &cfg); err != nil {
				t.Fatalf("loadConfigFile: %v", err)
			}
			if cfg.Password != tt.want {
				t.Errorf("Password = %q, want %q", cfg.Password, tt.want)
			}
		})
	}
}

func TestRedisPasswordEnvIsRedisOnly(t *testing.T) {
	t.Setenv(redisPasswordEnv, "from-env")

	tests := []struct {
		backend  string
		password func(*CLI) string
	}{
		{"consul", func(cli *CLI) string { return cli.Consul.Password }},
		{"etcd", func(cli *CLI) string { return cli.Etcd.Password }},
		{"vault", func(cli *CLI) string { return cli.Vault.Password }},
	}
	for _, tt := range tests {
		t.Run(tt.backend, func(t *testing.T) {
			cli, _ := parseCLI(t, []string{tt.backend})
			if got := tt.password(cli); got != "" {
				t.Errorf("%s password = %q, want it to ignore %s", tt.backend, got, redisPasswordEnv)
			}
			if cli.configSources.isExplicit("password") {
				t.Errorf("%s treats password as explicitly set because of %s", tt.backend, redisPasswordEnv)
			}
		})
	}
}

func TestRedisCmdKeepsFlags(t *testing.T) {
	unsetEnv(t, redisPasswordEnv)
	cli, _ := parseCLI(t, []string{
		"redis",
		"--node", "127.0.0.1:6379",
		"--password", "secret",
		"--username", "user",
		"--basic-auth",
		"--auth-token", "token",
		"--client-key", "key",
		"--client-cert", "cert",
		"--client-ca-keys", "ca",
		"--separator", ":",
	})
	cmd := cli.Redis
	got := strings.Join([]string{
		strings.Join(cmd.Node, ","), cmd.Password, cmd.Username, cmd.AuthToken,
		cmd.ClientKey, cmd.ClientCert, cmd.ClientCaKeys, cmd.Separator,
	}, " ")
	want := "127.0.0.1:6379 secret user token key cert ca :"
	if got != want {
		t.Errorf("parsed redis flags = %q, want %q", got, want)
	}
	if !cmd.BasicAuth {
		t.Error("--basic-auth was not parsed")
	}
}
