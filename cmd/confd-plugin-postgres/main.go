package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/abtreece/confd/pkg/backends/plugin/api"
	"github.com/abtreece/confd/pkg/backends/postgres"
	"github.com/hashicorp/go-plugin"
)

func main() {
	// The plugin doesn't receive CLI flags directly from confd since it's a subprocess.
	// We read configuration via Environment Variables.
	node := os.Getenv("CONFD_BACKEND_NODE")
	if node == "" {
		node = "127.0.0.1:5432"
	}
	username := os.Getenv("CONFD_USERNAME")
	password := os.Getenv("CONFD_PASSWORD")
	database := os.Getenv("CONFD_DATABASE")
	if database == "" {
		database = "confd"
	}
	table := os.Getenv("CONFD_TABLE")
	if table == "" {
		table = "confd_config"
	}

	// Initialize the exact same PostgreSQL backend we created earlier!
	client, err := postgres.New([]string{node}, username, password, database, table, 10*time.Second)
	if err != nil {
		log.Fatalf("Failed to initialize postgres plugin: %v", err)
	}

	// Serve the backend over RPC using HashiCorp go-plugin
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: api.Handshake,
		Plugins: map[string]plugin.Plugin{
			"backend": &api.ConfdBackendPlugin{
				Impl: &ProviderWrapper{client: client},
			},
		},
	})
}

// ProviderWrapper adapts confd's native StoreClient (which requires context)
// to our RPC-friendly BackendProvider interface.
type ProviderWrapper struct {
	client *postgres.Client
}

func (w *ProviderWrapper) GetValues(keys []string) (map[string]string, error) {
	return w.client.GetValues(context.Background(), keys)
}

func (w *ProviderWrapper) WatchPrefix(prefix string, keys []string, waitIndex uint64) (uint64, error) {
	stopChan := make(chan bool)
	return w.client.WatchPrefix(context.Background(), prefix, keys, waitIndex, stopChan)
}

func (w *ProviderWrapper) HealthCheck() error {
	return w.client.HealthCheck(context.Background())
}

func (w *ProviderWrapper) Close() error {
	return w.client.Close()
}
