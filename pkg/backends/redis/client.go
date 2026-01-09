package redis

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/redis/go-redis/v9"
)

type watchResponse struct {
	waitIndex uint64
	err       error
}

// Client is a wrapper around the redis client
type Client struct {
	client    *redis.Client
	machines  []string
	password  string
	separator string
	db        int
	pubsub    *redis.PubSub
	pscChan   chan watchResponse
}

// createClient attempts to connect to each machine in order.
// Returns the first successful connection or the last error encountered.
func createClient(machines []string, password string, withReadTimeout bool) (*redis.Client, int, error) {
	var lastErr error
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
		log.Debug("Trying to connect to redis node %s", address)

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

		if err != nil {
			client.Close()
			lastErr = err
			continue
		}

		return client, db, nil
	}

	return nil, 0, lastErr
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
		client, db, err := createClient(c.machines, c.password, true)
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

	client, db, err := createClient(machines, password, true)
	if err != nil {
		return nil, err
	}

	return &Client{
		client:    client,
		machines:  machines,
		password:  password,
		separator: separator,
		db:        db,
		pscChan:   make(chan watchResponse),
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
			// Create a new client for PubSub (without read timeout for blocking)
			rClient, db, err := createClient(c.machines, c.password, false)
			if err != nil {
				c.pubsub = nil
				select {
				case c.pscChan <- watchResponse{0, err}:
				case <-ctx.Done():
				}
				return
			}

			pattern := "__keyspace@" + strconv.Itoa(db) + "__:" + c.transform(prefix) + "*"
			c.pubsub = rClient.PSubscribe(ctx, pattern)

			go func() {
				defer func() {
					if c.pubsub != nil {
						c.pubsub.Close()
						c.pubsub = nil
					}
					rClient.Close()
				}()

				ch := c.pubsub.Channel()
				commands := map[string]bool{
					"del": true, "append": true, "rename_from": true, "rename_to": true,
					"expire": true, "set": true, "incrby": true, "incrbyfloat": true,
					"hset": true, "hincrby": true, "hincrbyfloat": true, "hdel": true,
				}

				for {
					select {
					case <-ctx.Done():
						return
					case msg, ok := <-ch:
						if !ok {
							// Channel closed - subscription ended
							select {
							case c.pscChan <- watchResponse{0, nil}:
							case <-ctx.Done():
							}
							return
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
			}()
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

// HealthCheck verifies the backend connection is healthy.
// It attempts to connect and ping the Redis server.
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.connectedClient(ctx)
	return err
}
