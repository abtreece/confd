package backends

import (
	"context"
	"errors"
	"strings"

	"github.com/abtreece/confd/pkg/backends/acm"
	"github.com/abtreece/confd/pkg/backends/consul"
	"github.com/abtreece/confd/pkg/backends/dynamodb"
	"github.com/abtreece/confd/pkg/backends/env"
	"github.com/abtreece/confd/pkg/backends/etcd"
	"github.com/abtreece/confd/pkg/backends/file"
	"github.com/abtreece/confd/pkg/backends/redis"
	"github.com/abtreece/confd/pkg/backends/secretsmanager"
	"github.com/abtreece/confd/pkg/backends/ssm"
	"github.com/abtreece/confd/pkg/backends/vault"
	"github.com/abtreece/confd/pkg/backends/zookeeper"
	"github.com/abtreece/confd/pkg/log"
)

// The StoreClient interface is implemented by objects that can retrieve
// key/value pairs from a backend store.
type StoreClient interface {
	GetValues(ctx context.Context, keys []string) (map[string]string, error)
	WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error)
	// HealthCheck verifies the backend connection is healthy.
	// Returns nil if the connection is healthy, otherwise returns an error.
	HealthCheck(ctx context.Context) error
}

// New is used to create a storage client based on our configuration.
func New(config Config) (StoreClient, error) {

	if config.Backend == "" {
		config.Backend = "etcd"
	}
	backendNodes := config.BackendNodes

	var client StoreClient
	var err error

	switch config.Backend {
	case "acm":
		log.Info("Backend source(s) set to AWS ACM")
		client, err = acm.New(config.ACMExportPrivateKey)
	case "consul":
		log.Info("Backend source(s) set to %s", strings.Join(backendNodes, ", "))
		client, err = consul.New(config.BackendNodes, config.Scheme,
			config.ClientCert, config.ClientKey,
			config.ClientCaKeys,
			config.BasicAuth,
			config.Username,
			config.Password,
		)
	case "etcd":
		log.Info("Backend source(s) set to %s", strings.Join(backendNodes, ", "))
		client, err = etcd.NewEtcdClient(backendNodes, config.ClientCert, config.ClientKey, config.ClientCaKeys, config.ClientInsecure, config.BasicAuth, config.Username, config.Password)
	case "zookeeper":
		log.Info("Backend source(s) set to %s", strings.Join(backendNodes, ", "))
		client, err = zookeeper.NewZookeeperClient(backendNodes)
	case "redis":
		log.Info("Backend source(s) set to %s", strings.Join(backendNodes, ", "))
		client, err = redis.NewRedisClient(backendNodes, config.ClientKey, config.Separator)
	case "env":
		client, err = env.NewEnvClient()
	case "file":
		log.Info("Backend source(s) set to %s", strings.Join(config.YAMLFile, ", "))
		client, err = file.NewFileClient(config.YAMLFile, config.Filter)
	case "vault":
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
		client, err = dynamodb.NewDynamoDBClient(table)
	case "ssm":
		client, err = ssm.New()
	case "secretsmanager":
		log.Info("Backend source(s) set to AWS Secrets Manager")
		client, err = secretsmanager.New(config.SecretsManagerVersionStage, config.SecretsManagerNoFlatten)
	default:
		return nil, errors.New("invalid backend")
	}

	if err != nil {
		return nil, err
	}

	// Wrap client with metrics collection
	return NewWithMetrics(config.Backend, client), nil
}
