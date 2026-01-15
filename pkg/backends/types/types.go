package types

import (
	"context"
	"encoding/json"
	"time"
)

// DurationMillis is a time.Duration that marshals to JSON as milliseconds.
type DurationMillis time.Duration

// MarshalJSON implements json.Marshaler for DurationMillis.
func (d DurationMillis) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).Milliseconds())
}

// HealthResult contains detailed health check information.
type HealthResult struct {
	Healthy   bool              `json:"healthy"`
	Message   string            `json:"message"`
	Duration  DurationMillis    `json:"duration_ms"`
	Details   map[string]string `json:"details,omitempty"`
	CheckedAt time.Time         `json:"checked_at"`
}

// DetailedHealthChecker is an optional interface for backends that provide
// extended health diagnostics beyond the basic HealthCheck.
type DetailedHealthChecker interface {
	HealthCheckDetailed(ctx context.Context) (*HealthResult, error)
}
