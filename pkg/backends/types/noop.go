package types

// NoopCloser is an embeddable struct for backends that hold no resources
// requiring explicit cleanup.
type NoopCloser struct{}

func (NoopCloser) Close() error {
	return nil
}
