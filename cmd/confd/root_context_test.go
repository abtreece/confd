//go:build !windows

package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
)

// guardSignal keeps SIGTERM from killing the test process for the duration of
// the test, independently of the handling under test.
func guardSignal(t *testing.T) chan os.Signal {
	t.Helper()
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, syscall.SIGTERM)
	t.Cleanup(func() { signal.Stop(guard) })
	return guard
}

func sendSIGTERM(t *testing.T) {
	t.Helper()
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
}

func TestRootContext_CancelledBySignal(t *testing.T) {
	guardSignal(t)
	ctx, cancel, endStartup := newRootContext()
	t.Cleanup(endStartup)
	t.Cleanup(cancel)

	if err := ctx.Err(); err != nil {
		t.Fatalf("root context cancelled before any signal: %v", err)
	}

	sendSIGTERM(t)

	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("root context was not cancelled within 5s of SIGTERM")
	}

	cause := context.Cause(ctx)
	if !errors.Is(cause, context.Canceled) {
		t.Errorf("cause = %v, want it to wrap context.Canceled", cause)
	}
	if !strings.Contains(cause.Error(), "startup interrupted") || !strings.Contains(cause.Error(), syscall.SIGTERM.String()) {
		t.Errorf("cause = %q, want it to say startup was interrupted by %v", cause, syscall.SIGTERM)
	}
}

// closeRecordingClient is a store client that reports when it is closed.
type closeRecordingClient struct {
	closed chan struct{}
}

func (c *closeRecordingClient) GetValues(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func (c *closeRecordingClient) HealthCheck(context.Context) error { return nil }

func (c *closeRecordingClient) Close() error {
	close(c.closed)
	return nil
}

func TestAwaitStoreClient_ReturnsConstructorResult(t *testing.T) {
	want := &closeRecordingClient{closed: make(chan struct{})}
	got, err := awaitStoreClient(context.Background(), func() (backends.StoreClient, error) {
		return want, nil
	})
	if err != nil {
		t.Fatalf("awaitStoreClient error = %v, want nil", err)
	}
	if got != backends.StoreClient(want) {
		t.Errorf("awaitStoreClient returned a different client than the constructor produced")
	}

	wantErr := errors.New("backend unreachable")
	_, err = awaitStoreClient(context.Background(), func() (backends.StoreClient, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("awaitStoreClient error = %v, want %v", err, wantErr)
	}
}

// A constructor that ignores the context must not delay the return, and a
// client it produces after being abandoned must be closed.
func TestAwaitStoreClient_AbandonsHungConstructorOnSignal(t *testing.T) {
	guardSignal(t)
	ctx, cancel, endStartup := newRootContext()
	t.Cleanup(endStartup)
	t.Cleanup(cancel)

	release := make(chan struct{})
	lateClient := &closeRecordingClient{closed: make(chan struct{})}
	type awaitResult struct {
		client backends.StoreClient
		err    error
	}
	returned := make(chan awaitResult, 1)
	go func() {
		client, err := awaitStoreClient(ctx, func() (backends.StoreClient, error) {
			<-release
			return lateClient, nil
		})
		returned <- awaitResult{client: client, err: err}
	}()

	sendSIGTERM(t)

	select {
	case result := <-returned:
		if result.client != nil {
			t.Errorf("client = %v, want nil when startup is interrupted", result.client)
		}
		if result.err == nil || !strings.Contains(result.err.Error(), "startup interrupted") {
			t.Errorf("err = %v, want a startup interrupted error", result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("awaitStoreClient still waiting on a hung constructor 5s after SIGTERM")
	}

	close(release)
	select {
	case <-lateClient.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("client produced by the abandoned constructor was not closed")
	}
}

func TestRootContext_NotCancelledAfterStartupEnds(t *testing.T) {
	guard := guardSignal(t)
	ctx, cancel, endStartup := newRootContext()
	t.Cleanup(cancel)

	endStartup()
	endStartup() // safe to call more than once

	sendSIGTERM(t)
	select {
	case <-guard:
	case <-time.After(5 * time.Second):
		t.Fatal("SIGTERM was not delivered within 5s")
	}

	select {
	case <-ctx.Done():
		t.Fatalf("root context cancelled by a signal after startup ended: %v", context.Cause(ctx))
	case <-time.After(100 * time.Millisecond):
	}

	cancel()
	if cause := context.Cause(ctx); !errors.Is(cause, context.Canceled) {
		t.Errorf("cause after cancel = %v, want context.Canceled", cause)
	}
}
