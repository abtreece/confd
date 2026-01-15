package consul

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/backends/types"
	"github.com/abtreece/confd/pkg/log"
	"github.com/hashicorp/consul/api"
)

// consulKVAPI defines the interface for Consul KV operations used by this client.
// This allows for easy mocking in tests.
type consulKVAPI interface {
	List(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error)
}

// ConsulClient provides a wrapper around the consulkv client
type ConsulClient struct {
	client    consulKVAPI
	apiClient *api.Client // Full API client for health checks
}

// NewConsulClient returns a new client to Consul for the given address
func New(nodes []string, scheme, cert, key, caCert string, basicAuth bool, username string, password string) (*ConsulClient, error) {
	start := time.Now()
	logger := log.With("backend", "consul")

	var address string
	if len(nodes) > 0 {
		address = nodes[0]
	} else {
		address = "127.0.0.1:8500"
	}

	logger.InfoContext(context.Background(), "Initializing Consul client",
		"address", address,
		"scheme", scheme,
		"tls_enabled", cert != "" && key != "",
		"basic_auth", basicAuth)

	conf := api.DefaultConfig()

	conf.Scheme = scheme

	if len(nodes) > 0 {
		conf.Address = nodes[0]
	}

	if basicAuth {
		conf.HttpAuth = &api.HttpBasicAuth{
			Username: username,
			Password: password,
		}
	}

	if cert != "" && key != "" {
		conf.TLSConfig.CertFile = cert
		conf.TLSConfig.KeyFile = key
	}
	if caCert != "" {
		conf.TLSConfig.CAFile = caCert
	}

	client, err := api.NewClient(conf)

	duration := time.Since(start)
	if err != nil {
		logger.ErrorContext(context.Background(), "Failed to initialize Consul client",
			"duration_ms", duration.Milliseconds(),
			"error", err.Error())
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	logger.InfoContext(context.Background(), "Successfully initialized Consul client",
		"duration_ms", duration.Milliseconds())
	return &ConsulClient{
		client:    client.KV(),
		apiClient: client,
	}, nil
}

// GetValues queries Consul for keys
func (c *ConsulClient) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	start := time.Now()
	logger := log.With("backend", "consul", "key_count", len(keys))
	logger.DebugContext(ctx, "Fetching values from Consul")

	vars := make(map[string]string)
	opts := &api.QueryOptions{}
	opts = opts.WithContext(ctx)
	for _, key := range keys {
		key := strings.TrimPrefix(key, "/")
		logger.DebugContext(ctx, "Listing key from Consul", "key", key)

		pairs, _, err := c.client.List(key, opts)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to fetch values",
				"duration_ms", time.Since(start).Milliseconds(),
				"key", key,
				"error", err.Error())
			return vars, err
		}

		logger.DebugContext(ctx, "Retrieved key pairs", "key", key, "pair_count", len(pairs))
		for _, p := range pairs {
			vars[path.Join("/", p.Key)] = string(p.Value)
		}
	}

	logger.InfoContext(ctx, "Successfully fetched values",
		"value_count", len(vars),
		"duration_ms", time.Since(start).Milliseconds())
	return vars, nil
}

type watchResponse struct {
	waitIndex uint64
	err       error
}

func (c *ConsulClient) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	logger := log.With("backend", "consul", "prefix", prefix, "wait_index", waitIndex)
	logger.DebugContext(ctx, "Starting watch on prefix")

	respChan := make(chan watchResponse)
	go func() {
		watchStart := time.Now()
		opts := &api.QueryOptions{
			WaitIndex: waitIndex,
		}
		opts = opts.WithContext(ctx)

		_, meta, err := c.client.List(prefix, opts)
		watchDuration := time.Since(watchStart)

		if err != nil {
			logger.ErrorContext(ctx, "Watch query failed",
				"duration_ms", watchDuration.Milliseconds(),
				"error", err.Error())
			respChan <- watchResponse{waitIndex, err}
			return
		}

		if meta.LastIndex != waitIndex {
			logger.InfoContext(ctx, "Watch detected index change",
				"old_index", waitIndex,
				"new_index", meta.LastIndex,
				"duration_ms", watchDuration.Milliseconds())
		} else {
			logger.DebugContext(ctx, "Watch completed with no index change",
				"duration_ms", watchDuration.Milliseconds())
		}

		respChan <- watchResponse{meta.LastIndex, err}
	}()

	select {
	case <-ctx.Done():
		logger.DebugContext(ctx, "Watch cancelled by context")
		return waitIndex, ctx.Err()
	case <-stopChan:
		logger.DebugContext(ctx, "Watch stopped by stop channel")
		return waitIndex, nil
	case r := <-respChan:
		return r.waitIndex, r.err
	}
}

// HealthCheck verifies the backend connection is healthy.
// It attempts a simple list operation to verify connectivity.
func (c *ConsulClient) HealthCheck(ctx context.Context) error {
	start := time.Now()
	logger := log.With("backend", "consul")

	opts := &api.QueryOptions{}
	opts = opts.WithContext(ctx)
	_, _, err := c.client.List("", opts)

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

// HealthCheckDetailed provides detailed health information for the consul backend.
func (c *ConsulClient) HealthCheckDetailed(ctx context.Context) (*types.HealthResult, error) {
	start := time.Now()

	opts := &api.QueryOptions{}
	opts = opts.WithContext(ctx)
	_, _, err := c.client.List("", opts)

	if err != nil {
		duration := time.Since(start)
		return &types.HealthResult{
			Healthy:   false,
			Message:   fmt.Sprintf("Consul health check failed: %s", err.Error()),
			Duration:  types.DurationMillis(duration),
			CheckedAt: time.Now(),
			Details: map[string]string{
				"error": err.Error(),
			},
		}, err
	}

	// Get agent info
	agent := c.apiClient.Agent()
	self, err := agent.Self()

	datacenter := "unknown"
	if err == nil {
		if cfg, cfgOk := self["Config"]; cfgOk {
			if dc, dcOk := cfg["Datacenter"]; dcOk {
				datacenter = fmt.Sprintf("%v", dc)
			}
		}
	}

	// Get leader info
	status := c.apiClient.Status()
	leader, err := status.Leader()
	if err != nil {
		leader = "unknown"
	}

	duration := time.Since(start)

	return &types.HealthResult{
		Healthy:   true,
		Message:   "Consul backend is healthy",
		Duration:  types.DurationMillis(duration),
		CheckedAt: time.Now(),
		Details: map[string]string{
			"datacenter": datacenter,
			"leader":     leader,
		},
	}, nil
}

// Close is a no-op for this backend.
func (c *ConsulClient) Close() error {
	return nil
}
