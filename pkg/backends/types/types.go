package types

import (
	"context"
	"time"
)

// HealthResult contains detailed health check information.
type HealthResult struct {
	Healthy   bool              `json:"healthy"`
	Message   string            `json:"message"`
	Duration  time.Duration     `json:"duration_ms"`
	Details   map[string]string `json:"details,omitempty"`
	CheckedAt time.Time         `json:"checked_at"`
}

// DetailedHealthChecker is an optional interface for backends that provide
// extended health diagnostics beyond the basic HealthCheck.
type DetailedHealthChecker interface {
	HealthCheckDetailed(ctx context.Context) (*HealthResult, error)
}
