package types

import "context"

// NoopWatcher is an embeddable struct for polling-only backends that do not
// support watch mode. It blocks until the context is canceled or the stop
// channel is closed.
type NoopWatcher struct{}

func (NoopWatcher) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	select {
	case <-ctx.Done():
		return waitIndex, ctx.Err()
	case <-stopChan:
		return waitIndex, nil
	}
}

// NoopCloser is an embeddable struct for backends that hold no resources
// requiring explicit cleanup.
type NoopCloser struct{}

func (NoopCloser) Close() error {
	return nil
}
