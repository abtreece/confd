package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/backends/types"
	"github.com/abtreece/confd/pkg/log"
)

const healthCheckTimeout = 5 * time.Second

// HealthHandler returns HTTP 200 if the process is alive.
// This is a liveness probe - it always returns OK if the process is running.
func HealthHandler(_ backends.StoreClient) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			log.Error("Failed to write health response: %v", err)
		}
	}
}

// ReadyHandler returns HTTP 200 if the backend is reachable, 503 otherwise.
// This is a readiness probe - it checks backend connectivity.
func ReadyHandler(client backends.StoreClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
		defer cancel()

		if err := client.HealthCheck(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, writeErr := w.Write([]byte("backend unhealthy: " + err.Error())); writeErr != nil {
				log.Error("Failed to write ready response: %v", writeErr)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			log.Error("Failed to write ready response: %v", err)
		}
	}
}

// ReadyDetailedHandler returns detailed health check information as JSON.
// Returns HTTP 200 with detailed health info if backend is healthy, 503 otherwise.
func ReadyDetailedHandler(client backends.StoreClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")

		// Try to get detailed health info if available
		var result *types.HealthResult
		var err error

		if detailedChecker, ok := client.(types.DetailedHealthChecker); ok {
			result, err = detailedChecker.HealthCheckDetailed(ctx)
		} else {
			// Fallback to basic health check
			basicErr := client.HealthCheck(ctx)
			result = &types.HealthResult{
				Healthy:   basicErr == nil,
				Message:   "Backend does not support detailed health checks",
				Duration:  0,
				CheckedAt: time.Now(),
				Details:   map[string]string{},
			}
			if basicErr != nil {
				result.Message = basicErr.Error()
				result.Details["error"] = basicErr.Error()
				err = basicErr
			}
		}

		// Set HTTP status based on health
		if err != nil || !result.Healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		// Marshal and write JSON response
		if jsonErr := json.NewEncoder(w).Encode(result); jsonErr != nil {
			log.Error("Failed to write detailed health response: %v", jsonErr)
		}
	}
}
