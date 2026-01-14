package template

import (
	"errors"
	"fmt"
	"strings"
)

// FailureMode defines how template processing errors are handled.
type FailureMode int

const (
	// FailModeBestEffort continues processing all templates even when errors occur (default).
	FailModeBestEffort FailureMode = iota
	// FailModeFast stops processing at the first error encountered.
	FailModeFast
)

// ParseFailureMode converts a string to a FailureMode.
func ParseFailureMode(s string) (FailureMode, error) {
	switch strings.ToLower(s) {
	case "best-effort":
		return FailModeBestEffort, nil
	case "fail-fast":
		return FailModeFast, nil
	default:
		return FailModeBestEffort, fmt.Errorf("invalid failure mode: %s (must be 'best-effort' or 'fail-fast')", s)
	}
}

// String returns the string representation of the FailureMode.
func (f FailureMode) String() string {
	switch f {
	case FailModeBestEffort:
		return "best-effort"
	case FailModeFast:
		return "fail-fast"
	default:
		return "unknown"
	}
}

// TemplateStatus represents the processing result for a single template.
type TemplateStatus struct {
	Dest    string // Destination path of the template
	Success bool   // Whether processing succeeded
	Error   error  // Error if processing failed
}

// BatchProcessResult represents the outcome of processing multiple templates.
type BatchProcessResult struct {
	Total     int              // Total templates processed
	Succeeded int              // Number of successful templates
	Failed    int              // Number of failed templates
	Statuses  []TemplateStatus // Per-template status
}

// Error returns an aggregated error from all failed templates using errors.Join.
// Returns nil if no templates failed.
func (r *BatchProcessResult) Error() error {
	if r.Failed == 0 {
		return nil
	}

	errs := make([]error, 0, r.Failed)
	for _, status := range r.Statuses {
		if status.Error != nil {
			errs = append(errs, fmt.Errorf("%s: %w", status.Dest, status.Error))
		}
	}

	return errors.Join(errs...)
}
