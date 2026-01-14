package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/abtreece/confd/pkg/backends"
)

const healthCheckTimeout = 5 * time.Second

// HealthHandler returns HTTP 200 if the process is alive.
// This is a liveness probe - it always returns OK if the process is running.
func HealthHandler(_ backends.StoreClient) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
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
			w.Write([]byte("backend unhealthy: " + err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}
