package zookeeper

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/backends/types"
	"github.com/abtreece/confd/pkg/log"
	zk "github.com/go-zookeeper/zk"
)

// zkConn defines the interface for Zookeeper connection operations.
// This allows for mocking in tests.
type zkConn interface {
	Children(path string) ([]string, *zk.Stat, error)
	Get(path string) ([]byte, *zk.Stat, error)
	Exists(path string) (bool, *zk.Stat, error)
	GetW(path string) ([]byte, *zk.Stat, <-chan zk.Event, error)
	ChildrenW(path string) ([]string, *zk.Stat, <-chan zk.Event, error)
}

// Client provides a wrapper around the zookeeper client
type Client struct {
	client zkConn
	conn   *zk.Conn // Full connection for health checks
}

func NewZookeeperClient(machines []string, dialTimeout time.Duration) (*Client, error) {
	// Defaults already applied via ApplyTimeoutDefaults in the factory
	c, _, err := zk.Connect(machines, dialTimeout)
	if err != nil {
		return nil, err
	}
	return &Client{
		client: c,
		conn:   c,
	}, nil
}

// existsCtx wraps Exists with context cancellation support.
// go-zookeeper does not expose context-aware methods, so we run the blocking
// call in a goroutine and select on ctx.Done(). The buffered channel ensures
// the goroutine can exit cleanly even if the caller has already returned.
func existsCtx(ctx context.Context, conn zkConn, path string) (bool, *zk.Stat, error) {
	type result struct {
		ok   bool
		stat *zk.Stat
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		ok, stat, err := conn.Exists(path)
		ch <- result{ok, stat, err}
	}()
	select {
	case <-ctx.Done():
		return false, nil, ctx.Err()
	case r := <-ch:
		return r.ok, r.stat, r.err
	}
}

// childrenCtx wraps Children with context cancellation support.
func childrenCtx(ctx context.Context, conn zkConn, path string) ([]string, *zk.Stat, error) {
	type result struct {
		children []string
		stat     *zk.Stat
		err      error
	}
	ch := make(chan result, 1)
	go func() {
		children, stat, err := conn.Children(path)
		ch <- result{children, stat, err}
	}()
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case r := <-ch:
		return r.children, r.stat, r.err
	}
}

// getCtx wraps Get with context cancellation support.
func getCtx(ctx context.Context, conn zkConn, path string) ([]byte, *zk.Stat, error) {
	type result struct {
		data []byte
		stat *zk.Stat
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		data, stat, err := conn.Get(path)
		ch <- result{data, stat, err}
	}()
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case r := <-ch:
		return r.data, r.stat, r.err
	}
}

func nodeWalk(ctx context.Context, prefix string, c *Client, vars map[string]string) error {
	var s string
	l, stat, err := childrenCtx(ctx, c.client, prefix)
	if err != nil {
		return err
	}

	if stat.NumChildren == 0 {
		b, _, err := getCtx(ctx, c.client, prefix)
		if err != nil {
			return err
		}
		vars[prefix] = string(b)

	} else {
		for _, key := range l {
			if prefix == "/" {
				s = "/" + key
			} else {
				s = prefix + "/" + key
			}
			_, stat, err := existsCtx(ctx, c.client, s)
			if err != nil {
				return err
			}
			if stat.NumChildren == 0 {
				b, _, err := getCtx(ctx, c.client, s)
				if err != nil {
					return err
				}
				vars[s] = string(b)
			} else {
				if err := nodeWalk(ctx, s, c, vars); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	vars := make(map[string]string)
	for _, v := range keys {
		v = strings.Replace(v, "/*", "", -1)
		_, _, err := existsCtx(ctx, c.client, v)
		if err != nil {
			return vars, err
		}
		err = nodeWalk(ctx, v, c, vars)
		if err != nil {
			return vars, err
		}
	}
	return vars, nil
}

type watchResponse struct {
	waitIndex uint64
	err       error
}

func (c *Client) watch(key string, respChan chan watchResponse, cancelRoutine chan bool) {
	_, _, keyEventCh, err := c.client.GetW(key)
	if err != nil {
		select {
		case respChan <- watchResponse{0, err}:
		case <-cancelRoutine:
		}
		return
	}
	_, _, childEventCh, err := c.client.ChildrenW(key)
	if err != nil {
		select {
		case respChan <- watchResponse{0, err}:
		case <-cancelRoutine:
		}
		return
	}

	for {
		select {
		case e := <-keyEventCh:
			if e.Type == zk.EventNodeDataChanged {
				select {
				case respChan <- watchResponse{1, e.Err}:
				case <-cancelRoutine:
					log.Debug("Stop watching: %s", key)
					return
				}
			}
		case e := <-childEventCh:
			if e.Type == zk.EventNodeChildrenChanged {
				select {
				case respChan <- watchResponse{1, e.Err}:
				case <-cancelRoutine:
					log.Debug("Stop watching: %s", key)
					return
				}
			}
		case <-cancelRoutine:
			log.Debug("Stop watching: %s", key)
			// There is no way to stop GetW/ChildrenW so just quit
			return
		}
	}
}

func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	// return something > 0 to trigger a key retrieval from the store
	if waitIndex == 0 {
		return 1, nil
	}

	// List the childrens first
	entries, err := c.GetValues(ctx, []string{prefix})
	if err != nil {
		return 0, err
	}

	respChan := make(chan watchResponse)
	cancelRoutine := make(chan bool)
	defer close(cancelRoutine)

	//watch all subfolders for changes
	watchMap := make(map[string]string)
	for k, _ := range entries {
		for _, v := range keys {
			if strings.HasPrefix(k, v) {
				for dir := filepath.Dir(k); dir != "/"; dir = filepath.Dir(dir) {
					if _, ok := watchMap[dir]; !ok {
						watchMap[dir] = ""
						log.Debug("Watching: %s", dir)
						go c.watch(dir, respChan, cancelRoutine)
					}
				}
				break
			}
		}
	}

	//watch all keys in prefix for changes
	for k, _ := range entries {
		for _, v := range keys {
			if strings.HasPrefix(k, v) {
				log.Debug("Watching: %s", k)
				go c.watch(k, respChan, cancelRoutine)
				break
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return waitIndex, ctx.Err()
		case <-stopChan:
			return waitIndex, nil
		case r := <-respChan:
			return r.waitIndex, r.err
		}
	}
}

// HealthCheck verifies the backend connection is healthy.
// It checks that the root path exists in Zookeeper.
func (c *Client) HealthCheck(ctx context.Context) error {
	start := time.Now()
	logger := log.With("backend", "zookeeper")

	_, _, err := c.client.Exists("/")

	duration := time.Since(start)
	if err != nil {
		logger.ErrorContext(ctx, "Backend health check failed",
			"duration_ms", duration.Milliseconds(),
			"error", err.Error())
		return err
	}

	logger.InfoContext(ctx, "Backend health check passed",
		"duration_ms", duration.Milliseconds())
	return nil
}

// HealthCheckDetailed provides detailed health information for the zookeeper backend.
func (c *Client) HealthCheckDetailed(ctx context.Context) (*types.HealthResult, error) {
	start := time.Now()

	_, _, err := c.client.Exists("/")

	if err != nil {
		duration := time.Since(start)
		return &types.HealthResult{
			Healthy:   false,
			Message:   fmt.Sprintf("Zookeeper health check failed: %s", err.Error()),
			Duration:  types.DurationMillis(duration),
			CheckedAt: time.Now(),
			Details: map[string]string{
				"error": err.Error(),
			},
		}, err
	}

	// Get session info
	sessionID := c.conn.SessionID()
	state := c.conn.State()

	duration := time.Since(start)

	return &types.HealthResult{
		Healthy:   true,
		Message:   "Zookeeper backend is healthy",
		Duration:  types.DurationMillis(duration),
		CheckedAt: time.Now(),
		Details: map[string]string{
			"session_id":    fmt.Sprintf("%d", sessionID),
			"session_state": state.String(),
		},
	}, nil
}

// Close closes the Zookeeper client connection.
func (c *Client) Close() error {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.client = nil
	}
	return nil
}
