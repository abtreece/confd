package service

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
)

// ShutdownManager coordinates graceful shutdown of confd services.
type ShutdownManager struct {
	timeout       time.Duration
	metricsServer *http.Server
	storeClient   backends.StoreClient
	inFlightCmds  *sync.WaitGroup
}

// NewShutdownManager creates a new shutdown manager.
func NewShutdownManager(timeout time.Duration, metricsServer *http.Server, storeClient backends.StoreClient, inFlightCmds *sync.WaitGroup) *ShutdownManager {
	return &ShutdownManager{
		timeout:       timeout,
		metricsServer: metricsServer,
		storeClient:   storeClient,
		inFlightCmds:  inFlightCmds,
	}
}

// Shutdown performs graceful shutdown in three phases:
// 1. Wait for in-flight commands to complete (with timeout)
// 2. Shutdown metrics server gracefully
// 3. Close backend connections
func (s *ShutdownManager) Shutdown(ctx context.Context) error {
	log.Info("Starting graceful shutdown (timeout: %v)", s.timeout)

	// Create a context with timeout for the entire shutdown process
	shutdownCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Phase 1: Wait for in-flight commands with timeout
	log.Info("Phase 1: Waiting for in-flight commands to complete")
	cmdsDone := make(chan struct{})
	go func() {
		if s.inFlightCmds != nil {
			s.inFlightCmds.Wait()
		}
		close(cmdsDone)
	}()

	select {
	case <-cmdsDone:
		log.Info("All in-flight commands completed")
	case <-shutdownCtx.Done():
		log.Warning("Shutdown timeout waiting for in-flight commands")
	}

	// Phase 2: Shutdown metrics server
	if s.metricsServer != nil {
		log.Info("Phase 2: Shutting down metrics server")
		// Give metrics server a small timeout (5 seconds or remaining time, whichever is less)
		serverTimeout := 5 * time.Second
		if deadline, ok := shutdownCtx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining < serverTimeout {
				serverTimeout = remaining
			}
		}

		serverCtx, serverCancel := context.WithTimeout(context.Background(), serverTimeout)
		defer serverCancel()

		if err := s.metricsServer.Shutdown(serverCtx); err != nil {
			log.Warning("Metrics server shutdown error: %v", err)
		} else {
			log.Info("Metrics server shut down successfully")
		}
	}

	// Phase 3: Close backend connections
	log.Info("Phase 3: Closing backend connections")
	if s.storeClient != nil {
		if err := s.storeClient.Close(); err != nil {
			log.Warning("Backend connection close error: %v", err)
			return fmt.Errorf("failed to close backend: %w", err)
		}
		log.Info("Backend connections closed successfully")
	}

	log.Info("Graceful shutdown completed")
	return nil
}
