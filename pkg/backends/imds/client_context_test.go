package imds

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// constructorDialTimeout is deliberately far longer than any test below should
// take, so a prompt return can only come from the caller's context.
const constructorDialTimeout = 30 * time.Second

// startHangingIMDS points IMDS_ENDPOINT at a server that accepts requests and
// never responds, and isolates the AWS SDK from the developer's environment.
// It returns a channel that receives once per request reaching the server.
func startHangingIMDS(t *testing.T) <-chan struct{} {
	t.Helper()

	requests := make(chan struct{}, 16)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case requests <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	missing := filepath.Join(t.TempDir(), "missing")
	t.Setenv("IMDS_ENDPOINT", server.URL)
	t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
	t.Setenv("AWS_CONFIG_FILE", missing)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", missing)
	t.Setenv("AWS_PROFILE", "")

	return requests
}

func TestNew_ContextCancelled(t *testing.T) {
	startHangingIMDS(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	client, err := New(ctx, time.Minute, constructorDialTimeout)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("New() with cancelled context returned client %v, want error", client)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("New() error = %v, want it to wrap context.Canceled", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("New() took %v with an already-cancelled context", elapsed)
	}
}

func TestNew_ContextCancelDuringValidation(t *testing.T) {
	requests := startHangingIMDS(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel only once the constructor is blocked on the unresponsive server.
	go func() {
		select {
		case <-requests:
			cancel()
		case <-ctx.Done():
		}
	}()

	start := time.Now()
	client, err := New(ctx, time.Minute, constructorDialTimeout)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("New() returned client %v after cancellation, want error", client)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("New() error = %v, want it to wrap context.Canceled", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("New() took %v after cancellation", elapsed)
	}
}
