package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	"sync"

	"github.com/abtreece/confd/pkg/backends/types"
	"github.com/abtreece/confd/pkg/log"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdKV defines the interface for etcd KV operations.
// This allows for mocking in tests.
type etcdKV interface {
	Txn(ctx context.Context) clientv3.Txn
}

// A watch only tells the latest revision
type Watch struct {
	// Last seen revision
	revision int64
	// A channel to wait, will be closed after revision changes
	cond chan struct{}
	// Use RWMutex to protect cond variable
	rwl sync.RWMutex
}

// Wait until revision is greater than lastRevision
func (w *Watch) WaitNext(ctx context.Context, lastRevision int64, notify chan<- int64) {
	for {
		w.rwl.RLock()
		if w.revision > lastRevision {
			w.rwl.RUnlock()
			break
		}
		cond := w.cond
		w.rwl.RUnlock()
		select {
		case <-cond:
		case <-ctx.Done():
			return
		}
	}
	// We accept larger revision, so do not need to use RLock
	select {
	case notify <- w.revision:
	case <-ctx.Done():
	}
}

// Update revision
func (w *Watch) update(newRevision int64) {
	w.rwl.Lock()
	defer w.rwl.Unlock()
	w.revision = newRevision
	close(w.cond)
	w.cond = make(chan struct{})
}

func createWatch(client *clientv3.Client, prefix string) (*Watch, error) {
	w := &Watch{0, make(chan struct{}), sync.RWMutex{}}
	// Use the client's context so watches terminate when client is closed
	clientCtx := client.Ctx()
	go func() {
		rch := client.Watch(clientCtx, prefix, clientv3.WithPrefix(),
			clientv3.WithCreatedNotify())
		log.Debug("Watch created on %s", prefix)
		for {
			for wresp := range rch {
				if wresp.CompactRevision > w.revision {
					// respect CompactRevision
					w.update(wresp.CompactRevision)
					log.Debug("Watch to '%s' updated to %d by CompactRevision", prefix, wresp.CompactRevision)
				} else if wresp.Header.GetRevision() > w.revision {
					// Watch created or updated
					w.update(wresp.Header.GetRevision())
					log.Debug("Watch to '%s' updated to %d by header revision", prefix, wresp.Header.GetRevision())
				}
				if err := wresp.Err(); err != nil {
					log.Error("Watch error: %s", err.Error())
				}
			}

			// Check if client was closed - if so, exit the goroutine
			if clientCtx.Err() != nil {
				log.Debug("Watch to '%s' terminating - client closed", prefix)
				return
			}

			log.Warning("Watch to '%s' stopped at revision %d", prefix, w.revision)
			// Disconnected or cancelled
			// Wait for a moment to avoid reconnecting too quickly
			select {
			case <-time.After(time.Second):
			case <-clientCtx.Done():
				log.Debug("Watch to '%s' terminating - client closed during reconnect delay", prefix)
				return
			}

			// Start from next revision so we are not missing anything
			if w.revision > 0 {
				rch = client.Watch(clientCtx, prefix, clientv3.WithPrefix(),
					clientv3.WithRev(w.revision+1))
			} else {
				// Start from the latest revision
				rch = client.Watch(clientCtx, prefix, clientv3.WithPrefix(),
					clientv3.WithCreatedNotify())
			}
		}
	}()
	return w, nil
}

// Client is a wrapper around the etcd client
type Client struct {
	client    *clientv3.Client
	kvClient  etcdKV
	watches   map[string]*Watch
	// Protect watch
	wm sync.Mutex
}

// NewEtcdClient returns an *etcd.Client with a connection to named machines.
func NewEtcdClient(machines []string, cert, key, caCert string, clientInsecure bool, basicAuth bool, username string, password string, dialTimeout time.Duration) (*Client, error) {
	// Defaults already applied via ApplyTimeoutDefaults in the factory
	cfg := clientv3.Config{
		Endpoints:            machines,
		DialTimeout:          dialTimeout,
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 3 * time.Second,
	}

	if basicAuth {
		cfg.Username = username
		cfg.Password = password
	}

	tlsEnabled := false
	tlsConfig := &tls.Config{
		InsecureSkipVerify: clientInsecure,
	}

	if caCert != "" {
		certBytes, err := os.ReadFile(caCert)
		if err != nil {
			return &Client{}, fmt.Errorf("failed to read CA certificate file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM(certBytes)

		if ok {
			tlsConfig.RootCAs = caCertPool
		}
		tlsEnabled = true
	}

	if cert != "" && key != "" {
		tlsCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return &Client{}, fmt.Errorf("failed to load client certificate key pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
		tlsEnabled = true
	}

	if tlsEnabled {
		cfg.TLS = tlsConfig
	}

	client, err := clientv3.New(cfg)
	if err != nil {
		return &Client{}, fmt.Errorf("failed to create etcd client: %w", err)
	}

	return &Client{client: client, kvClient: client, watches: make(map[string]*Watch), wm: sync.Mutex{}}, nil
}

// GetValues queries etcd for keys prefixed by prefix.
func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	// Use all operations on the same revision
	var first_rev int64 = 0
	vars := make(map[string]string)
	// Default ETCDv3 TXN limitation. Since it is configurable from v3.3,
	// maybe an option should be added (also set max-txn=0 can disable Txn?)
	maxTxnOps := 128
	getOps := make([]string, 0, maxTxnOps)
	doTxn := func(ops []string) error {
		// Use passed context if it has a deadline, otherwise add a default timeout
		txnCtx := ctx
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			txnCtx, cancel = context.WithTimeout(ctx, time.Duration(3)*time.Second)
			defer cancel()
		}

		txnOps := make([]clientv3.Op, 0, maxTxnOps)

		for _, k := range ops {
			txnOps = append(txnOps, clientv3.OpGet(k,
				clientv3.WithPrefix(),
				clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend),
				clientv3.WithRev(first_rev)))
		}

		result, err := c.kvClient.Txn(txnCtx).Then(txnOps...).Commit()
		if err != nil {
			return err
		}
		for i, r := range result.Responses {
			originKey := ops[i]
			// append a '/' if not already exists
			originKeyFixed := originKey
			if !strings.HasSuffix(originKeyFixed, "/") {
				originKeyFixed = originKey + "/"
			}
			for _, ev := range r.GetResponseRange().Kvs {
				k := string(ev.Key)
				if k == originKey || strings.HasPrefix(k, originKeyFixed) {
					vars[string(ev.Key)] = string(ev.Value)
				}
			}
		}
		if first_rev == 0 {
			// Save the revison of the first request
			first_rev = result.Header.GetRevision()
		}
		return nil
	}
	for _, key := range keys {
		getOps = append(getOps, key)
		if len(getOps) >= maxTxnOps {
			if err := doTxn(getOps); err != nil {
				return vars, err
			}
			getOps = getOps[:0]
		}
	}
	if len(getOps) > 0 {
		if err := doTxn(getOps); err != nil {
			return vars, err
		}
	}
	return vars, nil
}

// HealthCheck verifies the backend connection is healthy.
// It checks the status of the first etcd endpoint.
func (c *Client) HealthCheck(ctx context.Context) error {
	start := time.Now()
	logger := log.With("backend", "etcd")

	endpoints := c.client.Endpoints()
	if len(endpoints) == 0 {
		duration := time.Since(start)
		logger.ErrorContext(ctx, "Backend health check failed (no endpoints configured)",
			"duration_ms", duration.Milliseconds())
		return fmt.Errorf("etcd: no endpoints configured")
	}

	_, err := c.client.Status(ctx, endpoints[0])

	duration := time.Since(start)
	if err != nil {
		logger.ErrorContext(ctx, "Backend health check failed",
			"duration_ms", duration.Milliseconds(),
			"endpoint", endpoints[0],
			"error", err.Error())
		return err
	}

	logger.InfoContext(ctx, "Backend health check passed",
		"duration_ms", duration.Milliseconds(),
		"endpoint", endpoints[0])
	return nil
}

// HealthCheckDetailed provides detailed health information for the etcd backend.
func (c *Client) HealthCheckDetailed(ctx context.Context) (*types.HealthResult, error) {
	start := time.Now()

	endpoints := c.client.Endpoints()
	if len(endpoints) == 0 {
		duration := time.Since(start)
		return &types.HealthResult{
			Healthy:   false,
			Message:   "etcd: no endpoints configured",
			Duration:  duration,
			CheckedAt: time.Now(),
			Details: map[string]string{
				"error": "no endpoints configured",
			},
		}, fmt.Errorf("etcd: no endpoints configured")
	}

	status, err := c.client.Status(ctx, endpoints[0])

	duration := time.Since(start)
	if err != nil {
		return &types.HealthResult{
			Healthy:   false,
			Message:   fmt.Sprintf("etcd health check failed: %s", err.Error()),
			Duration:  duration,
			CheckedAt: time.Now(),
			Details: map[string]string{
				"endpoint": endpoints[0],
				"error":    err.Error(),
			},
		}, err
	}

	return &types.HealthResult{
		Healthy:   true,
		Message:   "etcd backend is healthy",
		Duration:  duration,
		CheckedAt: time.Now(),
		Details: map[string]string{
			"endpoint":   endpoints[0],
			"version":    status.Version,
			"db_size":    fmt.Sprintf("%d", status.DbSize),
			"leader_id":  fmt.Sprintf("%d", status.Leader),
			"raft_index": fmt.Sprintf("%d", status.RaftIndex),
		},
	}, nil
}

func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	var err error

	// Create watch for each key
	watches := make(map[string]*Watch)
	c.wm.Lock()
	for _, k := range keys {
		watch, ok := c.watches[k]
		if !ok {
			watch, err = createWatch(c.client, k)
			if err != nil {
				c.wm.Unlock()
				return 0, err
			}
			c.watches[k] = watch
		}
		watches[k] = watch
	}
	c.wm.Unlock()

	// Derive cancellable context from passed context
	watchCtx, cancel := context.WithCancel(ctx)
	cancelRoutine := make(chan struct{})
	defer cancel()
	defer close(cancelRoutine)
	go func() {
		select {
		case <-stopChan:
			cancel()
		case <-cancelRoutine:
			return
		}
	}()

	notify := make(chan int64)
	// Wait for all watches
	for _, v := range watches {
		go v.WaitNext(watchCtx, int64(waitIndex), notify)
	}
	select {
	case nextRevision := <-notify:
		return uint64(nextRevision), err
	case <-watchCtx.Done():
		return 0, watchCtx.Err()
	}

}
