package service

import (
	"sync"
	"testing"
	"time"
)

func TestNewReloadManager(t *testing.T) {
	mgr := NewReloadManager()
	if mgr == nil {
		t.Fatal("NewReloadManager returned nil")
	}
	if mgr.subscribers == nil {
		t.Fatal("subscribers was not initialized")
	}
	if len(mgr.subscribers) != 0 {
		t.Fatalf("subscribers len = %d, want 0", len(mgr.subscribers))
	}
}

func TestReloadManager_Subscribe(t *testing.T) {
	mgr := NewReloadManager()

	ch := mgr.Subscribe()
	if ch == nil {
		t.Fatal("Subscribe returned nil channel")
	}
	if cap(ch) != 1 {
		t.Fatalf("subscriber channel cap = %d, want 1", cap(ch))
	}
	if len(mgr.subscribers) != 1 {
		t.Fatalf("subscribers len = %d, want 1", len(mgr.subscribers))
	}
}

func TestReloadManager_TriggerReloadNotifiesSubscribers(t *testing.T) {
	mgr := NewReloadManager()
	ch1 := mgr.Subscribe()
	ch2 := mgr.Subscribe()

	mgr.TriggerReload()

	assertReloadSignal(t, ch1)
	assertReloadSignal(t, ch2)
}

func TestReloadManager_TriggerReloadDoesNotBlockWhenSignalPending(t *testing.T) {
	mgr := NewReloadManager()
	ch := mgr.Subscribe()

	mgr.TriggerReload()

	done := make(chan struct{})
	go func() {
		defer close(done)
		mgr.TriggerReload()
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("TriggerReload blocked with a pending subscriber signal")
	}

	assertReloadSignal(t, ch)
	select {
	case <-ch:
		t.Fatal("TriggerReload queued duplicate signal while one was pending")
	default:
	}
}

func TestReloadManager_TriggerReloadWithNoSubscribers(t *testing.T) {
	mgr := NewReloadManager()

	done := make(chan struct{})
	go func() {
		defer close(done)
		mgr.TriggerReload()
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("TriggerReload blocked with no subscribers")
	}
}

func TestReloadManager_ConcurrentSubscribeAndTrigger(t *testing.T) {
	mgr := NewReloadManager()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.Subscribe()
		}()
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.TriggerReload()
		}()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent subscribe/trigger did not complete")
	}
}

func assertReloadSignal(t *testing.T, ch chan struct{}) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reload signal")
	}
}
