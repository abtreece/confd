package shutdown

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestShutdown_BasicShutdown(t *testing.T) {
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	mgr := New(Config{
		Timeout:  5 * time.Second,
		StopChan: stopChan,
		DoneChan: doneChan,
		ErrChan:  errChan,
	})

	// Simulate processor closing doneChan after stopChan is closed
	go func() {
		<-stopChan
		close(doneChan)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mgr.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestShutdown_Timeout(t *testing.T) {
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	mgr := New(Config{
		Timeout:  1 * time.Second,
		StopChan: stopChan,
		DoneChan: doneChan,
		ErrChan:  errChan,
	})

	// Simulate processor that never finishes
	go func() {
		<-stopChan
		// Don't close doneChan - simulate hanging processor
		time.Sleep(5 * time.Second)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mgr.Shutdown(ctx)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
	if err.Error() != "shutdown timeout exceeded" {
		t.Errorf("Expected 'shutdown timeout exceeded', got: %v", err)
	}
}

func TestShutdown_CleanupFunctions(t *testing.T) {
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	mgr := New(Config{
		Timeout:  5 * time.Second,
		StopChan: stopChan,
		DoneChan: doneChan,
		ErrChan:  errChan,
	})

	// Track cleanup execution
	var cleanup1Called, cleanup2Called int32

	mgr.RegisterCleanup(func() error {
		atomic.StoreInt32(&cleanup1Called, 1)
		return nil
	})

	mgr.RegisterCleanup(func() error {
		atomic.StoreInt32(&cleanup2Called, 1)
		return nil
	})

	// Simulate processor
	go func() {
		<-stopChan
		close(doneChan)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mgr.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if atomic.LoadInt32(&cleanup1Called) != 1 {
		t.Error("Cleanup function 1 was not called")
	}
	if atomic.LoadInt32(&cleanup2Called) != 1 {
		t.Error("Cleanup function 2 was not called")
	}
}

func TestShutdown_CleanupFunctionError(t *testing.T) {
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	mgr := New(Config{
		Timeout:  5 * time.Second,
		StopChan: stopChan,
		DoneChan: doneChan,
		ErrChan:  errChan,
	})

	// Register cleanup that returns error
	expectedErr := fmt.Errorf("cleanup failed")
	mgr.RegisterCleanup(func() error {
		return expectedErr
	})

	// Simulate processor
	go func() {
		<-stopChan
		close(doneChan)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mgr.Shutdown(ctx)
	if err == nil {
		t.Error("Expected cleanup error, got nil")
	}
}

func TestShutdown_CleanupScript(t *testing.T) {
	// Create temporary cleanup script
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "cleanup.sh")
	markerPath := filepath.Join(tmpDir, "cleanup_ran")

	script := fmt.Sprintf("#!/bin/sh\ntouch %s\n", markerPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	mgr := New(Config{
		Timeout:       5 * time.Second,
		CleanupScript: scriptPath,
		StopChan:      stopChan,
		DoneChan:      doneChan,
		ErrChan:       errChan,
	})

	// Simulate processor
	go func() {
		<-stopChan
		close(doneChan)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mgr.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify script ran
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("Cleanup script did not run - marker file not created")
	}
}

func TestShutdown_MultipleStarts(t *testing.T) {
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	mgr := New(Config{
		Timeout:  5 * time.Second,
		StopChan: stopChan,
		DoneChan: doneChan,
		ErrChan:  errChan,
	})

	err := mgr.Start()
	if err != nil {
		t.Errorf("First Start() failed: %v", err)
	}

	err = mgr.Start()
	if err == nil {
		t.Error("Second Start() should return error")
	}
}

func TestShutdown_StopChanAlreadyClosed(t *testing.T) {
	stopChan := make(chan bool)
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	mgr := New(Config{
		Timeout:  5 * time.Second,
		StopChan: stopChan,
		DoneChan: doneChan,
		ErrChan:  errChan,
	})

	// Close stopChan before shutdown
	close(stopChan)

	// Simulate processor
	go func() {
		close(doneChan)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should not panic
	err := mgr.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}
