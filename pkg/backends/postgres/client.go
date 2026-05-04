package postgres

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Client is a wrapper around the pgxpool
type Client struct {
	pool    *pgxpool.Pool
	table   string
	connStr string // stored for LISTEN/NOTIFY dedicated connections
}

// New returns a new PostgreSQL client
func New(nodes []string, username, password, dbName, table string, dialTimeout time.Duration) (*Client, error) {
	node := "localhost:5432"
	if len(nodes) > 0 {
		node = nodes[0]
	}

	if dbName == "" {
		dbName = "confd"
	}

	if table == "" {
		table = "confd_config"
	}

	// Basic connection string
	var connStr string
	if username != "" {
		connStr = fmt.Sprintf("postgres://%s:%s@%s/%s", username, password, node, dbName)
	} else {
		connStr = fmt.Sprintf("postgres://%s/%s", node, dbName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	return &Client{
		pool:    pool,
		table:   table,
		connStr: connStr,
	}, nil
}

// GetValues queries postgres for keys
func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	vars := make(map[string]string)

	for _, key := range keys {
		// prefix search if we want to mimic etcd directory structure
		prefix := key
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		prefix += "%"

		query := fmt.Sprintf("SELECT key, value FROM %s WHERE key = $1 OR key LIKE $2", c.table)
		rows, err := c.pool.Query(ctx, query, key, prefix)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err != nil {
				rows.Close()
				return nil, err
			}
			vars[k] = v
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return vars, nil
}

// WatchPrefix uses PostgreSQL LISTEN/NOTIFY to wait for real-time changes.
// It opens a dedicated connection, subscribes to the "confd_update" channel,
// and blocks until a notification arrives or the stopChan is closed.
// This eliminates the need for polling entirely.
func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	// Acquire a dedicated connection from the pool (LISTEN requires a persistent conn)
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		log.Printf("[postgres] LISTEN/NOTIFY: failed to acquire connection: %v, falling back to 5s sleep", err)
		select {
		case <-time.After(5 * time.Second):
		case <-stopChan:
		}
		return waitIndex, nil
	}
	defer conn.Release()

	// Subscribe to the notification channel
	_, err = conn.Exec(ctx, "LISTEN confd_update")
	if err != nil {
		log.Printf("[postgres] LISTEN failed: %v, falling back to 5s sleep", err)
		select {
		case <-time.After(5 * time.Second):
		case <-stopChan:
		}
		return waitIndex, nil
	}

	log.Printf("[postgres] LISTEN/NOTIFY: waiting for events on channel 'confd_update'...")

	// Block until we receive a notification OR the stop signal
	for {
		select {
		case <-stopChan:
			return waitIndex, nil
		default:
		}

		// Wait up to 30 seconds for a notification, then loop to check stopChan
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		notification, err := conn.Conn().WaitForNotification(waitCtx)
		cancel()

		if err != nil {
			// Timeout — no notification received, loop again to check stopChan
			if waitCtx.Err() != nil {
				continue
			}
			// Real error
			log.Printf("[postgres] LISTEN/NOTIFY error: %v", err)
			return waitIndex, nil
		}

		log.Printf("[postgres] NOTIFY received: channel=%s payload=%s", notification.Channel, notification.Payload)
		return waitIndex + 1, nil
	}
}

// HealthCheck verifies the connection
func (c *Client) HealthCheck(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

// Close closes the connection pool
func (c *Client) Close() error {
	if c.pool != nil {
		c.pool.Close()
	}
	return nil
}
