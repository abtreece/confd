package env

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/backends/types"
	"github.com/abtreece/confd/pkg/log"
)

var replacer = strings.NewReplacer("/", "_")

// Client provides a shell for the env client
type Client struct{}

// NewEnvClient returns a new client
func NewEnvClient() (*Client, error) {
	return &Client{}, nil
}

// GetValues queries the environment for keys
func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	allEnvVars := os.Environ()
	envMap := make(map[string]string)
	for _, e := range allEnvVars {
		index := strings.Index(e, "=")
		envMap[e[:index]] = e[index+1:]
	}
	vars := make(map[string]string)
	for _, key := range keys {
		k := transform(key)
		for envKey, envValue := range envMap {
			if strings.HasPrefix(envKey, k) {
				vars[clean(envKey)] = envValue
			}
		}
	}

	log.Debug("Key Map: %#v", vars)

	return vars, nil
}

func transform(key string) string {
	k := strings.TrimPrefix(key, "/")
	return strings.ToUpper(replacer.Replace(k))
}

var cleanReplacer = strings.NewReplacer("_", "/")

func clean(key string) string {
	newKey := "/" + key
	return cleanReplacer.Replace(strings.ToLower(newKey))
}

func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	<-stopChan
	return 0, nil
}

// HealthCheck verifies the backend is healthy.
// Environment variables are always available, so this always returns nil.
func (c *Client) HealthCheck(ctx context.Context) error {
	start := time.Now()
	logger := log.With("backend", "env")

	duration := time.Since(start)
	logger.InfoContext(ctx, "Backend health check passed",
		"duration_ms", duration.Milliseconds())
	return nil
}

// HealthCheckDetailed provides detailed health information for the environment backend.
func (c *Client) HealthCheckDetailed(ctx context.Context) (*types.HealthResult, error) {
	start := time.Now()

	allEnvVars := os.Environ()
	envVarCount := len(allEnvVars)

	checkedAt := time.Now()
	duration := time.Since(start)

	return &types.HealthResult{
		Healthy:   true,
		Message:   "Environment backend is always healthy (no connectivity required)",
		Duration:  types.DurationMillis(duration),
		CheckedAt: checkedAt,
		Details: map[string]string{
			"env_var_count": fmt.Sprintf("%d", envVarCount),
			"note":          "No network connectivity required for environment backend",
		},
	}, nil
}

// Close is a no-op for this backend.
func (c *Client) Close() error {
	return nil
}
