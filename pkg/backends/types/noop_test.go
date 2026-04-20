package types_test

import (
	"context"
	"testing"

	"github.com/abtreece/confd/pkg/backends/types"
)

func TestNoopWatcher_StopChan(t *testing.T) {
	var w types.NoopWatcher
	stopChan := make(chan bool)
	close(stopChan)

	idx, err := w.WatchPrefix(context.Background(), "/prefix", nil, 42, stopChan)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if idx != 42 {
		t.Fatalf("expected waitIndex 42, got %d", idx)
	}
}

func TestNoopWatcher_ContextCanceled(t *testing.T) {
	var w types.NoopWatcher
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stopChan := make(chan bool)
	idx, err := w.WatchPrefix(ctx, "/prefix", nil, 42, stopChan)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if idx != 42 {
		t.Fatalf("expected waitIndex 42, got %d", idx)
	}
}

func TestNoopCloser_Close(t *testing.T) {
	var c types.NoopCloser
	if err := c.Close(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
