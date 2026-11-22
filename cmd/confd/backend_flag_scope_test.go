package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/abtreece/confd/pkg/backends"
)

// backendArgs is the command line that selects each backend, including the
// flags the backend requires.
var backendArgs = map[string][]string{
	"acm":            {"acm"},
	"consul":         {"consul"},
	"dynamodb":       {"dynamodb", "--table", "test"},
	"env":            {"env"},
	"etcd":           {"etcd"},
	"file":           {"file", "--file", "data.yaml"},
	"imds":           {"imds"},
	"redis":          {"redis"},
	"secretsmanager": {"secretsmanager"},
	"ssm":            {"ssm"},
	"vault":          {"vault"},
	"zookeeper":      {"zookeeper"},
}

// scopedSettings are the six timeout and retry settings, each with a value
// that differs from its default.
var scopedSettings = []struct {
	flag      string
	envVar    string
	tomlKey   string
	value     string
	tomlValue string
}{
	{"--dial-timeout", "CONFD_DIAL_TIMEOUT", "dial_timeout", "2s", `"2s"`},
	{"--read-timeout", "CONFD_READ_TIMEOUT", "read_timeout", "2s", `"2s"`},
	{"--write-timeout", "CONFD_WRITE_TIMEOUT", "write_timeout", "2s", `"2s"`},
	{"--retry-max-attempts", "CONFD_RETRY_MAX_ATTEMPTS", "retry_max_attempts", "5", `5`},
	{"--retry-base-delay", "CONFD_RETRY_BASE_DELAY", "retry_base_delay", "200ms", `"200ms"`},
	{"--retry-max-delay", "CONFD_RETRY_MAX_DELAY", "retry_max_delay", "3s", `"3s"`},
}

// flagWarnings parses args with env applied, loads a confd.toml holding
// tomlContent, and returns the ignored-flag warnings for the backend.
func flagWarnings(t *testing.T, backend string, env map[string]string, extraArgs []string, tomlContent string) []string {
	t.Helper()

	clearCONFDEnvVars(t)
	unsetEnv(t, redisPasswordEnv)
	for key, value := range env {
		t.Setenv(key, value)
	}

	configPath := filepath.Join(t.TempDir(), "confd.toml")
	if err := os.WriteFile(configPath, []byte(tomlContent), 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	args := slices.Concat([]string{"--config-file", configPath}, backendArgs[backend], extraArgs)
	cli, _ := parseCLI(t, args)
	cfg := backends.Config{Backend: backend}
	if err := loadConfigFile(cli, &cfg); err != nil {
		t.Fatalf("loadConfigFile: %v", err)
	}
	return ignoredBackendFlagWarnings(cli.configSources, backend)
}

func TestIgnoredBackendFlagWarnings(t *testing.T) {
	// The settings each backend uses, read from the constructor calls in
	// backends.New. Everything not listed is ignored and must warn.
	uses := map[string][]string{
		"acm":            {"--dial-timeout"},
		"consul":         {},
		"dynamodb":       {"--dial-timeout"},
		"env":            {},
		"etcd":           {"--dial-timeout"},
		"file":           {},
		"imds":           {"--dial-timeout"},
		"redis":          {"--dial-timeout", "--read-timeout", "--write-timeout", "--retry-max-attempts", "--retry-base-delay", "--retry-max-delay"},
		"secretsmanager": {"--dial-timeout"},
		"ssm":            {"--dial-timeout"},
		"vault":          {},
		"zookeeper":      {"--dial-timeout"},
	}

	t.Run("covers every backend subcommand", func(t *testing.T) {
		_, ctx := parseCLI(t, []string{"env"})
		var subcommands []string
		for _, child := range ctx.Model.Children {
			subcommands = append(subcommands, child.Name)
		}
		slices.Sort(subcommands)
		covered := make([]string, 0, len(uses))
		for backend := range uses {
			covered = append(covered, backend)
		}
		slices.Sort(covered)
		if !slices.Equal(subcommands, covered) {
			t.Errorf("test covers backends %v, CLI has %v", covered, subcommands)
		}
	})

	sources := []struct {
		name   string
		supply func(flag, envVar, tomlKey, value, tomlValue string) (env map[string]string, args []string, tomlContent string)
	}{
		{"flag", func(flag, _, _, value, _ string) (map[string]string, []string, string) {
			return nil, []string{flag, value}, ""
		}},
		{"env var", func(_, envVar, _, value, _ string) (map[string]string, []string, string) {
			return map[string]string{envVar: value}, nil, ""
		}},
		{"confd.toml", func(_, _, tomlKey, _, tomlValue string) (map[string]string, []string, string) {
			return nil, nil, tomlKey + " = " + tomlValue + "\n"
		}},
	}

	for backend, used := range uses {
		for _, setting := range scopedSettings {
			for _, source := range sources {
				t.Run(backend+"/"+setting.flag+"/"+source.name, func(t *testing.T) {
					env, args, tomlContent := source.supply(setting.flag, setting.envVar, setting.tomlKey, setting.value, setting.tomlValue)
					got := flagWarnings(t, backend, env, args, tomlContent)

					var want []string
					if !slices.Contains(used, setting.flag) {
						want = []string{setting.flag + " has no effect with the " + backend + " backend"}
					}
					if !slices.Equal(got, want) {
						t.Errorf("warnings = %q, want %q", got, want)
					}
				})
			}
		}
	}

	t.Run("nothing supplied", func(t *testing.T) {
		for backend := range uses {
			if got := flagWarnings(t, backend, nil, nil, ""); len(got) != 0 {
				t.Errorf("%s: warnings = %q, want none", backend, got)
			}
		}
	})

	t.Run("supplied with the default value", func(t *testing.T) {
		want := []string{"--retry-max-attempts has no effect with the env backend"}
		if got := flagWarnings(t, "env", nil, []string{"--retry-max-attempts", "3"}, ""); !slices.Equal(got, want) {
			t.Errorf("flag: warnings = %q, want %q", got, want)
		}
		if got := flagWarnings(t, "env", map[string]string{"CONFD_RETRY_MAX_ATTEMPTS": "3"}, nil, ""); !slices.Equal(got, want) {
			t.Errorf("env var: warnings = %q, want %q", got, want)
		}
		if got := flagWarnings(t, "env", nil, nil, "retry_max_attempts = 3\n"); !slices.Equal(got, want) {
			t.Errorf("confd.toml: warnings = %q, want %q", got, want)
		}
	})

	t.Run("one warning per flag when supplied more than one way", func(t *testing.T) {
		want := []string{"--read-timeout has no effect with the etcd backend"}
		got := flagWarnings(t, "etcd", map[string]string{"CONFD_READ_TIMEOUT": "3s"}, []string{"--read-timeout", "2s"}, `read_timeout = "4s"`+"\n")
		if !slices.Equal(got, want) {
			t.Errorf("warnings = %q, want %q", got, want)
		}
	})

	t.Run("all six supplied", func(t *testing.T) {
		var tomlLines []string
		for _, setting := range scopedSettings {
			tomlLines = append(tomlLines, setting.tomlKey+" = "+setting.tomlValue)
		}
		tomlContent := strings.Join(tomlLines, "\n") + "\n"

		if got := flagWarnings(t, "redis", nil, nil, tomlContent); len(got) != 0 {
			t.Errorf("redis: warnings = %q, want none", got)
		}
		got := flagWarnings(t, "vault", nil, nil, tomlContent)
		if len(got) != len(scopedSettings) {
			t.Fatalf("vault: got %d warnings %q, want %d", len(got), got, len(scopedSettings))
		}
		for i, setting := range scopedSettings {
			if want := setting.flag + " has no effect with the vault backend"; got[i] != want {
				t.Errorf("vault: warning %d = %q, want %q", i, got[i], want)
			}
		}
	})

	t.Run("unrelated settings do not warn", func(t *testing.T) {
		got := flagWarnings(t, "env", map[string]string{"CONFD_INTERVAL": "30"}, []string{"--backend-timeout", "3s"}, `prefix = "/app"`+"\n")
		if len(got) != 0 {
			t.Errorf("warnings = %q, want none", got)
		}
	})
}
