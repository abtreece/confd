package backends

import (
	"time"

	util "github.com/abtreece/confd/pkg/util"
)

// Config holds the configuration for backend connections.
type Config struct {
	AuthToken      string     `toml:"auth_token"`
	AuthType       string     `toml:"auth_type"`
	Backend        string     `toml:"backend"`
	BasicAuth      bool       `toml:"basic_auth"`
	ClientCaKeys   string     `toml:"client_cakeys"`
	ClientCert     string     `toml:"client_cert"`
	ClientKey      string     `toml:"client_key"`
	ClientInsecure bool       `toml:"client_insecure"`
	CertificateARN string     `toml:"certificate_arn"`
	BackendNodes   util.Nodes `toml:"nodes"`
	Password       string     `toml:"password"`
	Scheme         string     `toml:"scheme"`
	Table          string     `toml:"table"`
	Separator      string     `toml:"separator"`
	Username       string     `toml:"username"`
	AppID          string     `toml:"app_id"`
	UserID         string     `toml:"user_id"`
	RoleID         string     `toml:"role_id"`
	SecretID       string     `toml:"secret_id"`
	YAMLFile       util.Nodes `toml:"file"`
	Filter         string     `toml:"filter"`
	Path           string     `toml:"path"`
	Role                         string
	ACMExportPrivateKey          bool   `toml:"acm_export_private_key"`
	SecretsManagerVersionStage   string `toml:"secretsmanager_version_stage"`
	SecretsManagerNoFlatten      bool   `toml:"secretsmanager_no_flatten"`
	IMDSCacheTTL                 time.Duration `toml:"imds_cache_ttl"`

	// Connection timeouts
	DialTimeout  time.Duration `toml:"dial_timeout"`
	ReadTimeout  time.Duration `toml:"read_timeout"`
	WriteTimeout time.Duration `toml:"write_timeout"`

	// Retry configuration
	RetryMaxAttempts int           `toml:"retry_max_attempts"`
	RetryBaseDelay   time.Duration `toml:"retry_base_delay"`
	RetryMaxDelay    time.Duration `toml:"retry_max_delay"`
}

// ApplyTimeoutDefaults applies default timeout values if they are not set (zero values).
// This ensures consistent defaults across all backends without duplication.
func (c *Config) ApplyTimeoutDefaults() {
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 1 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 1 * time.Second
	}
	if c.RetryMaxAttempts == 0 {
		c.RetryMaxAttempts = 3
	}
	if c.RetryBaseDelay == 0 {
		c.RetryBaseDelay = 100 * time.Millisecond
	}
	if c.RetryMaxDelay == 0 {
		c.RetryMaxDelay = 5 * time.Second
	}
	if c.IMDSCacheTTL == 0 {
		c.IMDSCacheTTL = 60 * time.Second
	}
}
