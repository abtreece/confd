// Command confd-plugin-postgres is a reference implementation of a confd
// backend plugin. It speaks the confd plugin RPC protocol (see
// pkg/backends/plugin/api) and is served over HashiCorp go-plugin.
//
// It is intentionally self-contained: the PostgreSQL access logic is inlined
// here rather than imported from confd internals, so the plugin demonstrates
// everything an out-of-tree backend author needs in a single file.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/backends/plugin/api"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Parse CLI arguments to configure the plugin.
	var node, username, password, database, table string
	flag.StringVar(&node, "node", "127.0.0.1:5432", "Postgres node endpoint")
	flag.StringVar(&username, "username", "admin", "Postgres username")
	flag.StringVar(&password, "password", "secret", "Postgres password")
	flag.StringVar(&database, "database", "confd", "Postgres database name")
	flag.StringVar(&table, "table", "confd_config", "Postgres table name")
	flag.Parse()

	// Environment variables act as fallbacks (useful when spawned by confd
	// without args).
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

	provider, err := newPostgresProvider(node, username, password, database, table, 10*time.Second)
	if err != nil {
		log.Fatalf("Failed to initialize postgres plugin: %v", err)
	}

	// Serve the backend over RPC using HashiCorp go-plugin.
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: api.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"backend": &api.ConfdBackendPlugin{Impl: provider},
		},
	})
}

// postgresProvider implements api.BackendProvider against a PostgreSQL pool.
type postgresProvider struct {
	pool  *pgxpool.Pool
	table string
}

func newPostgresProvider(node, username, password, database, table string, dialTimeout time.Duration) (*postgresProvider, error) {
	if database == "" {
		database = "confd"
	}
	if table == "" {
		table = "confd_config"
	}

	var connStr string
	if username != "" {
		connStr = fmt.Sprintf("postgres://%s:%s@%s/%s", username, password, node, database)
	} else {
		connStr = fmt.Sprintf("postgres://%s/%s", node, database)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	return &postgresProvider{pool: pool, table: table}, nil
}

func (p *postgresProvider) GetValues(keys []string) (map[string]string, error) {
	ctx := context.Background()
	vars := make(map[string]string)
	for _, key := range keys {
		prefix := key
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		prefix += "%"

		query := fmt.Sprintf("SELECT key, value FROM %s WHERE key = $1 OR key LIKE $2", p.table)
		rows, err := p.pool.Query(ctx, query, key, prefix)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err != nil {
				rows.Close()
				return nil, err
			}
			vars[k] = v
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return vars, nil
}

// WatchPrefix uses PostgreSQL LISTEN/NOTIFY to wait for real-time changes on
// the "confd_update" channel. It blocks until a notification arrives.
func (p *postgresProvider) WatchPrefix(prefix string, keys []string, waitIndex uint64) (uint64, error) {
	ctx := context.Background()
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		log.Printf("[postgres-plugin] failed to acquire connection: %v, falling back to 5s sleep", err)
		time.Sleep(5 * time.Second)
		return waitIndex, nil
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN confd_update"); err != nil {
		log.Printf("[postgres-plugin] LISTEN failed: %v, falling back to 5s sleep", err)
		time.Sleep(5 * time.Second)
		return waitIndex, nil
	}

	notification, err := conn.Conn().WaitForNotification(ctx)
	if err != nil {
		log.Printf("[postgres-plugin] LISTEN/NOTIFY error: %v", err)
		return waitIndex, nil
	}
	log.Printf("[postgres-plugin] NOTIFY received: channel=%s payload=%s", notification.Channel, notification.Payload)
	return waitIndex + 1, nil
}

func (p *postgresProvider) HealthCheck() error {
	return p.pool.Ping(context.Background())
}

func (p *postgresProvider) Close() error {
	if p.pool != nil {
		p.pool.Close()
	}
	return nil
}
