package backends

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestNew_RedisPassword(t *testing.T) {
	const serverPassword = "secret"

	tests := []struct {
		name      string
		password  string
		clientKey string
		wantErr   bool
	}{
		{name: "password set", password: serverPassword},
		{name: "client key only", clientKey: serverPassword},
		{name: "password wins over client key", password: serverPassword, clientKey: "wrong"},
		{name: "wrong password is not rescued by client key", password: "wrong", clientKey: serverPassword, wantErr: true},
		{name: "neither set", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := miniredis.RunT(t)
			server.RequireAuth(serverPassword)
			if err := server.Set("/app/key", "value"); err != nil {
				t.Fatalf("seed miniredis: %v", err)
			}

			client, err := New(context.Background(), Config{
				Backend:          "redis",
				BackendNodes:     []string{server.Addr()},
				Password:         tt.password,
				ClientKey:        tt.clientKey,
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
					t.Fatal("New() succeeded, want authentication failure")
				}
				return
			}
			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}
			defer client.Close()

			values, err := client.GetValues(context.Background(), []string{"/app/key"})
			if err != nil {
				t.Fatalf("GetValues() unexpected error: %v", err)
			}
			if values["/app/key"] != "value" {
				t.Errorf("GetValues()[/app/key] = %q, want %q", values["/app/key"], "value")
			}
		})
	}
}
