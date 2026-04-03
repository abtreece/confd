package service

import (
	"context"
	"net/http"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
)

// ShutdownManager coordinates graceful shutdown of confd services.
type ShutdownManager struct {
	timeout       time.Duration
	metricsServer *http.Server
	storeClient   backends.StoreClient
}

// NewShutdownManager creates a new shutdown manager.
func NewShutdownManager(timeout time.Duration, metricsServer *http.Server, storeClient backends.StoreClient) *ShutdownManager {
	return &ShutdownManager{
		timeout:       timeout,
		metricsServer: metricsServer,
		storeClient:   storeClient,
	}
}

// Shutdown performs graceful shutdown in two phases:
// 1. Shutdown metrics server gracefully
// 2. Close backend connections
//
// In-flight check/reload commands do not need explicit tracking here: all
// three processor types (interval, watch, batch-watch) complete any
// in-progress process() call before closing doneChan, so by the time
// Shutdown is called the processor has already drained.
func (s *ShutdownManager) Shutdown(ctx context.Context) error {
	log.Info("Starting graceful shutdown (timeout: %v)", s.timeout)

	// Create a context with timeout for the entire shutdown process
	shutdownCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Phase 1: Shutdown metrics server
	if s.metricsServer != nil {
		log.Info("Phase 1: Shutting down metrics server")
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

	// Phase 2: Close backend connections
	log.Info("Phase 2: Closing backend connections")
	if s.storeClient != nil {
		if err := s.storeClient.Close(); err != nil {
			// Log but do not return — the process is exiting and the OS will
			// reclaim the socket. Failing shutdown on a close error would mask
			// the fact that everything else completed cleanly.
			log.Warning("Backend connection close error: %v", err)
		} else {
			log.Info("Backend connections closed successfully")
		}
	}

	log.Info("Graceful shutdown completed")
	return nil
}
