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
	"time"

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
	client      *redis.Client
	machines    []string
	password    string
	separator   string
	db          int
	pubsub      *redis.PubSub
	pscChan     chan watchResponse
	retryConfig RetryConfig
}

// createClient attempts to connect to each machine in order with exponential backoff retry logic.
// Returns the first successful connection or an aggregated error from all attempts.
func createClient(machines []string, password string, withReadTimeout bool, retryConfig RetryConfig) (*redis.Client, int, error) {
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
				DialTimeout:  time.Second,
				WriteTimeout: time.Second,
			}

			if withReadTimeout {
				opts.ReadTimeout = time.Second
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
		client, db, err := createClient(c.machines, c.password, true, c.retryConfig)
		if err != nil {
			return nil, err
		}
		c.client = client
		c.db = db
	}

	return c.client, nil
}

// NewRedisClient returns an *redis.Client with a connection to named machines.
// It returns an error if a connection to the cluster cannot be made.
func NewRedisClient(machines []string, password string, separator string) (*Client, error) {
	if separator == "" {
		separator = "/"
	}
	log.Debug("Redis Separator: %#v", separator)

	retryConfig := DefaultRetryConfig()
	client, db, err := createClient(machines, password, true, retryConfig)
	if err != nil {
		return nil, err
	}

	return &Client{
		client:      client,
		machines:    machines,
		password:    password,
		separator:   separator,
		db:          db,
		pscChan:     make(chan watchResponse),
		retryConfig: retryConfig,
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
		return nil, err
	}

	vars := make(map[string]string)
	for _, key := range keys {
		key = strings.Replace(key, "/*", "", -1)

		k := c.transform(key)
		t, err := rClient.Type(ctx, k).Result()

		if err != nil {
			return vars, err
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

	go func() {
		if c.pubsub == nil {
			go c.watchWithReconnect(ctx, prefix)
		}
	}()

	select {
	case <-ctx.Done():
		if c.pubsub != nil {
			c.pubsub.Close()
			c.pubsub = nil
		}
		return waitIndex, ctx.Err()
	case <-stopChan:
		if c.pubsub != nil {
			c.pubsub.PUnsubscribe(ctx)
		}
		return waitIndex, nil
	case r := <-c.pscChan:
		return r.waitIndex, r.err
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
		if c.pubsub != nil {
			c.pubsub.Close()
			c.pubsub = nil
		}
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
		rClient, db, err = createClient(c.machines, c.password, false, c.retryConfig)
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
		c.pubsub = rClient.PSubscribe(ctx, pattern)
		ch := c.pubsub.Channel()
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
					if c.pubsub != nil {
						c.pubsub.Close()
						c.pubsub = nil
					}
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
