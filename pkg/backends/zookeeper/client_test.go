package zookeeper

import (
	"testing"
)

func TestWatchPrefix_InitialCall(t *testing.T) {
	// Create a client with nil connection - WatchPrefix with waitIndex=0
	// should return immediately without using the connection
	client := &Client{client: nil}

	// waitIndex 0 should return immediately with 1
	index, err := client.WatchPrefix("/app", []string{"/app/key"}, 0, nil)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 1 {
		t.Errorf("WatchPrefix() index = %d, want 1", index)
	}
}

// Note: Full GetValues and WatchPrefix tests require a running Zookeeper instance.
// These are covered by integration tests in .github/workflows/integration-tests.yml
//
// Testing the nodeWalk function and GetValues would require mocking the zk.Conn
// interface, which is complex due to the number of methods involved.
// Consider using docker-based integration tests for comprehensive coverage.
