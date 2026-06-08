package api

import (
	"fmt"
	"testing"
)

// ─────────────────────────────────────────────────────────
// Mock Backend — implements BackendProvider in-memory
// ─────────────────────────────────────────────────────────

type MockBackend struct {
	data   map[string]string
	closed bool
}

func NewMockBackend() *MockBackend {
	return &MockBackend{
		data: map[string]string{
			"/app/database/host": "db-master.internal",
			"/app/database/port": "5432",
			"/app/feature_flags/maintenance": "false",
		},
	}
}

func (m *MockBackend) GetValues(keys []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, key := range keys {
		if val, ok := m.data[key]; ok {
			result[key] = val
		}
	}
	return result, nil
}

func (m *MockBackend) WatchPrefix(prefix string, keys []string, waitIndex uint64) (uint64, error) {
	return waitIndex + 1, nil
}

func (m *MockBackend) HealthCheck() error {
	if m.closed {
		return fmt.Errorf("backend is closed")
	}
	return nil
}

func (m *MockBackend) Close() error {
	m.closed = true
	return nil
}

// ─────────────────────────────────────────────────────────
// Tests for the RPC Server/Client layer (in-process)
// ─────────────────────────────────────────────────────────

func TestRPCServer_GetValues(t *testing.T) {
	mock := NewMockBackend()
	server := &BackendRPCServer{Impl: mock}

	args := GetValuesArgs{Keys: []string{"/app/database/host"}}
	var reply GetValuesReply

	err := server.GetValues(args, &reply)
	if err != nil {
		t.Fatalf("GetValues returned error: %v", err)
	}
	if reply.Values["/app/database/host"] != "db-master.internal" {
		t.Errorf("got %q, want %q", reply.Values["/app/database/host"], "db-master.internal")
	}
}

func TestRPCServer_GetValues_MultipleKeys(t *testing.T) {
	mock := NewMockBackend()
	server := &BackendRPCServer{Impl: mock}

	args := GetValuesArgs{Keys: []string{"/app/database/host", "/app/database/port"}}
	var reply GetValuesReply

	err := server.GetValues(args, &reply)
	if err != nil {
		t.Fatalf("GetValues returned error: %v", err)
	}
	if len(reply.Values) != 2 {
		t.Errorf("expected 2 values, got %d", len(reply.Values))
	}
	if reply.Values["/app/database/port"] != "5432" {
		t.Errorf("port = %q, want %q", reply.Values["/app/database/port"], "5432")
	}
}

func TestRPCServer_GetValues_MissingKey(t *testing.T) {
	mock := NewMockBackend()
	server := &BackendRPCServer{Impl: mock}

	args := GetValuesArgs{Keys: []string{"/nonexistent/key"}}
	var reply GetValuesReply

	err := server.GetValues(args, &reply)
	if err != nil {
		t.Fatalf("GetValues returned error: %v", err)
	}
	if len(reply.Values) != 0 {
		t.Errorf("expected 0 values for missing key, got %d", len(reply.Values))
	}
}

func TestRPCServer_GetValues_EmptyKeys(t *testing.T) {
	mock := NewMockBackend()
	server := &BackendRPCServer{Impl: mock}

	args := GetValuesArgs{Keys: []string{}}
	var reply GetValuesReply

	err := server.GetValues(args, &reply)
	if err != nil {
		t.Fatalf("GetValues returned error: %v", err)
	}
	if len(reply.Values) != 0 {
		t.Errorf("expected 0 values for empty keys, got %d", len(reply.Values))
	}
}

func TestRPCServer_WatchPrefix(t *testing.T) {
	mock := NewMockBackend()
	server := &BackendRPCServer{Impl: mock}

	args := WatchPrefixArgs{Prefix: "/app", Keys: []string{"/app/database/host"}, WaitIndex: 0}
	var reply WatchPrefixReply

	err := server.WatchPrefix(args, &reply)
	if err != nil {
		t.Fatalf("WatchPrefix returned error: %v", err)
	}
	if reply.WaitIndex != 1 {
		t.Errorf("expected WaitIndex=1, got %d", reply.WaitIndex)
	}
}

func TestRPCServer_WatchPrefix_IncrementingIndex(t *testing.T) {
	mock := NewMockBackend()
	server := &BackendRPCServer{Impl: mock}

	args := WatchPrefixArgs{Prefix: "/app", Keys: []string{}, WaitIndex: 42}
	var reply WatchPrefixReply

	err := server.WatchPrefix(args, &reply)
	if err != nil {
		t.Fatalf("WatchPrefix returned error: %v", err)
	}
	if reply.WaitIndex != 43 {
		t.Errorf("expected WaitIndex=43, got %d", reply.WaitIndex)
	}
}

func TestRPCServer_HealthCheck_Healthy(t *testing.T) {
	mock := NewMockBackend()
	server := &BackendRPCServer{Impl: mock}

	var reply struct{}
	err := server.HealthCheck(nil, &reply)
	if err != nil {
		t.Fatalf("HealthCheck should pass on a healthy backend, got: %v", err)
	}
}

func TestRPCServer_HealthCheck_AfterClose(t *testing.T) {
	mock := NewMockBackend()
	server := &BackendRPCServer{Impl: mock}

	// Close the backend first
	var closeReply struct{}
	_ = server.Close(nil, &closeReply)

	// Now HealthCheck should fail
	var reply struct{}
	err := server.HealthCheck(nil, &reply)
	if err == nil {
		t.Error("HealthCheck should fail after Close()")
	}
}

func TestRPCServer_Close(t *testing.T) {
	mock := NewMockBackend()
	server := &BackendRPCServer{Impl: mock}

	var reply struct{}
	err := server.Close(nil, &reply)
	if err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if !mock.closed {
		t.Error("backend should be marked as closed")
	}
}

// ─────────────────────────────────────────────────────────
// Tests for the Handshake Configuration
// ─────────────────────────────────────────────────────────

func TestHandshake_Values(t *testing.T) {
	if Handshake.ProtocolVersion != 1 {
		t.Errorf("ProtocolVersion = %d, want 1", Handshake.ProtocolVersion)
	}
	if Handshake.MagicCookieKey != "CONFD_PLUGIN" {
		t.Errorf("MagicCookieKey = %q, want %q", Handshake.MagicCookieKey, "CONFD_PLUGIN")
	}
	if Handshake.MagicCookieValue != "hello_confd" {
		t.Errorf("MagicCookieValue = %q, want %q", Handshake.MagicCookieValue, "hello_confd")
	}
}

// ─────────────────────────────────────────────────────────
// Tests for the Plugin Boilerplate
// ─────────────────────────────────────────────────────────

func TestConfdBackendPlugin_Server(t *testing.T) {
	mock := NewMockBackend()
	p := &ConfdBackendPlugin{Impl: mock}

	server, err := p.Server(nil)
	if err != nil {
		t.Fatalf("Server() returned error: %v", err)
	}
	if server == nil {
		t.Fatal("Server() returned nil")
	}

	// Verify it's the right type
	rpcServer, ok := server.(*BackendRPCServer)
	if !ok {
		t.Fatal("Server() did not return *BackendRPCServer")
	}
	if rpcServer.Impl != mock {
		t.Error("Server Impl does not match the provided mock")
	}
}

// ─────────────────────────────────────────────────────────
// Tests for DTO Serialization (struct fields)
// ─────────────────────────────────────────────────────────

func TestGetValuesArgs_Fields(t *testing.T) {
	args := GetValuesArgs{Keys: []string{"/a", "/b", "/c"}}
	if len(args.Keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(args.Keys))
	}
}

func TestWatchPrefixArgs_Fields(t *testing.T) {
	args := WatchPrefixArgs{
		Prefix:    "/app",
		Keys:      []string{"/app/db"},
		WaitIndex: 99,
	}
	if args.Prefix != "/app" {
		t.Errorf("Prefix = %q, want /app", args.Prefix)
	}
	if args.WaitIndex != 99 {
		t.Errorf("WaitIndex = %d, want 99", args.WaitIndex)
	}
}
