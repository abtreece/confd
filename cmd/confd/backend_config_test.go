package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/util"
	"github.com/alecthomas/kong"
)

// backendConfigCases has one entry per backend command: a command line that
// sets every flag the command copies into backends.Config, each to a value
// that differs from its default and from the other flags, and the config the
// command's backendConfig method must build from it.
var backendConfigCases = map[string]struct {
	args []string
	want backends.Config
}{
	"consul": {
		args: []string{
			"consul", "--node", "consul-a:8500", "--node", "consul-b:8500", "--scheme", "https",
			"--client-cert", "/cert", "--client-key", "/key", "--client-ca-keys", "/ca",
			"--basic-auth", "--username", "user", "--password", "pass",
		},
		want: backends.Config{
			Backend: "consul", BackendNodes: util.Nodes{"consul-a:8500", "consul-b:8500"}, Scheme: "https",
			ClientCert: "/cert", ClientKey: "/key", ClientCaKeys: "/ca",
			BasicAuth: true, Username: "user", Password: "pass",
		},
	},
	"etcd": {
		args: []string{
			"etcd", "--node", "http://etcd:2379", "--scheme", "https",
			"--client-cert", "/cert", "--client-key", "/key", "--client-ca-keys", "/ca", "--client-insecure",
			"--basic-auth", "--username", "user", "--password", "pass",
		},
		want: backends.Config{
			Backend: "etcd", BackendNodes: util.Nodes{"http://etcd:2379"}, Scheme: "https",
			ClientCert: "/cert", ClientKey: "/key", ClientCaKeys: "/ca", ClientInsecure: true,
			BasicAuth: true, Username: "user", Password: "pass",
		},
	},
	"vault": {
		args: []string{
			"vault", "--node", "http://vault:8200",
			"--client-cert", "/cert", "--client-key", "/key", "--client-ca-keys", "/ca",
			"--auth-type", "app-role", "--auth-token", "token",
			"--app-id", "app", "--user-id", "user-id", "--role-id", "role", "--secret-id", "secret",
			"--path", "mount", "--username", "user", "--password", "pass",
		},
		want: backends.Config{
			Backend: "vault", BackendNodes: util.Nodes{"http://vault:8200"},
			ClientCert: "/cert", ClientKey: "/key", ClientCaKeys: "/ca",
			AuthType: "app-role", AuthToken: "token",
			AppID: "app", UserID: "user-id", RoleID: "role", SecretID: "secret",
			Path: "mount", Username: "user", Password: "pass",
		},
	},
	"redis": {
		args: []string{
			"redis", "--node", "redis:6379",
			"--client-cert", "/cert", "--client-key", "/key", "--client-ca-keys", "/ca",
			"--username", "user", "--password", "pass", "--separator", ":",
		},
		want: backends.Config{
			Backend: "redis", BackendNodes: util.Nodes{"redis:6379"},
			ClientCert: "/cert", ClientKey: "/key", ClientCaKeys: "/ca",
			Username: "user", Password: "pass", Separator: ":",
		},
	},
	"zookeeper": {
		args: []string{"zookeeper", "--node", "zk:2181"},
		want: backends.Config{Backend: "zookeeper", BackendNodes: util.Nodes{"zk:2181"}},
	},
	"dynamodb": {
		args: []string{"dynamodb", "--table", "confd"},
		want: backends.Config{Backend: "dynamodb", Table: "confd"},
	},
	"ssm": {
		args: []string{"ssm"},
		want: backends.Config{Backend: "ssm"},
	},
	"acm": {
		args: []string{"acm", "--acm-export-private-key"},
		want: backends.Config{Backend: "acm", ACMExportPrivateKey: true},
	},
	"secretsmanager": {
		args: []string{"secretsmanager", "--secretsmanager-version-stage", "AWSPREVIOUS", "--secretsmanager-no-flatten"},
		want: backends.Config{Backend: "secretsmanager", SecretsManagerVersionStage: "AWSPREVIOUS", SecretsManagerNoFlatten: true},
	},
	"imds": {
		args: []string{"imds", "--imds-cache-ttl", "5m"},
		want: backends.Config{Backend: "imds", IMDSCacheTTL: 5 * time.Minute},
	},
	"env": {
		args: []string{"env"},
		want: backends.Config{Backend: "env"},
	},
	"file": {
		args: []string{"file", "--file", "a.yaml", "--file", "b.json", "--filter", "*.yaml"},
		want: backends.Config{Backend: "file", YAMLFile: util.Nodes{"a.yaml", "b.json"}, Filter: "*.yaml"},
	},
}

// TestBackendConfigMapsEveryFlag checks that each backend command copies every
// flag it maps into the matching backends.Config field, and nothing else.
func TestBackendConfigMapsEveryFlag(t *testing.T) {
	for command, tc := range backendConfigCases {
		t.Run(command, func(t *testing.T) {
			clearCONFDEnvVars(t)
			unsetEnv(t, redisPasswordEnv)

			cli, ctx := parseCLI(t, tc.args)
			if got := backendConfigFor(t, cli, ctx); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("backendConfig:\n got: %+v\nwant: %+v", got, tc.want)
			}
		})
	}
}

// TestEveryBackendCommandBuildsItsConfig fails when a subcommand does not
// implement backendCommand, or has no entry in backendConfigCases.
func TestEveryBackendCommandBuildsItsConfig(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli, append(parserOptions(), kong.Exit(func(int) {}))...)
	if err != nil {
		t.Fatalf("build parser: %v", err)
	}

	commands := 0
	for _, node := range parser.Model.Node.Children {
		if node.Type != kong.CommandNode {
			continue
		}
		commands++
		if _, ok := node.Target.Addr().Interface().(backendCommand); !ok {
			t.Errorf("the %s command does not implement backendCommand", node.Name)
		}
		if _, ok := backendConfigCases[node.Name]; !ok {
			t.Errorf("backendConfigCases has no entry for the %s command", node.Name)
		}
	}
	if commands != len(backendConfigCases) {
		t.Errorf("the CLI defines %d commands, backendConfigCases has %d entries", commands, len(backendConfigCases))
	}
}
