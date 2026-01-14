package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/abtreece/confd/pkg/backends"
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
