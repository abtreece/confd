package main

import (
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/alecthomas/kong"
)

// stringEnvFlag is a string flag that can be given through an environment
// variable, as found in the CLI definition.
type stringEnvFlag struct {
	// flag is the flag's long name, without the leading dashes.
	flag string
	// commands are the subcommands that define the flag; a global flag has
	// none.
	commands []string
}

// stringEnvFlags walks the CLI definition and returns every string flag with
// an env tag, keyed by variable name. The parser is built without confd's own
// options, so the result does not depend on the rule under test or on the
// environment.
func stringEnvFlags(t *testing.T) map[string]*stringEnvFlag {
	t.Helper()

	var cli CLI
	parser, err := kong.New(&cli, kong.Exit(func(int) {}))
	if err != nil {
		t.Fatalf("build parser: %v", err)
	}
	found := make(map[string]*stringEnvFlag)
	var visit func(node *kong.Node, command string)
	visit = func(node *kong.Node, command string) {
		for _, flag := range node.Flags {
			if !flag.Target.IsValid() || flag.Target.Kind() != reflect.String {
				continue
			}
			for _, env := range flag.Envs {
				entry, ok := found[env]
				if !ok {
					entry = &stringEnvFlag{flag: flag.Name}
					found[env] = entry
				}
				if entry.flag != flag.Name {
					t.Fatalf("%s is bound to both --%s and --%s", env, entry.flag, flag.Name)
				}
				if command != "" {
					entry.commands = append(entry.commands, command)
				}
			}
		}
		for _, child := range node.Children {
			visit(child, child.Name)
		}
	}
	visit(parser.Model.Node, "")
	return found
}

// emptyEnvBackendConfigs builds the backend config from the parsed command
// the way that command's Run method does, for every command a string
// environment variable can reach.
var emptyEnvBackendConfigs = map[string]func(*CLI) backends.Config{
	"env": func(*CLI) backends.Config {
		return backends.Config{Backend: "env"}
	},
	"consul": func(cli *CLI) backends.Config {
		c := cli.Consul
		return backends.Config{
			Backend: "consul", BackendNodes: c.Node, Scheme: c.Scheme,
			ClientCert: c.ClientCert, ClientKey: c.ClientKey, ClientCaKeys: c.ClientCaKeys,
			BasicAuth: c.BasicAuth, Username: c.Username, Password: c.Password,
		}
	},
	"etcd": func(cli *CLI) backends.Config {
		e := cli.Etcd
		return backends.Config{
			Backend: "etcd", BackendNodes: e.Node, Scheme: e.Scheme,
			ClientCert: e.ClientCert, ClientKey: e.ClientKey, ClientCaKeys: e.ClientCaKeys,
			ClientInsecure: e.ClientInsecure,
			BasicAuth:      e.BasicAuth, Username: e.Username, Password: e.Password,
		}
	},
	"vault": func(cli *CLI) backends.Config {
		v := cli.Vault
		return backends.Config{
			Backend: "vault", BackendNodes: v.Node,
			ClientCert: v.ClientCert, ClientKey: v.ClientKey, ClientCaKeys: v.ClientCaKeys,
			AuthType: v.AuthType, AuthToken: v.AuthToken,
			AppID: v.AppID, UserID: v.UserID, RoleID: v.RoleID, SecretID: v.SecretID,
			Path: v.Path, Username: v.Username, Password: v.Password,
		}
	},
	"redis": func(cli *CLI) backends.Config {
		cli.Redis.resolvePassword(cli)
		return cli.Redis.backendConfig()
	},
}

// emptyEnvCase says how one string environment variable is checked.
type emptyEnvCase struct {
	// tomlKey and tomlValue are the confd.toml key for the same setting and a
	// valid value for it. tomlKey is empty when confd.toml has no such key.
	tomlKey   string
	tomlValue string
	// flagValue is a valid value for the flag that differs from tomlValue and
	// from the built-in default.
	flagValue string
	// get returns the resolved setting.
	get func(*CLI, *backends.Config) string
}

// emptyEnvCases has one entry for every string environment variable.
// TestEmptyEnvVarIsUnset fails when the CLI definition and this table differ.
var emptyEnvCases = map[string]emptyEnvCase{
	"CONFD_CONFDIR": {
		tomlKey: "confdir", tomlValue: "/toml/confd", flagValue: "/flag/confd",
		get: func(cli *CLI, _ *backends.Config) string { return cli.ConfDir },
	},
	"CONFD_CONFIG_FILE": {
		flagValue: "/flag/confd.toml",
		get:       func(cli *CLI, _ *backends.Config) string { return cli.ConfigFile },
	},
	"CONFD_LOG_LEVEL": {
		tomlKey: "log_level", tomlValue: "debug", flagValue: "warn",
		get: func(cli *CLI, _ *backends.Config) string { return cli.LogLevel },
	},
	"CONFD_LOG_FORMAT": {
		tomlKey: "log_format", tomlValue: "json", flagValue: "text",
		get: func(cli *CLI, _ *backends.Config) string { return cli.LogFormat },
	},
	"CONFD_PREFIX": {
		tomlKey: "prefix", tomlValue: "/toml", flagValue: "/flag",
		get: func(cli *CLI, _ *backends.Config) string { return cli.Prefix },
	},
	"CONFD_FAILURE_MODE": {
		tomlKey: "failure_mode", tomlValue: "fail-fast", flagValue: "best-effort",
		get: func(cli *CLI, _ *backends.Config) string { return cli.FailureMode },
	},
	"CONFD_SRV_DOMAIN": {
		tomlKey: "srv_domain", tomlValue: "toml.example.com", flagValue: "flag.example.com",
		get: func(cli *CLI, _ *backends.Config) string { return cli.SRVDomain },
	},
	"CONFD_SRV_RECORD": {
		tomlKey: "srv_record", tomlValue: "_toml._tcp.example.com", flagValue: "_flag._tcp.example.com",
		get: func(cli *CLI, _ *backends.Config) string { return cli.SRVRecord },
	},
	"CONFD_DEBOUNCE": {
		tomlKey: "debounce", tomlValue: "3s", flagValue: "7s",
		get: func(cli *CLI, _ *backends.Config) string { return cli.DebounceStr },
	},
	"CONFD_BATCH_INTERVAL": {
		flagValue: "7s",
		get:       func(cli *CLI, _ *backends.Config) string { return cli.BatchIntervalStr },
	},
	"CONFD_METRICS_ADDR": {
		tomlKey: "metrics_addr", tomlValue: "127.0.0.1:9100", flagValue: "127.0.0.1:9200",
		get: func(cli *CLI, _ *backends.Config) string { return cli.MetricsAddr },
	},
	"CONFD_CLIENT_CERT": {
		tomlKey: "client_cert", tomlValue: "/toml/cert", flagValue: "/flag/cert",
		get: func(_ *CLI, cfg *backends.Config) string { return cfg.ClientCert },
	},
	"CONFD_CLIENT_KEY": {
		tomlKey: "client_key", tomlValue: "/toml/key", flagValue: "/flag/key",
		get: func(_ *CLI, cfg *backends.Config) string { return cfg.ClientKey },
	},
	"CONFD_CLIENT_CAKEYS": {
		tomlKey: "client_cakeys", tomlValue: "/toml/ca", flagValue: "/flag/ca",
		get: func(_ *CLI, cfg *backends.Config) string { return cfg.ClientCaKeys },
	},
	"VAULT_TOKEN": {
		tomlKey: "auth_token", tomlValue: "toml-token", flagValue: "flag-token",
		get: func(_ *CLI, cfg *backends.Config) string { return cfg.AuthToken },
	},
	redisPasswordEnv: {
		tomlKey: "password", tomlValue: "toml-password", flagValue: "flag-password",
		get: func(_ *CLI, cfg *backends.Config) string { return cfg.Password },
	},
}

// TestEmptyEnvVarIsUnset checks, for every string flag with an env tag, that
// the variable set to the empty string resolves exactly as the variable unset:
// the confd.toml value applies, then the built-in default, and a flag still
// wins. It fails when such a flag has no entry in emptyEnvCases.
func TestEmptyEnvVarIsUnset(t *testing.T) {
	flags := stringEnvFlags(t)
	allEnvs := slices.Sorted(maps.Keys(flags))

	for env := range emptyEnvCases {
		if _, ok := flags[env]; !ok {
			t.Errorf("emptyEnvCases has %s, which no string flag in the CLI definition reads", env)
		}
	}

	// resolved is the outcome of one run: the parsed CLI, with its recorded
	// sources, and the backend config, after confd.toml and the environment
	// are merged in the way run() does.
	type resolved struct {
		cli *CLI
		cfg backends.Config
	}
	// resolve parses args with env either unset or set to the empty string.
	// With loadFile false the config file is not read, for the one setting
	// that is the config file's own path.
	resolve := func(t *testing.T, env string, empty bool, command string, args []string, loadFile bool) resolved {
		t.Helper()
		clearCONFDEnvVars(t)
		for _, name := range allEnvs {
			unsetEnv(t, name)
		}
		if empty {
			t.Setenv(env, "")
		}
		cli, _ := parseCLI(t, args)
		buildConfig, ok := emptyEnvBackendConfigs[command]
		if !ok {
			t.Fatalf("emptyEnvBackendConfigs has no entry for the %s command", command)
		}
		cfg := buildConfig(cli)
		if loadFile {
			resolveConfig(t, cli, &cfg)
		}
		return resolved{cli: cli, cfg: cfg}
	}

	for _, env := range allEnvs {
		t.Run(env, func(t *testing.T) {
			tc, ok := emptyEnvCases[env]
			if !ok {
				t.Fatalf("--%s is a string flag read from %s, but emptyEnvCases has no entry for it", flags[env].flag, env)
			}
			flag := flags[env].flag
			commands := flags[env].commands
			if len(commands) == 0 {
				commands = []string{"env"}
			}
			isConfigFile := flag == "config-file"

			for _, command := range commands {
				t.Run(command, func(t *testing.T) {
					// check compares the variable empty against the variable
					// unset for one command line, and returns the empty run.
					check := func(t *testing.T, args []string, loadFile bool) resolved {
						t.Helper()
						unset := resolve(t, env, false, command, args, loadFile)
						empty := resolve(t, env, true, command, args, loadFile)
						if empty.cli.configSources.env[normalizeConfigName(flag)] {
							t.Errorf("%s= is recorded as an environment source for --%s", env, flag)
						}
						if got, want := tc.get(empty.cli, &empty.cfg), tc.get(unset.cli, &unset.cfg); got != want {
							t.Errorf("%s= resolves --%s to %q, want %q as with the variable unset", env, flag, got, want)
						}
						if !reflect.DeepEqual(empty, unset) {
							t.Errorf("%s= does not resolve as the variable unset:\n empty: %+v %+v\n unset: %+v %+v",
								env, *empty.cli, empty.cfg, *unset.cli, unset.cfg)
						}
						return empty
					}

					noFile := filepath.Join(t.TempDir(), "missing.toml")
					withFile := func(args ...string) []string {
						if isConfigFile {
							return args
						}
						return append([]string{"--config-file", noFile}, args...)
					}

					t.Run("default", func(t *testing.T) {
						got := check(t, withFile(command), !isConfigFile)
						if got.cli.configSources.isExplicit(flag) {
							t.Errorf("%s= makes --%s explicitly set", env, flag)
						}
					})

					t.Run("flag wins", func(t *testing.T) {
						got := check(t, withFile(command, "--"+flag, tc.flagValue), !isConfigFile)
						if value := tc.get(got.cli, &got.cfg); value != tc.flagValue {
							t.Errorf("--%s %s with %s= resolves to %q", flag, tc.flagValue, env, value)
						}
					})

					if tc.tomlKey == "" {
						return
					}
					configFile := writeConfigFile(t, tc.tomlKey+` = "`+tc.tomlValue+`"`)

					t.Run("confd.toml applies", func(t *testing.T) {
						got := check(t, []string{"--config-file", configFile, command}, true)
						if got.cli.configSources.isExplicit(flag) {
							t.Errorf("%s= makes --%s explicitly set", env, flag)
						}
						if value := tc.get(got.cli, &got.cfg); value != tc.tomlValue {
							t.Errorf("%s= with %s = %q in confd.toml resolves to %q, want the file's value", env, tc.tomlKey, tc.tomlValue, value)
						}
					})

					t.Run("flag wins over confd.toml", func(t *testing.T) {
						got := check(t, []string{"--config-file", configFile, command, "--" + flag, tc.flagValue}, true)
						if value := tc.get(got.cli, &got.cfg); value != tc.flagValue {
							t.Errorf("--%s %s with %s= and %s in confd.toml resolves to %q", flag, tc.flagValue, env, tc.tomlKey, value)
						}
					})
				})
			}
		})
	}
}
