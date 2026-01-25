package redis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abtreece/confd/pkg/backends/types"
	"github.com/abtreece/confd/pkg/log"
	"github.com/redis/go-redis/v9"
)

// rng is a package-level seeded random number generator for jitter calculation.
// Using a local source ensures thread-safety and proper randomization.
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

type watchResponse struct {
	waitIndex uint64
	err       error
}

// RetryConfig contains configuration for connection retry behavior
type RetryConfig struct {
	MaxRetries   int           // Maximum number of retry attempts (0 = no retries)
	BaseDelay    time.Duration // Initial backoff delay
	MaxDelay     time.Duration // Maximum backoff delay
	JitterFactor float64       // Jitter factor (0.0-1.0) to prevent thundering herd
}

// DefaultRetryConfig returns sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		JitterFactor: 0.3, // 30% jitter
	}
}

// calculateBackoff calculates the backoff duration for a given attempt with exponential backoff and jitter.
// Implements: backoff = min(baseDelay * 2^attempt, maxDelay) * (1 ± jitter)
func calculateBackoff(attempt int, config RetryConfig) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	backoff := float64(config.BaseDelay) * math.Pow(2, float64(attempt))

	// Cap at maxDelay
	if backoff > float64(config.MaxDelay) {
		backoff = float64(config.MaxDelay)
	}

	// Add jitter: backoff * (1 ± jitterFactor)
	if config.JitterFactor > 0 {
		jitter := backoff * config.JitterFactor
		// Random value between (backoff - jitter) and (backoff + jitter)
		backoff = backoff - jitter + (rng.Float64() * 2 * jitter)
	}

	return time.Duration(backoff)
}

// Client is a wrapper around the redis client
type Client struct {
	client       *redis.Client
	machines     []string
	password     string
	separator    string
	db           int
	pubsub       *redis.PubSub
	pubsubMu     sync.Mutex // protects pubsub, watchCtx, and watchCancel fields
	watchCtx     context.Context
	watchCancel  context.CancelFunc
	pscChan      chan watchResponse
	retryConfig  RetryConfig
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// createClient attempts to connect to each machine in order with exponential backoff retry logic.
// Returns the first successful connection or an aggregated error from all attempts.
func createClient(machines []string, password string, withReadTimeout bool, retryConfig RetryConfig, dialTimeout, readTimeout, writeTimeout time.Duration) (*redis.Client, int, error) {
	var allErrors []error

	for _, address := range machines {
		db := 0

		// Parse database from address (e.g., "localhost:6379/4")
		idx := strings.Index(address, "/")
		if idx != -1 {
			var err error
			db, err = strconv.Atoi(address[idx+1:])
			if err == nil {
				address = address[:idx]
			}
		}

		// Detect Unix socket vs TCP
		network := "tcp"
		if _, err := os.Stat(address); err == nil {
			network = "unix"
		}

		// Try connecting to this machine with retries
		for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
			if attempt > 0 {
				backoff := calculateBackoff(attempt-1, retryConfig)
				log.Debug("Redis connection retry %d/%d to %s after backoff %v",
					attempt, retryConfig.MaxRetries, address, backoff)
				time.Sleep(backoff)
			} else {
				log.Debug("Trying to connect to redis node %s", address)
			}

			opts := &redis.Options{
				Network:      network,
				Addr:         address,
				Password:     password,
				DB:           db,
				DialTimeout:  dialTimeout,
				WriteTimeout: writeTimeout,
			}

			if withReadTimeout {
				opts.ReadTimeout = readTimeout
			}

			client := redis.NewClient(opts)

			// Test connection
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			err := client.Ping(ctx).Err()
			cancel()

			if err == nil {
				// Connection successful
				if attempt > 0 {
					log.Info("Successfully connected to redis node %s after %d retries", address, attempt)
				}
				return client, db, nil
			}

			// Connection failed
			client.Close()

			if attempt == retryConfig.MaxRetries {
				// Only add error after all retries exhausted for this machine
				allErrors = append(allErrors, fmt.Errorf("%s: %w", address, err))
				log.Debug("Failed to connect to redis node %s after %d attempts: %v", address, attempt+1, err)
			}
		}
	}

	// Return aggregated error with context from all machines
	if len(allErrors) > 0 {
		return nil, 0, fmt.Errorf("failed to connect to any redis node after retries: %w", errors.Join(allErrors...))
	}
	return nil, 0, fmt.Errorf("failed to connect to any redis node: no machines provided")
}

// connectedClient returns the redis client, reconnecting if necessary.
func (c *Client) connectedClient(ctx context.Context) (*redis.Client, error) {
	if c.client != nil {
		log.Debug("Testing existing redis connection.")

		err := c.client.Ping(ctx).Err()
		if err != nil {
			log.Error("Existing redis connection no longer usable. Will try to re-establish. Error: %s", err.Error())
			c.client.Close()
			c.client = nil
		}
	}

	// Existing client could have been deleted by previous block
	if c.client == nil {
		client, db, err := createClient(c.machines, c.password, true, c.retryConfig, c.dialTimeout, c.readTimeout, c.writeTimeout)
		if err != nil {
			return nil, fmt.Errorf("failed to reconnect redis client: %w", err)
		}
		c.client = client
		c.db = db
	}

	return c.client, nil
}

// NewRedisClient returns an *redis.Client with a connection to named machines.
// It returns an error if a connection to the cluster cannot be made.
func NewRedisClient(machines []string, password string, separator string, dialTimeout, readTimeout, writeTimeout time.Duration, retryMaxAttempts int, retryBaseDelay, retryMaxDelay time.Duration) (*Client, error) {
	if separator == "" {
		separator = "/"
	}
	log.Debug("Redis Separator: %#v", separator)

	// Build retry config from parameters (defaults already applied via ApplyTimeoutDefaults)
	retryConfig := RetryConfig{
		MaxRetries:   retryMaxAttempts,
		BaseDelay:    retryBaseDelay,
		MaxDelay:     retryMaxDelay,
		JitterFactor: 0.3, // 30% jitter
	}

	client, db, err := createClient(machines, password, true, retryConfig, dialTimeout, readTimeout, writeTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	return &Client{
		client:       client,
		machines:     machines,
		password:     password,
		separator:    separator,
		db:           db,
		pscChan:      make(chan watchResponse, 1),
		retryConfig:  retryConfig,
		dialTimeout:  dialTimeout,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}, nil
}

func (c *Client) transform(key string) string {
	if c.separator == "/" {
		return key
	}
	k := strings.TrimPrefix(key, "/")
	return strings.Replace(k, "/", c.separator, -1)
}

func (c *Client) clean(key string) string {
	k := key
	if !strings.HasPrefix(k, "/") {
		k = "/" + k
	}
	return strings.Replace(k, c.separator, "/", -1)
}

// GetValues queries redis for keys prefixed by prefix.
func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	// Ensure we have a connected redis client
	rClient, err := c.connectedClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connected redis client: %w", err)
	}

	vars := make(map[string]string)
	for _, key := range keys {
		key = strings.Replace(key, "/*", "", -1)

		k := c.transform(key)
		t, err := rClient.Type(ctx, k).Result()

		if err != nil {
			return vars, fmt.Errorf("failed to get redis key type for %s: %w", k, err)
		}

		switch t {
		case "string":
			value, err := rClient.Get(ctx, k).Result()
			if err == redis.Nil {
				continue
			}
			if err != nil {
				return vars, err
			}
			vars[key] = value

		case "hash":
			cursor := uint64(0)
			for {
				keys, nextCursor, err := rClient.HScan(ctx, k, cursor, "*", 1000).Result()
				if err != nil {
					return vars, err
				}

				// keys is alternating field, value pairs
				for i := 0; i < len(keys); i += 2 {
					field := keys[i]
					value := keys[i+1]
					vars[c.clean(k+"/"+field)] = value
				}

				cursor = nextCursor
				if cursor == 0 {
					break
				}
			}

		default:
			// Pattern matching with SCAN
			pattern := k
			if key == "/" {
				pattern = "*"
			} else {
				pattern = fmt.Sprintf("%s/*", c.transform(key))
			}

			cursor := uint64(0)
			for {
				scanKeys, nextCursor, err := rClient.Scan(ctx, cursor, pattern, 1000).Result()
				if err != nil {
					return vars, err
				}

				for _, scanKey := range scanKeys {
					value, err := rClient.Get(ctx, scanKey).Result()
					if err == redis.Nil {
						continue
					}
					if err != nil {
						return vars, err
					}
					vars[c.clean(scanKey)] = value
				}

				cursor = nextCursor
				if cursor == 0 {
					break
				}
			}
		}
	}

	log.Debug("Key Map: %#v", vars)

	return vars, nil
}

func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	if waitIndex == 0 {
		return 1, nil
	}

	if len(c.pscChan) > 0 {
		var respChan watchResponse
		for len(c.pscChan) > 0 {
			respChan = <-c.pscChan
		}
		return respChan.waitIndex, respChan.err
	}

	// Start long-lived watch goroutine if not already running
	c.pubsubMu.Lock()
	if c.pubsub == nil && c.watchCtx == nil {
		// Create a long-lived context for the watcher that persists between calls
		c.watchCtx, c.watchCancel = context.WithCancel(context.Background())
		go c.watchWithReconnect(c.watchCtx, prefix)
	}
	c.pubsubMu.Unlock()

	// Wait for either: parent context done, stopChan signal, or watch response
	select {
	case <-ctx.Done():
		// Parent context cancelled - shut down the watcher
		c.stopWatch()
		return waitIndex, ctx.Err()
	case <-stopChan:
		// Stop signal received - shut down the watcher
		c.stopWatch()
		return waitIndex, nil
	case r := <-c.pscChan:
		// Watch response received - return without stopping the watcher
		return r.waitIndex, r.err
	}
}

// stopWatch cancels the long-lived watch goroutine and cleans up resources.
func (c *Client) stopWatch() {
	c.pubsubMu.Lock()
	defer c.pubsubMu.Unlock()

	if c.watchCancel != nil {
		c.watchCancel()
		c.watchCancel = nil
		c.watchCtx = nil
	}
	if c.pubsub != nil {
		c.pubsub.Close()
		c.pubsub = nil
	}
}

// watchWithReconnect manages the PubSub connection lifecycle with automatic reconnection.
// It attempts to maintain a persistent watch on Redis keyspace notifications, reconnecting
// with exponential backoff if the connection is lost.
func (c *Client) watchWithReconnect(ctx context.Context, prefix string) {
	var rClient *redis.Client
	var db int
	attempt := 0

	commands := map[string]bool{
		"del": true, "append": true, "rename_from": true, "rename_to": true,
		"expire": true, "set": true, "incrby": true, "incrbyfloat": true,
		"hset": true, "hincrby": true, "hincrbyfloat": true, "hdel": true,
	}

	defer func() {
		c.pubsubMu.Lock()
		if c.pubsub != nil {
			c.pubsub.Close()
			c.pubsub = nil
		}
		// Clear watchCtx/watchCancel so a new watcher can be started if needed.
		// Only clear if they still refer to this watcher's context to avoid
		// racing with stopWatch() or a concurrently started new watcher.
		if c.watchCtx == ctx {
			c.watchCtx = nil
			c.watchCancel = nil
		}
		c.pubsubMu.Unlock()
		if rClient != nil {
			rClient.Close()
		}
	}()

	for {
		// Check if context is cancelled before attempting connection
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Apply backoff delay for reconnection attempts
		if attempt > 0 {
			backoff := calculateBackoff(attempt-1, c.retryConfig)
			log.Info("Redis PubSub reconnection attempt %d/%d after %v", attempt, c.retryConfig.MaxRetries+1, backoff)
			time.Sleep(backoff)
		}

		// Create a new client for PubSub (without read timeout for blocking)
		var err error
		rClient, db, err = createClient(c.machines, c.password, false, c.retryConfig, c.dialTimeout, c.readTimeout, c.writeTimeout)
		if err != nil {
			attempt++
			if attempt > c.retryConfig.MaxRetries {
				log.Error("Redis PubSub connection failed after %d attempts: %v", attempt, err)
				select {
				case c.pscChan <- watchResponse{0, fmt.Errorf("PubSub connection failed after %d attempts: %w", attempt, err)}:
				case <-ctx.Done():
				}
				return
			}
			log.Warning("Redis PubSub connection attempt %d failed: %v", attempt, err)
			continue
		}

		// Successful connection - reset attempt counter
		if attempt > 0 {
			log.Info("Redis PubSub reconnected successfully after %d attempts", attempt)
		}
		attempt = 0

		// Subscribe to keyspace notifications pattern
		pattern := "__keyspace@" + strconv.Itoa(db) + "__:" + c.transform(prefix) + "*"
		c.pubsubMu.Lock()
		c.pubsub = rClient.PSubscribe(ctx, pattern)
		ch := c.pubsub.Channel()
		c.pubsubMu.Unlock()
		log.Debug("Redis PubSub subscribed to pattern: %s", pattern)

		// Process messages until channel closes or context is cancelled
		channelClosed := false
		for !channelClosed {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					// Channel closed - connection lost, will reconnect
					log.Warning("Redis PubSub channel closed, attempting reconnection")
					channelClosed = true

					// Clean up current connection before reconnecting
					c.pubsubMu.Lock()
					if c.pubsub != nil {
						c.pubsub.Close()
						c.pubsub = nil
					}
					c.pubsubMu.Unlock()
					if rClient != nil {
						rClient.Close()
						rClient = nil
					}

					attempt = 1 // Start reconnection attempts
					break
				}
				log.Debug("Redis Message: %s %s", msg.Channel, msg.Payload)
				if commands[msg.Payload] {
					select {
					case c.pscChan <- watchResponse{1, nil}:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}
}

// HealthCheck verifies the backend connection is healthy.
// It attempts to connect and ping the Redis server.
func (c *Client) HealthCheck(ctx context.Context) error {
	start := time.Now()
	logger := log.With("backend", "redis")

	_, err := c.connectedClient(ctx)

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

// HealthCheckDetailed provides detailed health information for the redis backend.
func (c *Client) HealthCheckDetailed(ctx context.Context) (*types.HealthResult, error) {
	start := time.Now()

	client, err := c.connectedClient(ctx)
	if err != nil {
		duration := time.Since(start)
		return &types.HealthResult{
			Healthy:   false,
			Message:   fmt.Sprintf("Redis health check failed: %s", err.Error()),
			Duration:  types.DurationMillis(duration),
			CheckedAt: time.Now(),
			Details: map[string]string{
				"error": err.Error(),
			},
		}, err
	}

	// Get connection pool stats
	stats := client.PoolStats()

	// Get server info
	info, err := client.Info(ctx, "server").Result()
	version := "unknown"
	if err == nil {
		// Parse version from info string (format: "redis_version:x.x.x")
		for _, line := range strings.Split(info, "\n") {
			if strings.HasPrefix(line, "redis_version:") {
				version = strings.TrimSpace(strings.TrimPrefix(line, "redis_version:"))
				break
			}
		}
	}

	duration := time.Since(start)

	return &types.HealthResult{
		Healthy:   true,
		Message:   "Redis backend is healthy",
		Duration:  types.DurationMillis(duration),
		CheckedAt: time.Now(),
		Details: map[string]string{
			"version":        version,
			"hits":           fmt.Sprintf("%d", stats.Hits),
			"misses":         fmt.Sprintf("%d", stats.Misses),
			"timeouts":       fmt.Sprintf("%d", stats.Timeouts),
			"total_conns":    fmt.Sprintf("%d", stats.TotalConns),
			"idle_conns":     fmt.Sprintf("%d", stats.IdleConns),
			"stale_conns":    fmt.Sprintf("%d", stats.StaleConns),
		},
	}, nil
}

// Close closes the Redis client connections.
func (c *Client) Close() error {
	// Stop the watch goroutine and clean up pubsub
	c.stopWatch()

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			return fmt.Errorf("failed to close Redis client: %w", err)
		}
		c.client = nil
	}

	return nil
}
