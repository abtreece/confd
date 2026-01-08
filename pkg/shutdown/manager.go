package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/abtreece/confd/pkg/log"
)

// Manager handles graceful shutdown of the application
type Manager struct {
	timeout       time.Duration
	cleanupScript string
	stopChan      chan bool
	doneChan      chan bool
	errChan       chan error
	mu            sync.Mutex
	started       bool
	shutdownFuncs []func() error
	signalChan    chan os.Signal
	quitChan      chan os.Signal
}

// Config contains configuration for the shutdown manager
type Config struct {
	Timeout       time.Duration
	CleanupScript string
	StopChan      chan bool
	DoneChan      chan bool
	ErrChan       chan error
}

// New creates a new shutdown manager
func New(cfg Config) *Manager {
	return &Manager{
		timeout:       cfg.Timeout,
		cleanupScript: cfg.CleanupScript,
		stopChan:      cfg.StopChan,
		doneChan:      cfg.DoneChan,
		errChan:       cfg.ErrChan,
		shutdownFuncs: make([]func() error, 0),
		signalChan:    make(chan os.Signal, 1),
		quitChan:      make(chan os.Signal, 1),
	}
}

// RegisterCleanup registers a cleanup function to be called during shutdown
func (m *Manager) RegisterCleanup(fn func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownFuncs = append(m.shutdownFuncs, fn)
}

// Start begins listening for shutdown signals
func (m *Manager) Start() error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("shutdown manager already started")
	}
	m.started = true
	m.mu.Unlock()

	// SIGTERM and SIGINT: graceful shutdown
	signal.Notify(m.signalChan, syscall.SIGTERM, syscall.SIGINT)

	// SIGQUIT: immediate shutdown
	signal.Notify(m.quitChan, syscall.SIGQUIT)

	go m.handleSignals()

	return nil
}

// handleSignals processes incoming signals
func (m *Manager) handleSignals() {
	select {
	case s := <-m.signalChan:
		log.Info("Received signal %v, initiating graceful shutdown (timeout: %v)", s, m.timeout)
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()

		if err := m.Shutdown(ctx); err != nil {
			log.Error("Shutdown error: %v", err)
			if m.errChan != nil {
				select {
				case m.errChan <- err:
				default:
				}
			}
		}

	case s := <-m.quitChan:
		log.Warning("Received signal %v, forcing immediate shutdown", s)
		m.forceShutdown()
	}
}

// Shutdown performs a graceful shutdown with the given context
func (m *Manager) Shutdown(ctx context.Context) error {
	log.Info("=== Graceful Shutdown Initiated ===")
	log.Info("Step 1/4: Stopping new events")

	// Signal processors to stop (check if already closed)
	select {
	case <-m.stopChan:
		// Already closed
	default:
		close(m.stopChan)
	}

	log.Info("Step 2/4: Waiting for in-flight operations (timeout: %v)", m.timeout)

	// Wait for processor to finish or timeout
	done := make(chan struct{})
	go func() {
		<-m.doneChan
		close(done)
	}()

	select {
	case <-done:
		log.Info("All operations completed successfully")
	case <-ctx.Done():
		log.Warning("Shutdown timeout exceeded after %v, forcing termination", m.timeout)
		return fmt.Errorf("shutdown timeout exceeded")
	}

	log.Info("Step 3/4: Executing cleanup hooks")
	if err := m.executeCleanup(); err != nil {
		log.Error("Cleanup failed: %v", err)
		return err
	}

	log.Info("Step 4/4: Cleanup complete")
	log.Info("=== Graceful Shutdown Complete ===")

	return nil
}

// forceShutdown immediately shuts down without waiting
func (m *Manager) forceShutdown() {
	log.Warning("=== Forced Immediate Shutdown ===")

	// Close stopChan if not already closed
	select {
	case <-m.stopChan:
		// Already closed
	default:
		close(m.stopChan)
	}

	// Don't wait for doneChan, just exit
	os.Exit(1)
}

// executeCleanup runs all registered cleanup functions and the cleanup script
func (m *Manager) executeCleanup() error {
	m.mu.Lock()
	funcs := m.shutdownFuncs
	m.mu.Unlock()

	// Execute registered cleanup functions
	for i, fn := range funcs {
		log.Debug("Executing cleanup function %d/%d", i+1, len(funcs))
		if err := fn(); err != nil {
			return fmt.Errorf("cleanup function %d failed: %w", i+1, err)
		}
	}

	// Execute cleanup script if provided
	if m.cleanupScript != "" {
		log.Info("Executing cleanup script: %s", m.cleanupScript)
		if err := m.runCleanupScript(); err != nil {
			return fmt.Errorf("cleanup script failed: %w", err)
		}
	}

	return nil
}

// runCleanupScript executes the configured cleanup script
func (m *Manager) runCleanupScript() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", m.cleanupScript)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("cleanup script timed out after 30 seconds")
		}
		return err
	}

	return nil
}
