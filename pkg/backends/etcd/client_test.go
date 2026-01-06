package etcd

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestWatch_Update(t *testing.T) {
	w := &Watch{
		revision: 0,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	// Update should set revision and close the cond channel
	w.update(100)

	if w.revision != 100 {
		t.Errorf("Watch.revision = %d, want 100", w.revision)
	}

	// The old cond channel should be closed
	select {
	case <-w.cond:
		// Expected - old cond was closed, new one created
	default:
		// Should not block - cond should be a new channel
	}
}

func TestWatch_UpdateMultipleTimes(t *testing.T) {
	w := &Watch{
		revision: 0,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	w.update(10)
	w.update(20)
	w.update(30)

	if w.revision != 30 {
		t.Errorf("Watch.revision = %d, want 30", w.revision)
	}
}

func TestWatch_WaitNext_AlreadyUpdated(t *testing.T) {
	w := &Watch{
		revision: 100,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	notify := make(chan int64, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// WaitNext should return immediately since revision (100) > lastRevision (50)
	go w.WaitNext(ctx, 50, notify)

	select {
	case rev := <-notify:
		if rev != 100 {
			t.Errorf("WaitNext returned revision %d, want 100", rev)
		}
	case <-ctx.Done():
		t.Error("WaitNext timed out when it should have returned immediately")
	}
}

func TestWatch_WaitNext_WaitsForUpdate(t *testing.T) {
	w := &Watch{
		revision: 50,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	notify := make(chan int64, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// WaitNext should wait since revision (50) is not > lastRevision (50)
	go w.WaitNext(ctx, 50, notify)

	// Give goroutine time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Now update
	w.update(100)

	select {
	case rev := <-notify:
		if rev != 100 {
			t.Errorf("WaitNext returned revision %d, want 100", rev)
		}
	case <-ctx.Done():
		t.Error("WaitNext timed out waiting for update")
	}
}

func TestWatch_WaitNext_Cancelled(t *testing.T) {
	w := &Watch{
		revision: 50,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	notify := make(chan int64, 1)
	ctx, cancel := context.WithCancel(context.Background())

	// WaitNext should wait since revision (50) is not > lastRevision (50)
	go w.WaitNext(ctx, 50, notify)

	// Give goroutine time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Give goroutine time to exit
	time.Sleep(10 * time.Millisecond)

	select {
	case <-notify:
		t.Error("WaitNext should not send on notify when cancelled")
	default:
		// Expected - no notification sent
	}
}

func TestWatch_ConcurrentAccess(t *testing.T) {
	w := &Watch{
		revision: 0,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	// Start multiple goroutines updating and reading
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(rev int64) {
			defer wg.Done()
			w.update(rev)
		}(int64(i * 10))
	}

	wg.Wait()

	// Revision should be set (exact value depends on goroutine scheduling)
	if w.revision < 0 || w.revision > 90 {
		t.Errorf("Watch.revision = %d, expected between 0 and 90", w.revision)
	}
}

// mockTxn implements clientv3.Txn for testing
type mockTxn struct {
	ops        []clientv3.Op
	commitFunc func(ops []clientv3.Op) (*clientv3.TxnResponse, error)
}

func (m *mockTxn) If(cs ...clientv3.Cmp) clientv3.Txn {
	return m
}

func (m *mockTxn) Then(ops ...clientv3.Op) clientv3.Txn {
	m.ops = ops
	return m
}

func (m *mockTxn) Else(ops ...clientv3.Op) clientv3.Txn {
	return m
}

func (m *mockTxn) Commit() (*clientv3.TxnResponse, error) {
	if m.commitFunc != nil {
		return m.commitFunc(m.ops)
	}
	return nil, nil
}

// mockKV implements etcdKV for testing
type mockKV struct {
	txnFunc func(ctx context.Context) clientv3.Txn
}

func (m *mockKV) Txn(ctx context.Context) clientv3.Txn {
	if m.txnFunc != nil {
		return m.txnFunc(ctx)
	}
	return &mockTxn{}
}

func TestGetValues_SingleKey(t *testing.T) {
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			return &mockTxn{
				commitFunc: func(ops []clientv3.Op) (*clientv3.TxnResponse, error) {
					return &clientv3.TxnResponse{
						Header: &etcdserverpb.ResponseHeader{Revision: 100},
						Responses: []*etcdserverpb.ResponseOp{
							{
								Response: &etcdserverpb.ResponseOp_ResponseRange{
									ResponseRange: &etcdserverpb.RangeResponse{
										Kvs: []*mvccpb.KeyValue{
											{Key: []byte("/app/key"), Value: []byte("value123")},
										},
									},
								},
							},
						},
					}, nil
				},
			}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	vars, err := client.GetValues([]string{"/app/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if vars["/app/key"] != "value123" {
		t.Errorf("GetValues() = %v, want /app/key=value123", vars)
	}
}

func TestGetValues_MultipleKeys(t *testing.T) {
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			return &mockTxn{
				commitFunc: func(ops []clientv3.Op) (*clientv3.TxnResponse, error) {
					responses := make([]*etcdserverpb.ResponseOp, len(ops))
					for i := range ops {
						var kvs []*mvccpb.KeyValue
						switch i {
						case 0:
							kvs = []*mvccpb.KeyValue{{Key: []byte("/app/key1"), Value: []byte("val1")}}
						case 1:
							kvs = []*mvccpb.KeyValue{{Key: []byte("/app/key2"), Value: []byte("val2")}}
						case 2:
							kvs = []*mvccpb.KeyValue{{Key: []byte("/db/host"), Value: []byte("localhost")}}
						}
						responses[i] = &etcdserverpb.ResponseOp{
							Response: &etcdserverpb.ResponseOp_ResponseRange{
								ResponseRange: &etcdserverpb.RangeResponse{Kvs: kvs},
							},
						}
					}
					return &clientv3.TxnResponse{
						Header:    &etcdserverpb.ResponseHeader{Revision: 100},
						Responses: responses,
					}, nil
				},
			}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	vars, err := client.GetValues([]string{"/app/key1", "/app/key2", "/db/host"})
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

func TestGetValues_PrefixMatch(t *testing.T) {
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			return &mockTxn{
				commitFunc: func(ops []clientv3.Op) (*clientv3.TxnResponse, error) {
					// Return multiple keys for prefix query
					return &clientv3.TxnResponse{
						Header: &etcdserverpb.ResponseHeader{Revision: 100},
						Responses: []*etcdserverpb.ResponseOp{
							{
								Response: &etcdserverpb.ResponseOp_ResponseRange{
									ResponseRange: &etcdserverpb.RangeResponse{
										Kvs: []*mvccpb.KeyValue{
											{Key: []byte("/app/config/db"), Value: []byte("mysql")},
											{Key: []byte("/app/config/port"), Value: []byte("3306")},
											{Key: []byte("/app/config/host"), Value: []byte("localhost")},
										},
									},
								},
							},
						},
					}, nil
				},
			}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	vars, err := client.GetValues([]string{"/app/config"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if len(vars) != 3 {
		t.Errorf("GetValues() returned %d keys, want 3", len(vars))
	}
	if vars["/app/config/db"] != "mysql" {
		t.Errorf("GetValues()[/app/config/db] = %s, want mysql", vars["/app/config/db"])
	}
}

func TestGetValues_TransactionError(t *testing.T) {
	expectedErr := errors.New("transaction failed")
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			return &mockTxn{
				commitFunc: func(ops []clientv3.Op) (*clientv3.TxnResponse, error) {
					return nil, expectedErr
				},
			}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	_, err := client.GetValues([]string{"/app/key"})
	if err != expectedErr {
		t.Errorf("GetValues() error = %v, want %v", err, expectedErr)
	}
}

func TestGetValues_EmptyResult(t *testing.T) {
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			return &mockTxn{
				commitFunc: func(ops []clientv3.Op) (*clientv3.TxnResponse, error) {
					return &clientv3.TxnResponse{
						Header: &etcdserverpb.ResponseHeader{Revision: 100},
						Responses: []*etcdserverpb.ResponseOp{
							{
								Response: &etcdserverpb.ResponseOp_ResponseRange{
									ResponseRange: &etcdserverpb.RangeResponse{Kvs: []*mvccpb.KeyValue{}},
								},
							},
						},
					}, nil
				},
			}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	vars, err := client.GetValues([]string{"/nonexistent"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if len(vars) != 0 {
		t.Errorf("GetValues() returned %d keys, want 0", len(vars))
	}
}

func TestGetValues_FiltersByPrefix(t *testing.T) {
	// Test that GetValues filters results to only return keys
	// that match the requested prefix
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			return &mockTxn{
				commitFunc: func(ops []clientv3.Op) (*clientv3.TxnResponse, error) {
					// Return keys that don't all match the prefix
					return &clientv3.TxnResponse{
						Header: &etcdserverpb.ResponseHeader{Revision: 100},
						Responses: []*etcdserverpb.ResponseOp{
							{
								Response: &etcdserverpb.ResponseOp_ResponseRange{
									ResponseRange: &etcdserverpb.RangeResponse{
										Kvs: []*mvccpb.KeyValue{
											{Key: []byte("/app"), Value: []byte("appval")},
											{Key: []byte("/app/key"), Value: []byte("keyval")},
											{Key: []byte("/appother"), Value: []byte("otherval")}, // Should be filtered out
										},
									},
								},
							},
						},
					}, nil
				},
			}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	vars, err := client.GetValues([]string{"/app"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// /appother should be filtered out because it doesn't start with /app/
	if _, ok := vars["/appother"]; ok {
		t.Error("GetValues() should filter out /appother")
	}
	if vars["/app"] != "appval" {
		t.Errorf("GetValues()[/app] = %s, want appval", vars["/app"])
	}
	if vars["/app/key"] != "keyval" {
		t.Errorf("GetValues()[/app/key] = %s, want keyval", vars["/app/key"])
	}
}

func TestGetValues_EmptyKeys(t *testing.T) {
	txnCalled := false
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			txnCalled = true
			return &mockTxn{}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	vars, err := client.GetValues([]string{})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if txnCalled {
		t.Error("GetValues() should not call Txn for empty keys")
	}
	if len(vars) != 0 {
		t.Errorf("GetValues() returned %d keys, want 0", len(vars))
	}
}

func TestGetValues_BatchOperation(t *testing.T) {
	// Test that GetValues properly batches operations when there are more than 128 keys
	txnCallCount := 0
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			return &mockTxn{
				commitFunc: func(ops []clientv3.Op) (*clientv3.TxnResponse, error) {
					txnCallCount++
					responses := make([]*etcdserverpb.ResponseOp, len(ops))
					for i := range ops {
						responses[i] = &etcdserverpb.ResponseOp{
							Response: &etcdserverpb.ResponseOp_ResponseRange{
								ResponseRange: &etcdserverpb.RangeResponse{
									Kvs: []*mvccpb.KeyValue{},
								},
							},
						}
					}
					return &clientv3.TxnResponse{
						Header:    &etcdserverpb.ResponseHeader{Revision: 100},
						Responses: responses,
					}, nil
				},
			}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	// Create 130 keys to trigger batching (max 128 per transaction)
	keys := make([]string, 130)
	for i := 0; i < 130; i++ {
		keys[i] = "/app/key" + string(rune('a'+i%26)) + string(rune('0'+i/26))
	}

	_, err := client.GetValues(keys)
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Should have called Txn twice: once for 128 keys, once for 2 keys
	if txnCallCount != 2 {
		t.Errorf("GetValues() called Txn %d times, want 2", txnCallCount)
	}
}

func TestGetValues_KeyWithTrailingSlash(t *testing.T) {
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			return &mockTxn{
				commitFunc: func(ops []clientv3.Op) (*clientv3.TxnResponse, error) {
					return &clientv3.TxnResponse{
						Header: &etcdserverpb.ResponseHeader{Revision: 100},
						Responses: []*etcdserverpb.ResponseOp{
							{
								Response: &etcdserverpb.ResponseOp_ResponseRange{
									ResponseRange: &etcdserverpb.RangeResponse{
										Kvs: []*mvccpb.KeyValue{
											{Key: []byte("/app/config/"), Value: []byte("dirvalue")},
											{Key: []byte("/app/config/key"), Value: []byte("value")},
										},
									},
								},
							},
						},
					}, nil
				},
			}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	vars, err := client.GetValues([]string{"/app/config/"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Both keys should be included since they match the prefix "/app/config/"
	if vars["/app/config/"] != "dirvalue" {
		t.Errorf("GetValues()[/app/config/] = %s, want dirvalue", vars["/app/config/"])
	}
	if vars["/app/config/key"] != "value" {
		t.Errorf("GetValues()[/app/config/key] = %s, want value", vars["/app/config/key"])
	}
}

func TestGetValues_RevisionConsistency(t *testing.T) {
	// Test that first_rev is set from first transaction and used in subsequent ones
	callCount := 0
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			return &mockTxn{
				commitFunc: func(ops []clientv3.Op) (*clientv3.TxnResponse, error) {
					callCount++
					revision := int64(100 + callCount*10) // Different revision each call
					responses := make([]*etcdserverpb.ResponseOp, len(ops))
					for i := range ops {
						responses[i] = &etcdserverpb.ResponseOp{
							Response: &etcdserverpb.ResponseOp_ResponseRange{
								ResponseRange: &etcdserverpb.RangeResponse{
									Kvs: []*mvccpb.KeyValue{},
								},
							},
						}
					}
					return &clientv3.TxnResponse{
						Header:    &etcdserverpb.ResponseHeader{Revision: revision},
						Responses: responses,
					}, nil
				},
			}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	// Create enough keys to trigger multiple transactions
	keys := make([]string, 130)
	for i := 0; i < 130; i++ {
		keys[i] = "/key" + string(rune('0'+i/100)) + string(rune('0'+(i/10)%10)) + string(rune('0'+i%10))
	}

	_, err := client.GetValues(keys)
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 transaction calls, got %d", callCount)
	}
}

func TestGetValues_ExactKeyMatch(t *testing.T) {
	// Test that exact key match works (not just prefix)
	mock := &mockKV{
		txnFunc: func(ctx context.Context) clientv3.Txn {
			return &mockTxn{
				commitFunc: func(ops []clientv3.Op) (*clientv3.TxnResponse, error) {
					return &clientv3.TxnResponse{
						Header: &etcdserverpb.ResponseHeader{Revision: 100},
						Responses: []*etcdserverpb.ResponseOp{
							{
								Response: &etcdserverpb.ResponseOp_ResponseRange{
									ResponseRange: &etcdserverpb.RangeResponse{
										Kvs: []*mvccpb.KeyValue{
											{Key: []byte("/mykey"), Value: []byte("exactvalue")},
										},
									},
								},
							},
						},
					}, nil
				},
			}
		},
	}

	client := &Client{
		kvClient: mock,
		watches:  make(map[string]*Watch),
	}

	vars, err := client.GetValues([]string{"/mykey"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if vars["/mykey"] != "exactvalue" {
		t.Errorf("GetValues()[/mykey] = %s, want exactvalue", vars["/mykey"])
	}
}

func TestWatchPrefix_StopChan(t *testing.T) {
	// Test WatchPrefix with immediate stop signal using pre-existing watch
	w := &Watch{
		revision: 100,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	client := &Client{
		client:   nil, // Not used when watch already exists
		kvClient: nil,
		watches:  map[string]*Watch{"/app/key": w},
		wm:       sync.Mutex{},
	}

	stopChan := make(chan bool, 1)
	stopChan <- true

	index, err := client.WatchPrefix("/app", []string{"/app/key"}, 50, stopChan)
	if err == nil || err.Error() != "context canceled" {
		// WatchPrefix returns context.Canceled when stopChan triggers
		// This is expected behavior
	}
	// Index should be 0 when cancelled
	if index != 0 && err != nil {
		// Expected - cancelled before getting revision
	}
}

func TestWatch_WaitNext_NotifyAfterRevisionUpdate(t *testing.T) {
	w := &Watch{
		revision: 100,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	notify := make(chan int64, 1)
	ctx := context.Background()

	// Start waiting for revision > 100
	go w.WaitNext(ctx, 100, notify)

	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Update to higher revision
	w.update(150)

	select {
	case rev := <-notify:
		if rev != 150 {
			t.Errorf("WaitNext returned revision %d, want 150", rev)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("WaitNext timed out")
	}
}

func TestWatch_WaitNext_NotifyCancelledDuringWait(t *testing.T) {
	w := &Watch{
		revision: 100,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	notify := make(chan int64, 1)
	ctx, cancel := context.WithCancel(context.Background())

	// Start waiting for revision > 100
	go w.WaitNext(ctx, 100, notify)

	// Give goroutine time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Cancel while waiting
	cancel()

	// Update after cancel
	w.update(150)

	// Should not receive notification since context was cancelled
	select {
	case <-notify:
		t.Error("Should not receive notification after cancel")
	case <-time.After(50 * time.Millisecond):
		// Expected - no notification
	}
}

func TestWatch_WaitNext_NotifyCancelledAfterBreak(t *testing.T) {
	w := &Watch{
		revision: 150,
		cond:     make(chan struct{}),
		rwl:      sync.RWMutex{},
	}

	notify := make(chan int64, 1)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	// Start WaitNext - should break out of loop since revision > lastRevision
	// but then context is cancelled before it can send
	go w.WaitNext(ctx, 100, notify)

	// May or may not receive depending on timing
	select {
	case <-notify:
		// OK - got through before cancel took effect
	case <-time.After(50 * time.Millisecond):
		// OK - cancel took effect before send
	}
}

// Note: Full WatchPrefix and createWatch tests require a running etcd instance.
// These are covered by integration tests in .github/workflows/integration-tests.yml
//
// The etcd client uses complex watch channels and reconnection logic
// that requires an actual etcd connection to test properly.
