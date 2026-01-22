package memkv

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
)

// seenPoolCapacity is the initial capacity for maps in seenPool.
// Sized for typical List/ListDir usage where templates iterate over
// hierarchical key structures. A capacity of 64 balances avoiding
// reallocations for common workloads against over-allocating memory.
const seenPoolCapacity = 64

var (
	// ErrNotExist is returned when a key does not exist in the store.
	ErrNotExist = errors.New("key does not exist")
	// ErrNoMatch is returned when no keys match a pattern.
	ErrNoMatch = errors.New("no keys match")

	// seenPool is a pool of maps used by List and ListDir to track unique entries.
	// This reduces allocations during high-frequency template processing.
	seenPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]struct{}, seenPoolCapacity)
		},
	}
)

// KeyError wraps an error with the associated key.
type KeyError struct {
	Key string
	Err error
}

func (e *KeyError) Error() string {
	return fmt.Sprintf("%s: %s", e.Err, e.Key)
}

// Unwrap returns the underlying error for use with errors.Is/As.
func (e *KeyError) Unwrap() error {
	return e.Err
}

// Store is an in-memory key-value store safe for concurrent access.
// It provides hierarchical path-based operations suitable for template rendering.
type Store struct {
	FuncMap map[string]interface{}
	mu      sync.RWMutex
	m       map[string]KVPair
}

// New creates and initializes a new Store with template functions pre-registered.
func New() *Store {
	s := &Store{m: make(map[string]KVPair)}
	s.FuncMap = map[string]interface{}{
		"exists": s.Exists,
		"ls":     s.List,
		"lsdir":  s.ListDir,
		"get":    s.Get,
		"gets":   s.GetAll,
		"getv":   s.GetValue,
		"getvs":  s.GetAllValues,
	}
	return s
}

// Del deletes the KVPair associated with key.
func (s *Store) Del(key string) {
	s.mu.Lock()
	delete(s.m, key)
	s.mu.Unlock()
}

// Exists checks for the existence of key in the store.
func (s *Store) Exists(key string) bool {
	s.mu.RLock()
	_, ok := s.m[key]
	s.mu.RUnlock()
	return ok
}

// Get returns the KVPair associated with key.
// If the key does not exist, it returns an error wrapping ErrNotExist.
func (s *Store) Get(key string) (KVPair, error) {
	s.mu.RLock()
	kv, ok := s.m[key]
	s.mu.RUnlock()
	if !ok {
		return KVPair{}, &KeyError{Key: key, Err: ErrNotExist}
	}
	return kv, nil
}

// GetValue returns the value associated with key.
// If the key does not exist and a default is provided, the default is returned.
// Otherwise, it returns an error wrapping ErrNotExist.
func (s *Store) GetValue(key string, defaultValue ...string) (string, error) {
	s.mu.RLock()
	kv, ok := s.m[key]
	s.mu.RUnlock()
	if !ok {
		if len(defaultValue) > 0 {
			return defaultValue[0], nil
		}
		return "", &KeyError{Key: key, Err: ErrNotExist}
	}
	return kv.Value, nil
}

// GetAll returns all KVPairs with keys matching the given pattern.
// The pattern syntax is the same as path.Match.
// Returns an empty slice (not an error) if no keys match.
func (s *Store) GetAll(pattern string) (KVPairs, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result KVPairs
	for _, kv := range s.m {
		matched, err := path.Match(pattern, kv.Key)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if matched {
			result = append(result, kv)
		}
	}
	sort.Sort(result)
	return result, nil
}

// GetAllValues returns all values for keys matching the given pattern.
// The pattern syntax is the same as path.Match.
// Returns an empty slice (not an error) if no keys match.
func (s *Store) GetAllValues(pattern string) ([]string, error) {
	pairs, err := s.GetAll(pattern)
	if err != nil {
		return nil, err
	}
	values := make([]string, len(pairs))
	for i, kv := range pairs {
		values[i] = kv.Value
	}
	sort.Strings(values)
	return values, nil
}

// List returns all entry names directly under the given path.
// This includes both leaf keys and intermediate path components.
func (s *Store) List(filePath string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := seenPool.Get().(map[string]struct{})
	defer func() {
		clear(seen)
		seenPool.Put(seen)
	}()

	prefix := pathToTerms(filePath)

	for _, kv := range s.m {
		if kv.Key == filePath {
			seen[path.Base(kv.Key)] = struct{}{}
			continue
		}
		target := pathToTerms(path.Dir(kv.Key))
		if hasPrefixTerms(prefix, target) {
			stripped := stripKey(kv.Key, filePath)
			if idx := strings.Index(stripped, "/"); idx >= 0 {
				seen[stripped[:idx]] = struct{}{}
			} else {
				seen[stripped] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(seen))
	for k := range seen {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// ListDir returns all directory names directly under the given path.
// Unlike List, this only returns intermediate path components, not leaf keys.
func (s *Store) ListDir(filePath string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := seenPool.Get().(map[string]struct{})
	defer func() {
		clear(seen)
		seenPool.Put(seen)
	}()

	prefix := pathToTerms(filePath)

	for _, kv := range s.m {
		if !strings.HasPrefix(kv.Key, filePath) {
			continue
		}
		items := pathToTerms(path.Dir(kv.Key))
		if hasPrefixTerms(prefix, items) && len(items) > len(prefix) {
			seen[items[len(prefix)]] = struct{}{}
		}
	}

	result := make([]string, 0, len(seen))
	for k := range seen {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// Set stores a key-value pair in the store.
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	s.m[key] = KVPair{Key: key, Value: value}
	s.mu.Unlock()
}

// Purge removes all entries from the store.
func (s *Store) Purge() {
	s.mu.Lock()
	clear(s.m)
	s.mu.Unlock()
}

// stripKey removes the prefix from key, along with any leading slash.
func stripKey(key, prefix string) string {
	return strings.TrimPrefix(strings.TrimPrefix(key, prefix), "/")
}

// pathToTerms splits a path into its component parts.
func pathToTerms(filePath string) []string {
	return strings.Split(path.Clean(filePath), "/")
}

// hasPrefixTerms reports whether test starts with all terms in prefix.
func hasPrefixTerms(prefix, test []string) bool {
	if len(test) < len(prefix) {
		return false
	}
	for i, term := range prefix {
		if test[i] != term {
			return false
		}
	}
	return true
}
