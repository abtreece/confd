package memkv

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.FuncMap == nil {
		t.Error("FuncMap is nil")
	}
	if s.m == nil {
		t.Error("internal map is nil")
	}

	// Check that all expected functions are registered
	expectedFuncs := []string{"exists", "ls", "lsdir", "get", "gets", "getv", "getvs"}
	for _, name := range expectedFuncs {
		if _, ok := s.FuncMap[name]; !ok {
			t.Errorf("FuncMap missing function %q", name)
		}
	}
}

func TestStore_SetAndGet(t *testing.T) {
	s := New()
	s.Set("/app/config/key1", "value1")

	kv, err := s.Get("/app/config/key1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if kv.Key != "/app/config/key1" {
		t.Errorf("Key = %q, want %q", kv.Key, "/app/config/key1")
	}
	if kv.Value != "value1" {
		t.Errorf("Value = %q, want %q", kv.Value, "value1")
	}
}

func TestStore_Get_NotExist(t *testing.T) {
	s := New()
	_, err := s.Get("/nonexistent")
	if err == nil {
		t.Fatal("Get() expected error for nonexistent key")
	}

	var keyErr *KeyError
	if !errors.As(err, &keyErr) {
		t.Fatalf("error should be *KeyError, got %T", err)
	}
	if !errors.Is(keyErr, ErrNotExist) {
		t.Errorf("error should wrap ErrNotExist")
	}
	if keyErr.Key != "/nonexistent" {
		t.Errorf("KeyError.Key = %q, want %q", keyErr.Key, "/nonexistent")
	}
}

func TestStore_GetValue(t *testing.T) {
	s := New()
	s.Set("/key", "value")

	// Existing key
	v, err := s.GetValue("/key")
	if err != nil {
		t.Fatalf("GetValue() error: %v", err)
	}
	if v != "value" {
		t.Errorf("GetValue() = %q, want %q", v, "value")
	}

	// Non-existent key without default
	_, err = s.GetValue("/nonexistent")
	if err == nil {
		t.Error("GetValue() expected error for nonexistent key")
	}

	// Non-existent key with default
	v, err = s.GetValue("/nonexistent", "default")
	if err != nil {
		t.Fatalf("GetValue() with default error: %v", err)
	}
	if v != "default" {
		t.Errorf("GetValue() = %q, want %q", v, "default")
	}
}

func TestStore_Exists(t *testing.T) {
	s := New()
	s.Set("/exists", "value")

	if !s.Exists("/exists") {
		t.Error("Exists() = false for existing key")
	}
	if s.Exists("/nonexistent") {
		t.Error("Exists() = true for nonexistent key")
	}
}

func TestStore_Del(t *testing.T) {
	s := New()
	s.Set("/key", "value")

	if !s.Exists("/key") {
		t.Fatal("key should exist before deletion")
	}

	s.Del("/key")

	if s.Exists("/key") {
		t.Error("key should not exist after deletion")
	}
}

func TestStore_Purge(t *testing.T) {
	s := New()
	s.Set("/key1", "value1")
	s.Set("/key2", "value2")
	s.Set("/key3", "value3")

	s.Purge()

	if s.Exists("/key1") || s.Exists("/key2") || s.Exists("/key3") {
		t.Error("keys should not exist after Purge()")
	}
}

func TestStore_GetAll(t *testing.T) {
	s := New()
	s.Set("/app/config/db/host", "localhost")
	s.Set("/app/config/db/port", "5432")
	s.Set("/app/config/cache/host", "redis")
	s.Set("/other/key", "value")

	// Pattern matching
	pairs, err := s.GetAll("/app/config/db/*")
	if err != nil {
		t.Fatalf("GetAll() error: %v", err)
	}
	if len(pairs) != 2 {
		t.Errorf("GetAll() returned %d pairs, want 2", len(pairs))
	}

	// Results should be sorted by key
	if len(pairs) >= 2 {
		if pairs[0].Key > pairs[1].Key {
			t.Error("GetAll() results not sorted by key")
		}
	}

	// No matches returns empty slice, not error
	pairs, err = s.GetAll("/nomatch/*")
	if err != nil {
		t.Fatalf("GetAll() error: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("GetAll() returned %d pairs, want 0", len(pairs))
	}
}

func TestStore_GetAll_InvalidPattern(t *testing.T) {
	s := New()
	s.Set("/key", "value")

	_, err := s.GetAll("[invalid")
	if err == nil {
		t.Error("GetAll() expected error for invalid pattern")
	}
}

func TestStore_GetAllValues(t *testing.T) {
	s := New()
	s.Set("/app/a", "value1")
	s.Set("/app/b", "value2")
	s.Set("/app/c", "value3")

	values, err := s.GetAllValues("/app/*")
	if err != nil {
		t.Fatalf("GetAllValues() error: %v", err)
	}
	if len(values) != 3 {
		t.Errorf("GetAllValues() returned %d values, want 3", len(values))
	}

	// Values should be sorted
	for i := 1; i < len(values); i++ {
		if values[i-1] > values[i] {
			t.Error("GetAllValues() results not sorted")
			break
		}
	}
}

func TestStore_List(t *testing.T) {
	s := New()
	s.Set("/app/config/key1", "value1")
	s.Set("/app/config/key2", "value2")
	s.Set("/app/data/key3", "value3")

	items := s.List("/app")
	if len(items) != 2 {
		t.Errorf("List() returned %d items, want 2 (config, data)", len(items))
	}

	// Check contents
	found := make(map[string]bool)
	for _, item := range items {
		found[item] = true
	}
	if !found["config"] || !found["data"] {
		t.Errorf("List() = %v, want [config, data]", items)
	}
}

func TestStore_ListDir(t *testing.T) {
	s := New()
	s.Set("/app/config/db/host", "localhost")
	s.Set("/app/config/cache/host", "redis")
	s.Set("/app/config/key", "value") // leaf at config level

	dirs := s.ListDir("/app/config")
	if len(dirs) != 2 {
		t.Errorf("ListDir() returned %d dirs, want 2 (db, cache)", len(dirs))
	}

	found := make(map[string]bool)
	for _, dir := range dirs {
		found[dir] = true
	}
	if !found["db"] || !found["cache"] {
		t.Errorf("ListDir() = %v, want [db, cache]", dirs)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	n := 100

	// Concurrent writes
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.Set("/key", "value")
		}(i)
	}

	// Concurrent reads
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Get("/key")
			s.Exists("/key")
			s.GetAll("/*")
		}()
	}

	wg.Wait()
}

func TestKeyError_Unwrap(t *testing.T) {
	err := &KeyError{Key: "/test", Err: ErrNotExist}

	if !errors.Is(err, ErrNotExist) {
		t.Error("errors.Is should return true for wrapped ErrNotExist")
	}

	var keyErr *KeyError
	if !errors.As(err, &keyErr) {
		t.Error("errors.As should work with *KeyError")
	}
}

func TestKeyError_Error(t *testing.T) {
	err := &KeyError{Key: "/test/key", Err: ErrNotExist}
	expected := "key does not exist: /test/key"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestKVPairs_Sort(t *testing.T) {
	pairs := KVPairs{
		{Key: "/c", Value: "3"},
		{Key: "/a", Value: "1"},
		{Key: "/b", Value: "2"},
	}

	// Test sort interface methods
	if pairs.Len() != 3 {
		t.Errorf("Len() = %d, want 3", pairs.Len())
	}

	if !pairs.Less(1, 0) { // /a < /c
		t.Error("Less() should return true for /a < /c")
	}

	pairs.Swap(0, 1)
	if pairs[0].Key != "/a" || pairs[1].Key != "/c" {
		t.Error("Swap() did not swap correctly")
	}
}

// setupBenchmarkStore creates a store with a hierarchical key structure for benchmarking.
func setupBenchmarkStore(numKeys int) *Store {
	s := New()
	// Create a hierarchical structure: /app/service{i}/config/key{j}
	for i := 0; i < numKeys/10; i++ {
		for j := 0; j < 10; j++ {
			key := fmt.Sprintf("/app/service%d/config/key%d", i, j)
			s.Set(key, fmt.Sprintf("value%d", j))
		}
	}
	return s
}

func BenchmarkList(b *testing.B) {
	s := setupBenchmarkStore(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.List("/app")
	}
}

func BenchmarkList_HighFrequency(b *testing.B) {
	s := setupBenchmarkStore(100)
	b.ResetTimer()
	// Simulate high-frequency calls as would happen in watch mode
	for i := 0; i < b.N; i++ {
		s.List("/app")
		s.List("/app/service0")
		s.List("/app/service1")
	}
}

func BenchmarkListDir(b *testing.B) {
	s := setupBenchmarkStore(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ListDir("/app")
	}
}

func BenchmarkListDir_HighFrequency(b *testing.B) {
	s := setupBenchmarkStore(100)
	b.ResetTimer()
	// Simulate high-frequency calls as would happen in watch mode
	for i := 0; i < b.N; i++ {
		s.ListDir("/app")
		s.ListDir("/app/service0")
		s.ListDir("/app/service1")
	}
}

func BenchmarkList_LargeStore(b *testing.B) {
	s := setupBenchmarkStore(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.List("/app")
	}
}

func BenchmarkListDir_LargeStore(b *testing.B) {
	s := setupBenchmarkStore(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ListDir("/app")
	}
}
