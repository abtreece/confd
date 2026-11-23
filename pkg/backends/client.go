package backends

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/abtreece/confd/pkg/backends/acm"
	"github.com/abtreece/confd/pkg/backends/consul"
	"github.com/abtreece/confd/pkg/backends/dynamodb"
	"github.com/abtreece/confd/pkg/backends/env"
	"github.com/abtreece/confd/pkg/backends/etcd"
	"github.com/abtreece/confd/pkg/backends/file"
	"github.com/abtreece/confd/pkg/backends/imds"
	"github.com/abtreece/confd/pkg/backends/redis"
	"github.com/abtreece/confd/pkg/backends/secretsmanager"
	"github.com/abtreece/confd/pkg/backends/ssm"
	"github.com/abtreece/confd/pkg/backends/types"
	"github.com/abtreece/confd/pkg/backends/vault"
	"github.com/abtreece/confd/pkg/backends/zookeeper"
	"github.com/abtreece/confd/pkg/log"
)

// The StoreClient interface is implemented by objects that can retrieve
// key/value pairs from a backend store. Backends that can also watch for
// changes implement Watcher.
type StoreClient interface {
	GetValues(ctx context.Context, keys []string) (map[string]string, error)
	// HealthCheck verifies the backend connection is healthy.
	// Returns nil if the connection is healthy, otherwise returns an error.
	HealthCheck(ctx context.Context) error
	// Close releases any resources held by the client.
	// It should be called when the client is no longer needed.
	Close() error
}

// The Watcher interface is implemented by store clients whose backend can
// watch for changes. Watch mode requires it; polling-only backends do not
// implement it.
type Watcher interface {
	// WatchPrefix blocks until a key under prefix changes, stopChan fires or
	// ctx is done, and returns the index to pass as waitIndex on the next call.
	WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error)
}

// Re-export types for convenience
type HealthResult = types.HealthResult
type DetailedHealthChecker = types.DetailedHealthChecker

// New is used to create a storage client based on our configuration.
// Backends that perform I/O during initialization abort when ctx is done.
func New(ctx context.Context, config Config) (StoreClient, error) {

	if config.Backend == "" {
		config.Backend = "etcd"
	}

	// Apply default timeout values if not set
	config.ApplyTimeoutDefaults()

	backendNodes := config.BackendNodes

	var client StoreClient
	var err error
	switch config.Backend {
	case "acm":
		log.Info("Backend source(s) set to AWS ACM")
		client, err = acm.New(ctx, config.ACMExportPrivateKey, config.DialTimeout)
	case "consul":
		log.Info("Backend source(s) set to %s", strings.Join(backendNodes, ", "))
		client, err = consul.New(config.BackendNodes, config.Scheme,
			config.ClientCert, config.ClientKey,
			config.ClientCaKeys,
			config.BasicAuth,
			config.Username,
			config.Password,
			config.AuthToken,
		)
	case "etcd":
		log.Info("Backend source(s) set to %s", strings.Join(backendNodes, ", "))
		client, err = etcd.NewEtcdClient(backendNodes, config.ClientCert, config.ClientKey, config.ClientCaKeys, config.ClientInsecure, config.BasicAuth, config.Username, config.Password, config.DialTimeout)
	case "zookeeper":
		log.Info("Backend source(s) set to %s", strings.Join(backendNodes, ", "))
		client, err = zookeeper.NewZookeeperClient(backendNodes, config.DialTimeout)
	case "redis":
		log.Info("Backend source(s) set to %s", strings.Join(backendNodes, ", "))
		// The client key is the legacy way to supply the redis password.
		redisPassword := config.Password
		if redisPassword == "" {
			redisPassword = config.ClientKey
		}
		client, err = redis.NewRedisClient(backendNodes, redisPassword, config.Separator,
			config.DialTimeout, config.ReadTimeout, config.WriteTimeout,
			config.RetryMaxAttempts, config.RetryBaseDelay, config.RetryMaxDelay)
	case "env":
		client, err = env.NewEnvClient()
	case "file":
		log.Info("Backend source(s) set to %s", strings.Join(config.YAMLFile, ", "))
		client, err = file.NewFileClient(config.YAMLFile, config.Filter)
	case "vault":
		if len(backendNodes) == 0 {
			return nil, errors.New("vault backend requires at least one node address")
		}
		log.Info("Backend source(s) set to %s", strings.Join(backendNodes, ", "))
		vaultConfig := map[string]string{
			"app-id":    config.AppID,
			"user-id":   config.UserID,
			"role-id":   config.RoleID,
			"secret-id": config.SecretID,
			"username":  config.Username,
			"password":  config.Password,
			"token":     config.AuthToken,
			"cert":      config.ClientCert,
			"key":       config.ClientKey,
			"caCert":    config.ClientCaKeys,
			"path":      config.Path,
		}
		client, err = vault.New(backendNodes[0], config.AuthType, vaultConfig)
	case "dynamodb":
		table := config.Table
		log.Info("DynamoDB table set to %s", table)
		client, err = dynamodb.NewDynamoDBClient(ctx, table, config.DialTimeout)
	case "ssm":
		client, err = ssm.New(ctx, config.DialTimeout)
	case "imds":
		log.Info("Backend source(s) set to AWS EC2 IMDS")
		client, err = imds.New(ctx, config.IMDSCacheTTL, config.DialTimeout)
	case "secretsmanager":
		log.Info("Backend source(s) set to AWS Secrets Manager")
		client, err = secretsmanager.New(ctx, config.SecretsManagerVersionStage, config.SecretsManagerNoFlatten, config.DialTimeout)
	default:
		return nil, errors.New("invalid backend")
	}
	if err != nil {
		return nil, fmt.Errorf("initialize %s backend: %w", config.Backend, err)
	}
	return client, nil
}
