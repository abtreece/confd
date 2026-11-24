package backends

import (
	"slices"
	"testing"
)

// suppliedKeys returns a supplied function that reports the given keys.
func suppliedKeys(keys ...string) func(string) bool {
	return func(key string) bool { return slices.Contains(keys, key) }
}

var allBackends = []string{"acm", "consul", "dynamodb", "env", "etcd", "file", "imds", "redis", "secretsmanager", "ssm", "vault", "zookeeper"}

func TestIgnoredSettingWarningsScope(t *testing.T) {
	for _, scope := range settingScope {
		for _, key := range scope.keys {
			for _, backend := range allBackends {
				t.Run(backend+"/"+key, func(t *testing.T) {
					// Basic auth on, so no credential warning mixes in.
					cfg := Config{Backend: backend, BasicAuth: true}
					got := IgnoredSettingWarnings(cfg, suppliedKeys(key))

					var want []string
					if !slices.Contains(scope.usedBy, backend) {
						want = []string{"--" + scope.flag + " has no effect with the " + backend + " backend"}
					}
					if !slices.Equal(got, want) {
						t.Errorf("warnings = %q, want %q", got, want)
					}
				})
			}
		}
	}
}

func TestIgnoredSettingWarningsOneMessagePerSetting(t *testing.T) {
	got := IgnoredSettingWarnings(Config{Backend: "redis"}, suppliedKeys("client_ca_keys", "client_cakeys"))
	want := []string{"--client-ca-keys has no effect with the redis backend"}
	if !slices.Equal(got, want) {
		t.Errorf("warnings = %q, want %q", got, want)
	}
}

func TestIgnoredSettingWarningsBasicAuthCredentials(t *testing.T) {
	warning := func(key, backend string) string {
		return "--" + key + " has no effect with the " + backend + " backend unless --basic-auth is set"
	}
	tests := []struct {
		name string
		cfg  Config
		keys []string
		want []string
	}{
		{name: "consul username", cfg: Config{Backend: "consul"}, keys: []string{"username"}, want: []string{warning("username", "consul")}},
		{name: "etcd password", cfg: Config{Backend: "etcd"}, keys: []string{"password"}, want: []string{warning("password", "etcd")}},
		{name: "both, username first", cfg: Config{Backend: "consul"}, keys: []string{"password", "username"}, want: []string{warning("username", "consul"), warning("password", "consul")}},
		{name: "basic auth on", cfg: Config{Backend: "etcd", BasicAuth: true}, keys: []string{"username", "password"}},
		{name: "vault", cfg: Config{Backend: "vault"}, keys: []string{"username", "password"}},
		{name: "redis", cfg: Config{Backend: "redis"}, keys: []string{"username", "password"}},
		{
			name: "after scope warnings",
			cfg:  Config{Backend: "consul"},
			keys: []string{"username", "dial_timeout"},
			want: []string{"--dial-timeout has no effect with the consul backend", warning("username", "consul")},
		},
		{name: "nothing supplied", cfg: Config{Backend: "consul"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IgnoredSettingWarnings(tt.cfg, suppliedKeys(tt.keys...))
			if !slices.Equal(got, tt.want) {
				t.Errorf("warnings = %q, want %q", got, tt.want)
			}
		})
	}
}
