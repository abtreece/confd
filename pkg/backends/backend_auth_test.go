package backends

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// TestNew_ConsulAuthToken checks that Config.AuthToken reaches the consul
// client and is sent as the ACL token.
func TestNew_ConsulAuthToken(t *testing.T) {
	t.Setenv("CONSUL_HTTP_TOKEN", "")
	t.Setenv("CONSUL_HTTP_TOKEN_FILE", "")

	var mu sync.Mutex
	var tokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		tokens = append(tokens, r.Header.Get("X-Consul-Token"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	client, err := New(context.Background(), Config{
		Backend:      "consul",
		BackendNodes: []string{strings.TrimPrefix(server.URL, "http://")},
		Scheme:       "http",
		AuthToken:    "acl-token",
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	if err := client.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(tokens) == 0 {
		t.Fatal("no request reached the server")
	}
	for _, token := range tokens {
		if token != "acl-token" {
			t.Errorf("X-Consul-Token = %q, want %q", token, "acl-token")
		}
	}
}

// TestNew_EtcdAuthToken checks that Config.AuthToken reaches the etcd client.
// The etcd client rejects a token together with basic auth, so that error can
// only come back if the token was passed through.
func TestNew_EtcdAuthToken(t *testing.T) {
	_, err := New(context.Background(), Config{
		Backend:      "etcd",
		BackendNodes: []string{"127.0.0.1:1"},
		BasicAuth:    true,
		Username:     "user",
		Password:     "pass",
		AuthToken:    "jwt-token",
		DialTimeout:  time.Second,
	})
	if !errors.Is(err, clientv3.ErrMutuallyExclusiveCfg) {
		t.Fatalf("New() error = %v, want %v", err, clientv3.ErrMutuallyExclusiveCfg)
	}
}

// TestNew_RedisUsername checks that Config.Username reaches the redis client
// and authenticates as that ACL user.
func TestNew_RedisUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{name: "username sent", username: "app"},
		{name: "no username", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := miniredis.RunT(t)
			server.RequireUserAuth("app", "secret")

			client, err := New(context.Background(), Config{
				Backend:          "redis",
				BackendNodes:     []string{server.Addr()},
				Username:         tt.username,
				Password:         "secret",
				DialTimeout:      time.Second,
				ReadTimeout:      time.Second,
				WriteTimeout:     time.Second,
				RetryMaxAttempts: 1,
				RetryBaseDelay:   time.Millisecond,
				RetryMaxDelay:    time.Millisecond,
			})
			if tt.wantErr {
				if err == nil {
					_ = client.Close()
					t.Fatal("New() succeeded without the ACL username, want authentication failure")
				}
				return
			}
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}
			_ = client.Close()
		})
	}
}
