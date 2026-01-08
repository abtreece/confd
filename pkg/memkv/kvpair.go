// Package memkv implements an in-memory key/value store with hierarchical
// path-based operations designed for template rendering.
//
// Originally from github.com/kelseyhightower/memkv, internalized for maintenance.
package memkv

// KVPair represents a key-value pair stored in the Store.
type KVPair struct {
	Key   string
	Value string
}

// KVPairs is a slice of KVPair that implements sort.Interface for sorting by key.
type KVPairs []KVPair

// Len returns the number of pairs.
func (ks KVPairs) Len() int {
	return len(ks)
}

// Less reports whether the pair at index i should sort before the pair at index j.
func (ks KVPairs) Less(i, j int) bool {
	return ks[i].Key < ks[j].Key
}

// Swap swaps the pairs at indices i and j.
func (ks KVPairs) Swap(i, j int) {
	ks[i], ks[j] = ks[j], ks[i]
}
