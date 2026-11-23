package main

import (
	"slices"
	"testing"

	"github.com/BurntSushi/toml"
)

// The tests in this file cover source tracking only: which options Kong
// reports as explicitly set on the command line or in the environment.
// Parsed values are covered in cli_test.go and the merge with confd.toml in
// config_test.go.

// sourceNames returns the option names recorded in one source map, sorted.
func sourceNames(sources map[string]bool) []string {
	names := make([]string, 0, len(sources))
	for name, set := range sources {
		if set {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// parseSources parses args with env applied and returns the recorded sources.
func parseSources(t *testing.T, env map[string]string, args []string) *configSources {
	t.Helper()

	clearCONFDEnvVars(t)
	unsetEnv(t, redisPasswordEnv)
	for key, value := range env {
		t.Setenv(key, value)
	}
	cli, _ := parseCLI(t, args)
	if cli.configSources == nil {
		t.Fatal("configSources is nil after parsing, want it set by AfterApply")
	}
	return cli.configSources
}

func assertSources(t *testing.T, sources *configSources, wantCLI, wantEnv []string) {
	t.Helper()

	if got := sourceNames(sources.cli); !slices.Equal(got, wantCLI) {
		t.Errorf("cli sources = %v, want %v", got, wantCLI)
	}
	if got := sourceNames(sources.env); !slices.Equal(got, wantEnv) {
		t.Errorf("env sources = %v, want %v", got, wantEnv)
	}
	for _, name := range slices.Concat(wantCLI, wantEnv) {
		if !sources.isExplicit(name) {
			t.Errorf("isExplicit(%q) = false, want true", name)
		}
	}
}

func TestSourceTracking_GlobalFlags(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		args    []string
		wantCLI []string
		wantEnv []string
	}{
		{
			name: "nothing set",
			args: []string{"env"},
		},
		{
			name:    "int flag",
			args:    []string{"--interval", "120", "env"},
			wantCLI: []string{"interval"},
		},
		{
			name:    "flag set to its default value",
			args:    []string{"--interval", "600", "env"},
			wantCLI: []string{"interval"},
		},
		{
			name:    "hyphenated flag is normalized to the toml spelling",
			args:    []string{"--log-level", "debug", "env"},
			wantCLI: []string{"log_level"},
		},
		{
			name:    "bool flag",
			args:    []string{"--noop", "env"},
			wantCLI: []string{"noop"},
		},
		{
			name:    "bool flag set to false",
			args:    []string{"--noop=false", "env"},
			wantCLI: []string{"noop"},
		},
		{
			name:    "negated bool flag",
			args:    []string{"--no-template-cache", "env"},
			wantCLI: []string{"template_cache"},
		},
		{
			name:    "duration flag",
			args:    []string{"--backend-timeout", "45s", "env"},
			wantCLI: []string{"backend_timeout"},
		},
		{
			name:    "global flag after the subcommand",
			args:    []string{"env", "--prefix", "/app"},
			wantCLI: []string{"prefix"},
		},
		{
			name:    "several flags",
			args:    []string{"--confdir", "/tmp/confd", "--watch", "--failure-mode", "fail-fast", "env"},
			wantCLI: []string{"confdir", "failure_mode", "watch"},
		},
		{
			name:    "int env var",
			env:     map[string]string{"CONFD_INTERVAL": "300"},
			args:    []string{"env"},
			wantEnv: []string{"interval"},
		},
		{
			name:    "env var set to the default value",
			env:     map[string]string{"CONFD_FAILURE_MODE": "best-effort"},
			args:    []string{"env"},
			wantEnv: []string{"failure_mode"},
		},
		{
			name:    "bool env var set to false",
			env:     map[string]string{"CONFD_WATCH": "false"},
			args:    []string{"env"},
			wantEnv: []string{"watch"},
		},
		{
			name:    "duration env var",
			env:     map[string]string{"CONFD_DIAL_TIMEOUT": "9s"},
			args:    []string{"env"},
			wantEnv: []string{"dial_timeout"},
		},
		{
			name:    "env var set to the empty string",
			env:     map[string]string{"CONFD_PREFIX": ""},
			args:    []string{"env"},
			wantEnv: []string{"prefix"},
		},
		{
			name:    "several env vars",
			env:     map[string]string{"CONFD_LOG_FORMAT": "json", "CONFD_SYNC_ONLY": "true"},
			args:    []string{"env"},
			wantEnv: []string{"log_format", "sync_only"},
		},
		{
			name:    "flag and env var for the same option",
			env:     map[string]string{"CONFD_INTERVAL": "300"},
			args:    []string{"--interval", "120", "env"},
			wantCLI: []string{"interval"},
			wantEnv: []string{"interval"},
		},
		{
			name:    "flag and env var for different options",
			env:     map[string]string{"CONFD_PREFIX": "/env"},
			args:    []string{"--noop", "env"},
			wantCLI: []string{"noop"},
			wantEnv: []string{"prefix"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := parseSources(t, tt.env, tt.args)
			assertSources(t, sources, tt.wantCLI, tt.wantEnv)
		})
	}
}

func TestSourceTracking_BackendFlags(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		args    []string
		wantCLI []string
		wantEnv []string
		// tomlKey, when set, is a confd.toml key that is defined in the config
		// file; tomlAliases are the other names loadConfigFile checks for it.
		// wantTOMLApplied says whether that TOML value may still be applied.
		tomlKey         string
		tomlAliases     []string
		wantTOMLApplied bool
	}{
		{
			name:            "no backend flags",
			args:            []string{"consul"},
			tomlKey:         "nodes",
			tomlAliases:     []string{"node"},
			wantTOMLApplied: true,
		},
		{
			name:        "node flag blocks toml nodes",
			args:        []string{"consul", "--node", "cli:8500"},
			wantCLI:     []string{"node"},
			tomlKey:     "nodes",
			tomlAliases: []string{"node"},
		},
		{
			name:        "short node flag blocks toml nodes",
			args:        []string{"etcd", "-n", "http://a:2379", "-n", "http://b:2379"},
			wantCLI:     []string{"node"},
			tomlKey:     "nodes",
			tomlAliases: []string{"node"},
		},
		{
			name:    "scheme flag",
			args:    []string{"etcd", "--scheme", "https"},
			wantCLI: []string{"scheme"},
			tomlKey: "scheme",
		},
		{
			name:    "scheme flag set to its default value",
			args:    []string{"consul", "--scheme", "http"},
			wantCLI: []string{"scheme"},
			tomlKey: "scheme",
		},
		{
			name:            "scheme left at its default is not explicit",
			args:            []string{"consul", "--node", "cli:8500"},
			wantCLI:         []string{"node"},
			tomlKey:         "scheme",
			wantTOMLApplied: true,
		},
		{
			name:            "file flag",
			args:            []string{"file", "--file", "a.yaml", "--file", "b.yaml"},
			wantCLI:         []string{"file"},
			tomlKey:         "filter",
			wantTOMLApplied: true,
		},
		{
			name:    "file and filter flags",
			args:    []string{"file", "--file", "dir/", "--filter", "*.yaml"},
			wantCLI: []string{"file", "filter"},
			tomlKey: "filter",
		},
		{
			name:    "secrets manager flags",
			args:    []string{"secretsmanager", "--secretsmanager-version-stage", "AWSPREVIOUS", "--secretsmanager-no-flatten"},
			wantCLI: []string{"secretsmanager_no_flatten", "secretsmanager_version_stage"},
			tomlKey: "secretsmanager_version_stage",
		},
		{
			name:    "secrets manager version stage set to its default value",
			args:    []string{"secretsmanager", "--secretsmanager-version-stage", "AWSCURRENT"},
			wantCLI: []string{"secretsmanager_version_stage"},
			tomlKey: "secretsmanager_version_stage",
		},
		{
			name:    "secrets manager no-flatten set to false",
			args:    []string{"secretsmanager", "--secretsmanager-no-flatten=false"},
			wantCLI: []string{"secretsmanager_no_flatten"},
			tomlKey: "secretsmanager_no_flatten",
		},
		{
			name:    "acm flag",
			args:    []string{"acm", "--acm-export-private-key"},
			wantCLI: []string{"acm_export_private_key"},
			tomlKey: "acm_export_private_key",
		},
		{
			name:    "acm env var",
			env:     map[string]string{"ACM_EXPORT_PRIVATE_KEY": "true"},
			args:    []string{"acm"},
			wantEnv: []string{"acm_export_private_key"},
			tomlKey: "acm_export_private_key",
		},
		{
			name:            "env var of a backend that is not selected",
			env:             map[string]string{"ACM_EXPORT_PRIVATE_KEY": "true"},
			args:            []string{"env"},
			tomlKey:         "acm_export_private_key",
			wantTOMLApplied: true,
		},
		{
			name:    "vault token env var",
			env:     map[string]string{"VAULT_TOKEN": "env-token"},
			args:    []string{"vault"},
			wantEnv: []string{"auth_token"},
			tomlKey: "auth_token",
		},
		{
			name:    "vault token flag and env var",
			env:     map[string]string{"VAULT_TOKEN": "env-token"},
			args:    []string{"vault", "--auth-token", "cli-token"},
			wantCLI: []string{"auth_token"},
			wantEnv: []string{"auth_token"},
			tomlKey: "auth_token",
		},
		{
			name:        "client ca keys flag blocks toml client_cakeys",
			args:        []string{"etcd", "--client-ca-keys", "/cli/ca"},
			wantCLI:     []string{"client_ca_keys"},
			tomlKey:     "client_cakeys",
			tomlAliases: []string{"client_ca_keys"},
		},
		{
			name:        "client ca keys env var blocks toml client_cakeys",
			env:         map[string]string{"CONFD_CLIENT_CAKEYS": "/env/ca"},
			args:        []string{"etcd"},
			wantEnv:     []string{"client_ca_keys"},
			tomlKey:     "client_cakeys",
			tomlAliases: []string{"client_ca_keys"},
		},
		{
			name:    "backend and global flags together",
			env:     map[string]string{"CONFD_INTERVAL": "300"},
			args:    []string{"--watch", "redis", "--node", "cli:6379", "--separator", ":"},
			wantCLI: []string{"node", "separator", "watch"},
			wantEnv: []string{"interval"},
			tomlKey: "separator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := parseSources(t, tt.env, tt.args)
			assertSources(t, sources, tt.wantCLI, tt.wantEnv)

			if tt.tomlKey == "" {
				return
			}
			// The value type is irrelevant here: only whether the key is
			// defined matters to shouldApplyTOML.
			var decoded map[string]any
			md, err := toml.Decode(tt.tomlKey+` = "defined"`, &decoded)
			if err != nil {
				t.Fatalf("failed to decode toml: %v", err)
			}
			if got := shouldApplyTOML(sources, md, tt.tomlKey, tt.tomlAliases...); got != tt.wantTOMLApplied {
				t.Errorf("shouldApplyTOML(%q, %v) = %v, want %v", tt.tomlKey, tt.tomlAliases, got, tt.wantTOMLApplied)
			}
		})
	}
}
