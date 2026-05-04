package plugin

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/abtreece/confd/pkg/backends/plugin/api"
	"github.com/hashicorp/go-plugin"
)

// Client is a wrapper around the go-plugin RPC client that implements backends.StoreClient
type Client struct {
	client     *plugin.Client
	rpcClient  api.BackendProvider
	pluginPath string
	mu         sync.Mutex
}

// New returns a new plugin client that launches the external binary
func New(pluginPath string) (*Client, error) {
	if pluginPath == "" {
		return nil, fmt.Errorf("plugin-path is required for plugin backend")
	}

	// Launch the external binary as a subprocess
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: api.Handshake,
		Plugins: map[string]plugin.Plugin{
			"backend": &api.ConfdBackendPlugin{},
		},
		Cmd: exec.Command(pluginPath),
	})

	// Connect via RPC
	rpcClientRaw, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to start plugin subprocess: %w", err)
	}

	// Request the plugin implementation
	raw, err := rpcClientRaw.Dispense("backend")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense plugin implementation: %w", err)
	}

	return &Client{
		client:     client,
		rpcClient:  raw.(api.BackendProvider),
		pluginPath: pluginPath,
	}, nil
}

// GetValues queries the external plugin for keys
func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rpcClient.GetValues(keys)
}

// WatchPrefix is proxied to the plugin
func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Note: We ignore stopChan here because net/rpc doesn't easily support channel passing.
	// The plugin is expected to return 0 to force polling, or handle its own blocking logic internally.
	// We check if stopChan is closed before doing the call.
	select {
	case <-stopChan:
		return 0, nil
	default:
	}

	return c.rpcClient.WatchPrefix(prefix, keys, waitIndex)
}

// HealthCheck verifies the plugin is healthy
func (c *Client) HealthCheck(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rpcClient.HealthCheck()
}

// Close kills the plugin subprocess
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.rpcClient.Close()
	c.client.Kill()
	return err
}
