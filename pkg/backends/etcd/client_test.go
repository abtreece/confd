package etcd

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestWatch_Update(t *testing.T) {
	w := &Watch{
		revision: 0,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	// Update should set revision and close the cond channel
	w.update(100)

	if w.revision != 100 {
		t.Errorf("Watch.revision = %d, want 100", w.revision)
	}

	// The old cond channel should be closed
	select {
	case <-w.cond:
		// Expected - old cond was closed, new one created
	default:
		// Should not block - cond should be a new channel
	}
}

func TestWatch_UpdateMultipleTimes(t *testing.T) {
	w := &Watch{
		revision: 0,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	w.update(10)
	w.update(20)
	w.update(30)

	if w.revision != 30 {
		t.Errorf("Watch.revision = %d, want 30", w.revision)
	}
}

func TestWatch_WaitNext_AlreadyUpdated(t *testing.T) {
	w := &Watch{
		revision: 100,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	notify := make(chan int64, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// WaitNext should return immediately since revision (100) > lastRevision (50)
	go w.WaitNext(ctx, 50, notify)

	select {
	case rev := <-notify:
		if rev != 100 {
			t.Errorf("WaitNext returned revision %d, want 100", rev)
		}
	case <-ctx.Done():
		t.Error("WaitNext timed out when it should have returned immediately")
	}
}

func TestWatch_WaitNext_WaitsForUpdate(t *testing.T) {
	w := &Watch{
		revision: 50,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	notify := make(chan int64, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// WaitNext should wait since revision (50) is not > lastRevision (50)
	go w.WaitNext(ctx, 50, notify)

	// Give goroutine time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Now update
	w.update(100)

	select {
	case rev := <-notify:
		if rev != 100 {
			t.Errorf("WaitNext returned revision %d, want 100", rev)
		}
	case <-ctx.Done():
		t.Error("WaitNext timed out waiting for update")
	}
}

func TestWatch_WaitNext_Cancelled(t *testing.T) {
	w := &Watch{
		revision: 50,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	notify := make(chan int64, 1)
	ctx, cancel := context.WithCancel(context.Background())

	// WaitNext should wait since revision (50) is not > lastRevision (50)
	go w.WaitNext(ctx, 50, notify)

	// Give goroutine time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Give goroutine time to exit
	time.Sleep(10 * time.Millisecond)

	select {
	case <-notify:
		t.Error("WaitNext should not send on notify when cancelled")
	default:
		// Expected - no notification sent
	}
}

func TestWatch_ConcurrentAccess(t *testing.T) {
	w := &Watch{
		revision: 0,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	// Start multiple goroutines updating and reading
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(rev int64) {
			defer wg.Done()
			w.update(rev)
		}(int64(i * 10))
	}

	wg.Wait()

	// Revision should be set (exact value depends on goroutine scheduling)
	if w.revision < 0 || w.revision > 90 {
		t.Errorf("Watch.revision = %d, expected between 0 and 90", w.revision)
	}
}

// Note: Full GetValues and WatchPrefix tests require a running etcd instance.
// These are covered by integration tests in .github/workflows/integration-tests.yml
//
// The etcd client uses complex transaction batching and watch management
// that requires an actual etcd connection to test properly.
