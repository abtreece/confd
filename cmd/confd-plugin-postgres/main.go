package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/abtreece/confd/pkg/backends/plugin/api"
	"github.com/abtreece/confd/pkg/backends/postgres"
	"github.com/hashicorp/go-plugin"
)

func main() {
	// Parse CLI arguments to configure the plugin
	var node, username, password, database, table string
	flag.StringVar(&node, "node", "127.0.0.1:5432", "Postgres node endpoint")
	flag.StringVar(&username, "username", "admin", "Postgres username")
	flag.StringVar(&password, "password", "secret", "Postgres password")
	flag.StringVar(&database, "database", "confd", "Postgres database name")
	flag.StringVar(&table, "table", "confd_config", "Postgres table name")
	flag.Parse()

	// If environment variables are set, they can act as fallbacks (useful when spawned by confd without args)
	if envNode := os.Getenv("CONFD_BACKEND_NODE"); envNode != "" {
		node = envNode
	}
	if envUser := os.Getenv("CONFD_USERNAME"); envUser != "" {
		username = envUser
	}
	if envPass := os.Getenv("CONFD_PASSWORD"); envPass != "" {
		password = envPass
	}
	if envDB := os.Getenv("CONFD_DATABASE"); envDB != "" {
		database = envDB
	}
	if envTable := os.Getenv("CONFD_TABLE"); envTable != "" {
		table = envTable
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
