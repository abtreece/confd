package zookeeper

import (
	"context"
	"errors"
	"testing"

	zk "github.com/go-zookeeper/zk"
)

// mockZkConn implements zkConn for testing
type mockZkConn struct {
	childrenFunc  func(path string) ([]string, *zk.Stat, error)
	getFunc       func(path string) ([]byte, *zk.Stat, error)
	existsFunc    func(path string) (bool, *zk.Stat, error)
	getWFunc      func(path string) ([]byte, *zk.Stat, <-chan zk.Event, error)
	childrenWFunc func(path string) ([]string, *zk.Stat, <-chan zk.Event, error)
}

func (m *mockZkConn) Children(path string) ([]string, *zk.Stat, error) {
	if m.childrenFunc != nil {
		return m.childrenFunc(path)
	}
	return nil, nil, nil
}

func (m *mockZkConn) Get(path string) ([]byte, *zk.Stat, error) {
	if m.getFunc != nil {
		return m.getFunc(path)
	}
	return nil, nil, nil
}

func (m *mockZkConn) Exists(path string) (bool, *zk.Stat, error) {
	if m.existsFunc != nil {
		return m.existsFunc(path)
	}
	return true, &zk.Stat{}, nil
}

func (m *mockZkConn) GetW(path string) ([]byte, *zk.Stat, <-chan zk.Event, error) {
	if m.getWFunc != nil {
		return m.getWFunc(path)
	}
	ch := make(chan zk.Event)
	return nil, nil, ch, nil
}

func (m *mockZkConn) ChildrenW(path string) ([]string, *zk.Stat, <-chan zk.Event, error) {
	if m.childrenWFunc != nil {
		return m.childrenWFunc(path)
	}
	ch := make(chan zk.Event)
	return nil, nil, ch, nil
}

func TestWatchPrefix_InitialCall(t *testing.T) {
	// Create a client with nil connection - WatchPrefix with waitIndex=0
	// should return immediately without using the connection
	client := &Client{client: nil}

	// waitIndex 0 should return immediately with 1
	index, err := client.WatchPrefix(context.Background(), "/app", []string{"/app/key"}, 0, nil)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 1 {
		t.Errorf("WatchPrefix() index = %d, want 1", index)
	}
}

func TestGetValues_SingleLeafNode(t *testing.T) {
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			return true, &zk.Stat{}, nil
		},
		childrenFunc: func(path string) ([]string, *zk.Stat, error) {
			// Return stat with NumChildren=0 to indicate leaf node
			return []string{}, &zk.Stat{NumChildren: 0}, nil
		},
		getFunc: func(path string) ([]byte, *zk.Stat, error) {
			if path == "/app/key" {
				return []byte("value123"), &zk.Stat{}, nil
			}
			return nil, nil, errors.New("not found")
		},
	}

	client := &Client{client: mock}

	vars, err := client.GetValues(context.Background(), []string{"/app/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if vars["/app/key"] != "value123" {
		t.Errorf("GetValues() = %v, want /app/key=value123", vars)
	}
}

func TestGetValues_WithChildren(t *testing.T) {
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			if path == "/app/child1" || path == "/app/child2" {
				return true, &zk.Stat{NumChildren: 0}, nil
			}
			return true, &zk.Stat{NumChildren: 2}, nil
		},
		childrenFunc: func(path string) ([]string, *zk.Stat, error) {
			if path == "/app" {
				return []string{"child1", "child2"}, &zk.Stat{NumChildren: 2}, nil
			}
			return []string{}, &zk.Stat{NumChildren: 0}, nil
		},
		getFunc: func(path string) ([]byte, *zk.Stat, error) {
			switch path {
			case "/app/child1":
				return []byte("val1"), &zk.Stat{}, nil
			case "/app/child2":
				return []byte("val2"), &zk.Stat{}, nil
			}
			return nil, nil, errors.New("not found")
		},
	}

	client := &Client{client: mock}

	vars, err := client.GetValues(context.Background(), []string{"/app"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if vars["/app/child1"] != "val1" {
		t.Errorf("GetValues()[/app/child1] = %s, want val1", vars["/app/child1"])
	}
	if vars["/app/child2"] != "val2" {
		t.Errorf("GetValues()[/app/child2] = %s, want val2", vars["/app/child2"])
	}
}

func TestGetValues_NestedChildren(t *testing.T) {
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			switch path {
			case "/app":
				return true, &zk.Stat{NumChildren: 1}, nil
			case "/app/config":
				return true, &zk.Stat{NumChildren: 2}, nil
			case "/app/config/db", "/app/config/port":
				return true, &zk.Stat{NumChildren: 0}, nil
			}
			return true, &zk.Stat{}, nil
		},
		childrenFunc: func(path string) ([]string, *zk.Stat, error) {
			switch path {
			case "/app":
				return []string{"config"}, &zk.Stat{NumChildren: 1}, nil
			case "/app/config":
				return []string{"db", "port"}, &zk.Stat{NumChildren: 2}, nil
			}
			return []string{}, &zk.Stat{NumChildren: 0}, nil
		},
		getFunc: func(path string) ([]byte, *zk.Stat, error) {
			switch path {
			case "/app/config/db":
				return []byte("mysql"), &zk.Stat{}, nil
			case "/app/config/port":
				return []byte("3306"), &zk.Stat{}, nil
			}
			return nil, nil, errors.New("not found")
		},
	}

	client := &Client{client: mock}

	vars, err := client.GetValues(context.Background(), []string{"/app"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if vars["/app/config/db"] != "mysql" {
		t.Errorf("GetValues()[/app/config/db] = %s, want mysql", vars["/app/config/db"])
	}
	if vars["/app/config/port"] != "3306" {
		t.Errorf("GetValues()[/app/config/port] = %s, want 3306", vars["/app/config/port"])
	}
}

func TestGetValues_MultipleKeys(t *testing.T) {
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			return true, &zk.Stat{NumChildren: 0}, nil
		},
		childrenFunc: func(path string) ([]string, *zk.Stat, error) {
			return []string{}, &zk.Stat{NumChildren: 0}, nil
		},
		getFunc: func(path string) ([]byte, *zk.Stat, error) {
			switch path {
			case "/app/key1":
				return []byte("val1"), &zk.Stat{}, nil
			case "/app/key2":
				return []byte("val2"), &zk.Stat{}, nil
			case "/db/host":
				return []byte("localhost"), &zk.Stat{}, nil
			}
			return nil, nil, errors.New("not found")
		},
	}

	client := &Client{client: mock}

	vars, err := client.GetValues(context.Background(), []string{"/app/key1", "/app/key2", "/db/host"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/app/key1": "val1",
		"/app/key2": "val2",
		"/db/host":  "localhost",
	}

	for k, v := range expected {
		if vars[k] != v {
			t.Errorf("GetValues()[%s] = %s, want %s", k, vars[k], v)
		}
	}
}

func TestGetValues_ExistsError(t *testing.T) {
	expectedErr := errors.New("connection error")
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			return false, nil, expectedErr
		},
	}

	client := &Client{client: mock}

	_, err := client.GetValues(context.Background(), []string{"/app/key"})
	if err != expectedErr {
		t.Errorf("GetValues() error = %v, want %v", err, expectedErr)
	}
}

func TestGetValues_ChildrenError(t *testing.T) {
	expectedErr := errors.New("children error")
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			return true, &zk.Stat{}, nil
		},
		childrenFunc: func(path string) ([]string, *zk.Stat, error) {
			return nil, nil, expectedErr
		},
	}

	client := &Client{client: mock}

	_, err := client.GetValues(context.Background(), []string{"/app"})
	if err != expectedErr {
		t.Errorf("GetValues() error = %v, want %v", err, expectedErr)
	}
}

func TestGetValues_GetError(t *testing.T) {
	expectedErr := errors.New("get error")
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			return true, &zk.Stat{}, nil
		},
		childrenFunc: func(path string) ([]string, *zk.Stat, error) {
			return []string{}, &zk.Stat{NumChildren: 0}, nil
		},
		getFunc: func(path string) ([]byte, *zk.Stat, error) {
			return nil, nil, expectedErr
		},
	}

	client := &Client{client: mock}

	_, err := client.GetValues(context.Background(), []string{"/app/key"})
	if err != expectedErr {
		t.Errorf("GetValues() error = %v, want %v", err, expectedErr)
	}
}

func TestGetValues_WildcardRemoval(t *testing.T) {
	pathReceived := ""
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			pathReceived = path
			return true, &zk.Stat{}, nil
		},
		childrenFunc: func(path string) ([]string, *zk.Stat, error) {
			return []string{}, &zk.Stat{NumChildren: 0}, nil
		},
		getFunc: func(path string) ([]byte, *zk.Stat, error) {
			return []byte("value"), &zk.Stat{}, nil
		},
	}

	client := &Client{client: mock}

	_, err := client.GetValues(context.Background(), []string{"/app/*"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Wildcard should be removed
	if pathReceived != "/app" {
		t.Errorf("GetValues() called Exists with path %s, want /app", pathReceived)
	}
}

func TestGetValues_RootPath(t *testing.T) {
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			return true, &zk.Stat{NumChildren: 1}, nil
		},
		childrenFunc: func(path string) ([]string, *zk.Stat, error) {
			if path == "/" {
				return []string{"app"}, &zk.Stat{NumChildren: 1}, nil
			}
			return []string{}, &zk.Stat{NumChildren: 0}, nil
		},
		getFunc: func(path string) ([]byte, *zk.Stat, error) {
			if path == "/app" {
				return []byte("appvalue"), &zk.Stat{}, nil
			}
			return nil, nil, errors.New("not found")
		},
	}

	client := &Client{client: mock}

	vars, err := client.GetValues(context.Background(), []string{"/"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if vars["/app"] != "appvalue" {
		t.Errorf("GetValues()[/app] = %s, want appvalue", vars["/app"])
	}
}

func TestWatchPrefix_StopChan(t *testing.T) {
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			return true, &zk.Stat{NumChildren: 0}, nil
		},
		childrenFunc: func(path string) ([]string, *zk.Stat, error) {
			return []string{}, &zk.Stat{NumChildren: 0}, nil
		},
		getFunc: func(path string) ([]byte, *zk.Stat, error) {
			return []byte("value"), &zk.Stat{}, nil
		},
		getWFunc: func(path string) ([]byte, *zk.Stat, <-chan zk.Event, error) {
			ch := make(chan zk.Event)
			return []byte("value"), &zk.Stat{}, ch, nil
		},
		childrenWFunc: func(path string) ([]string, *zk.Stat, <-chan zk.Event, error) {
			ch := make(chan zk.Event)
			return []string{}, &zk.Stat{}, ch, nil
		},
	}

	client := &Client{client: mock}
	stopChan := make(chan bool, 1)

	// Send stop signal
	go func() {
		stopChan <- true
	}()

	index, err := client.WatchPrefix(context.Background(), "/app", []string{"/app/key"}, 1, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 1 {
		t.Errorf("WatchPrefix() index = %d, want 1", index)
	}
}

func TestWatchPrefix_GetValuesError(t *testing.T) {
	expectedErr := errors.New("getvalues error")
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			return false, nil, expectedErr
		},
	}

	client := &Client{client: mock}
	stopChan := make(chan bool)

	_, err := client.WatchPrefix(context.Background(), "/app", []string{"/app/key"}, 1, stopChan)
	if err != expectedErr {
		t.Errorf("WatchPrefix() error = %v, want %v", err, expectedErr)
	}
}

func TestWatch_GetWError(t *testing.T) {
	expectedErr := errors.New("getw error")
	mock := &mockZkConn{
		getWFunc: func(path string) ([]byte, *zk.Stat, <-chan zk.Event, error) {
			return nil, nil, nil, expectedErr
		},
	}

	client := &Client{client: mock}
	respChan := make(chan watchResponse, 1)
	cancelRoutine := make(chan bool)

	go client.watch("/app/key", respChan, cancelRoutine)

	resp := <-respChan
	if resp.err != expectedErr {
		t.Errorf("watch() error = %v, want %v", resp.err, expectedErr)
	}
	close(cancelRoutine)
}

func TestWatch_ChildrenWError(t *testing.T) {
	expectedErr := errors.New("childrenw error")
	mock := &mockZkConn{
		getWFunc: func(path string) ([]byte, *zk.Stat, <-chan zk.Event, error) {
			ch := make(chan zk.Event)
			return []byte("value"), &zk.Stat{}, ch, nil
		},
		childrenWFunc: func(path string) ([]string, *zk.Stat, <-chan zk.Event, error) {
			return nil, nil, nil, expectedErr
		},
	}

	client := &Client{client: mock}
	respChan := make(chan watchResponse, 1)
	cancelRoutine := make(chan bool)

	go client.watch("/app/key", respChan, cancelRoutine)

	resp := <-respChan
	if resp.err != expectedErr {
		t.Errorf("watch() error = %v, want %v", resp.err, expectedErr)
	}
	close(cancelRoutine)
}

func TestWatch_DataChanged(t *testing.T) {
	keyEventCh := make(chan zk.Event, 1)
	mock := &mockZkConn{
		getWFunc: func(path string) ([]byte, *zk.Stat, <-chan zk.Event, error) {
			return []byte("value"), &zk.Stat{}, keyEventCh, nil
		},
		childrenWFunc: func(path string) ([]string, *zk.Stat, <-chan zk.Event, error) {
			ch := make(chan zk.Event)
			return []string{}, &zk.Stat{}, ch, nil
		},
	}

	client := &Client{client: mock}
	respChan := make(chan watchResponse, 1)
	cancelRoutine := make(chan bool)

	go client.watch("/app/key", respChan, cancelRoutine)

	// Send data changed event
	keyEventCh <- zk.Event{Type: zk.EventNodeDataChanged}

	resp := <-respChan
	if resp.waitIndex != 1 {
		t.Errorf("watch() waitIndex = %d, want 1", resp.waitIndex)
	}
	close(cancelRoutine)
}

func TestWatch_ChildrenChanged(t *testing.T) {
	childEventCh := make(chan zk.Event, 1)
	mock := &mockZkConn{
		getWFunc: func(path string) ([]byte, *zk.Stat, <-chan zk.Event, error) {
			ch := make(chan zk.Event)
			return []byte("value"), &zk.Stat{}, ch, nil
		},
		childrenWFunc: func(path string) ([]string, *zk.Stat, <-chan zk.Event, error) {
			return []string{}, &zk.Stat{}, childEventCh, nil
		},
	}

	client := &Client{client: mock}
	respChan := make(chan watchResponse, 1)
	cancelRoutine := make(chan bool)

	go client.watch("/app/key", respChan, cancelRoutine)

	// Send children changed event
	childEventCh <- zk.Event{Type: zk.EventNodeChildrenChanged}

	resp := <-respChan
	if resp.waitIndex != 1 {
		t.Errorf("watch() waitIndex = %d, want 1", resp.waitIndex)
	}
	close(cancelRoutine)
}

func TestWatch_Cancel(t *testing.T) {
	mock := &mockZkConn{
		getWFunc: func(path string) ([]byte, *zk.Stat, <-chan zk.Event, error) {
			ch := make(chan zk.Event)
			return []byte("value"), &zk.Stat{}, ch, nil
		},
		childrenWFunc: func(path string) ([]string, *zk.Stat, <-chan zk.Event, error) {
			ch := make(chan zk.Event)
			return []string{}, &zk.Stat{}, ch, nil
		},
	}

	client := &Client{client: mock}
	respChan := make(chan watchResponse, 1)
	cancelRoutine := make(chan bool, 1)

	go client.watch("/app/key", respChan, cancelRoutine)

	// Cancel the watch
	cancelRoutine <- true

	// Give goroutine time to exit
	select {
	case <-respChan:
		t.Error("watch() should not send response when cancelled")
	default:
		// Expected - no response
	}
}

func TestHealthCheck_Success(t *testing.T) {
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			return true, &zk.Stat{}, nil
		},
	}

	client := &Client{client: mock}

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() unexpected error: %v", err)
	}
}

func TestHealthCheck_Error(t *testing.T) {
	expectedErr := errors.New("connection lost")
	mock := &mockZkConn{
		existsFunc: func(path string) (bool, *zk.Stat, error) {
			return false, nil, expectedErr
		},
	}

	client := &Client{client: mock}

	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("HealthCheck() error = %v, want %v", err, expectedErr)
	}
}

// Note: Full integration tests require a running Zookeeper instance.
// These are covered by integration tests in .github/workflows/integration-tests.yml
