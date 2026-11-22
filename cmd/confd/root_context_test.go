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
