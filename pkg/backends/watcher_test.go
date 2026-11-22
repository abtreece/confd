package backends

import (
	"testing"

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
	"github.com/abtreece/confd/pkg/backends/vault"
	"github.com/abtreece/confd/pkg/backends/zookeeper"
)

// TestWatcher_ImplementedOnlyByWatchCapableBackends pins which backends
// implement Watcher. The clients are typed nil pointers: only the method set
// matters, so no backend service is needed.
func TestWatcher_ImplementedOnlyByWatchCapableBackends(t *testing.T) {
	tests := []struct {
		backend   string
		client    StoreClient
		wantWatch bool
	}{
		{"consul", (*consul.ConsulClient)(nil), true},
		{"etcd", (*etcd.Client)(nil), true},
		{"file", (*file.Client)(nil), true},
		{"redis", (*redis.Client)(nil), true},
		{"zookeeper", (*zookeeper.Client)(nil), true},
		{"acm", (*acm.Client)(nil), false},
		{"dynamodb", (*dynamodb.Client)(nil), false},
		{"env", (*env.Client)(nil), false},
		{"imds", (*imds.Client)(nil), false},
		{"secretsmanager", (*secretsmanager.Client)(nil), false},
		{"ssm", (*ssm.Client)(nil), false},
		{"vault", (*vault.Client)(nil), false},
	}

	for _, tt := range tests {
		t.Run(tt.backend, func(t *testing.T) {
			_, gotWatch := tt.client.(Watcher)
			if gotWatch != tt.wantWatch {
				t.Errorf("%s implements Watcher = %v, want %v", tt.backend, gotWatch, tt.wantWatch)
			}
		})
	}
}
