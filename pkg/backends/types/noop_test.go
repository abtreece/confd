package types_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends/types"
)

func TestNoopCloser_Close(t *testing.T) {
	var c types.NoopCloser
	if err := c.Close(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestDurationMillis_MarshalJSON(t *testing.T) {
	got, err := json.Marshal(types.DurationMillis(1500 * time.Millisecond))
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if string(got) != "1500" {
		t.Fatalf("Marshal = %s, want 1500", got)
	}
}
