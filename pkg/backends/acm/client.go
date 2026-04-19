package acm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/backends/awsutil"
	"github.com/abtreece/confd/pkg/backends/types"
	"github.com/abtreece/confd/pkg/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
)

// acmAPI defines the interface for ACM operations used by this client
type acmAPI interface {
	GetCertificate(ctx context.Context, input *acm.GetCertificateInput, opts ...func(*acm.Options)) (*acm.GetCertificateOutput, error)
	ExportCertificate(ctx context.Context, input *acm.ExportCertificateInput, opts ...func(*acm.Options)) (*acm.ExportCertificateOutput, error)
	ListCertificates(ctx context.Context, input *acm.ListCertificatesInput, opts ...func(*acm.Options)) (*acm.ListCertificatesOutput, error)
}

type Client struct {
	client           acmAPI
	exportPrivateKey bool
	passphrase       []byte
}

// New initializes the AWS ACM backend for confd
func New(exportPrivateKey bool, dialTimeout time.Duration) (*Client, error) {
	ctx := context.Background()

	cfg, err := awsutil.LoadAWSConfig(ctx, dialTimeout)
	if err != nil {
		return nil, err
	}

	var acmOpts []func(*acm.Options)
	if os.Getenv("ACM_LOCAL") != "" {
		log.Debug("ACM_LOCAL is set")
		endpoint := os.Getenv("ACM_ENDPOINT_URL")
		acmOpts = append(acmOpts, func(o *acm.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	var passphrase []byte
	if exportPrivateKey {
		passphraseStr := os.Getenv("ACM_PASSPHRASE")
		if passphraseStr == "" {
			return nil, fmt.Errorf("ACM_PASSPHRASE environment variable is required when exporting private keys")
		}
		passphrase = []byte(passphraseStr)
		log.Debug("Private key export enabled")
	}

	client := acm.NewFromConfig(cfg, acmOpts...)
	return &Client{
		client:           client,
		exportPrivateKey: exportPrivateKey,
		passphrase:       passphrase,
	}, nil
}

func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	vars := make(map[string]string)

	for _, key := range keys {
		// Strip leading slash added by confd's prefix handling
		// ARNs should start with "arn:" not "/arn:"
		arn := strings.TrimPrefix(key, "/")

		if c.exportPrivateKey {
			// Use ExportCertificate API to get certificate with private key
			input := &acm.ExportCertificateInput{
				CertificateArn: aws.String(arn),
				Passphrase:     c.passphrase,
			}

			result, err := c.client.ExportCertificate(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("failed to export certificate: %w", err)
			}

			// Use the original key (with prefix) for the return map
			// so confd template functions work correctly
			if result.Certificate != nil {
				vars[key] = *result.Certificate
			}

			if result.CertificateChain != nil {
				vars[key+"_chain"] = *result.CertificateChain
			}

			if result.PrivateKey != nil {
				vars[key+"_private_key"] = *result.PrivateKey
			}

			log.Debug("Exported certificate with private key for ARN: %s", arn)
		} else {
			// Use GetCertificate API (default behavior)
			input := &acm.GetCertificateInput{
				CertificateArn: aws.String(arn),
			}

			result, err := c.client.GetCertificate(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve certificate: %w", err)
			}

			// Use the original key (with prefix) for the return map
			// so confd template functions work correctly
			if result.Certificate != nil {
				vars[key] = *result.Certificate
			}

			if result.CertificateChain != nil {
				vars[key+"_chain"] = *result.CertificateChain
			}

			log.Debug("Retrieved certificate for ARN: %s", arn)
		}
	}

	return vars, nil
}

func (c *Client) ListCertificates(ctx context.Context) ([]string, error) {
	var certs []string

	paginator := acm.NewListCertificatesPaginator(c.client, &acm.ListCertificatesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list certificates: %w", err)
		}
		for _, cert := range page.CertificateSummaryList {
			certs = append(certs, aws.ToString(cert.CertificateArn))
		}
	}

	return certs, nil
}

// WatchPrefix is not implemented for ACM
func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	select {
	case <-ctx.Done():
		return waitIndex, ctx.Err()
	case <-stopChan:
		return waitIndex, nil
	}
}

// HealthCheck verifies the backend connection is healthy.
// It attempts to list certificates to verify AWS credentials and connectivity.
func (c *Client) HealthCheck(ctx context.Context) error {
	start := time.Now()
	logger := log.With("backend", "acm")

	_, err := c.client.ListCertificates(ctx, &acm.ListCertificatesInput{
		MaxItems: aws.Int32(1),
	})

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

// HealthCheckDetailed provides detailed health information for the ACM backend.
func (c *Client) HealthCheckDetailed(ctx context.Context) (*types.HealthResult, error) {
	start := time.Now()

	// List certificates to get count
	paginator := acm.NewListCertificatesPaginator(c.client, &acm.ListCertificatesInput{})
	certCount := 0
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			duration := time.Since(start)
			return &types.HealthResult{
				Healthy:   false,
				Message:   fmt.Sprintf("ACM health check failed: %s", err.Error()),
				Duration:  types.DurationMillis(duration),
				CheckedAt: time.Now(),
				Details: map[string]string{
					"error": err.Error(),
				},
			}, err
		}
		certCount += len(page.CertificateSummaryList)
	}

	duration := time.Since(start)

	return &types.HealthResult{
		Healthy:   true,
		Message:   "ACM backend is healthy",
		Duration:  types.DurationMillis(duration),
		CheckedAt: time.Now(),
		Details: map[string]string{
			"certificate_count": fmt.Sprintf("%d", certCount),
		},
	}, nil
}

// Close is a no-op for this backend.
func (c *Client) Close() error {
	return nil
}
