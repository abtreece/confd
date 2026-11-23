package backends

import (
	"fmt"
	"slices"
)

// settingScope lists the backends that use each connection timeout, retry,
// auth and TLS setting that not every backend uses. It mirrors the constructor
// calls in New (client.go): a backend uses a setting only if New passes that
// setting to its constructor. Update this table whenever those calls change.
var settingScope = []struct {
	// flag is the command-line name, used in the warning.
	flag string
	// keys are the TOML spellings of the setting, in confd.toml and in a
	// per-resource [backend] section.
	keys   []string
	usedBy []string
}{
	{flag: "dial-timeout", keys: []string{"dial_timeout"}, usedBy: []string{"acm", "dynamodb", "etcd", "imds", "redis", "secretsmanager", "ssm", "zookeeper"}},
	{flag: "read-timeout", keys: []string{"read_timeout"}, usedBy: []string{"redis"}},
	{flag: "write-timeout", keys: []string{"write_timeout"}, usedBy: []string{"redis"}},
	{flag: "retry-max-attempts", keys: []string{"retry_max_attempts"}, usedBy: []string{"redis"}},
	{flag: "retry-base-delay", keys: []string{"retry_base_delay"}, usedBy: []string{"redis"}},
	{flag: "retry-max-delay", keys: []string{"retry_max_delay"}, usedBy: []string{"redis"}},
	{flag: "auth-token", keys: []string{"auth_token"}, usedBy: []string{"consul", "etcd", "vault"}},
	{flag: "basic-auth", keys: []string{"basic_auth"}, usedBy: []string{"consul", "etcd"}},
	{flag: "client-cert", keys: []string{"client_cert"}, usedBy: []string{"consul", "etcd", "vault"}},
	{flag: "client-ca-keys", keys: []string{"client_ca_keys", "client_cakeys"}, usedBy: []string{"consul", "etcd", "vault"}},
}

// basicAuthBackends send username and password only when basic auth is
// enabled. settingScope cannot express this: the backends use the settings,
// but only when another setting is on.
var basicAuthBackends = []string{"consul", "etcd"}

// IgnoredSettingWarnings returns one message for each setting supplied for
// cfg.Backend that the backend does not use: the settingScope settings, then
// --username and --password for a backend in basicAuthBackends while
// cfg.BasicAuth is off. supplied reports whether the setting was given under
// the TOML key passed to it. cfg.BasicAuth must be the resolved value.
func IgnoredSettingWarnings(cfg Config, supplied func(key string) bool) []string {
	var warnings []string
	for _, scope := range settingScope {
		if !slices.ContainsFunc(scope.keys, supplied) || slices.Contains(scope.usedBy, cfg.Backend) {
			continue
		}
		warnings = append(warnings, fmt.Sprintf("--%s has no effect with the %s backend", scope.flag, cfg.Backend))
	}
	if cfg.BasicAuth || !slices.Contains(basicAuthBackends, cfg.Backend) {
		return warnings
	}
	for _, key := range []string{"username", "password"} {
		if supplied(key) {
			warnings = append(warnings, fmt.Sprintf("--%s has no effect with the %s backend unless --basic-auth is set", key, cfg.Backend))
		}
	}
	return warnings
}
