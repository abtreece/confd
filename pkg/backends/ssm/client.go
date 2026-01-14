package ssm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ssmAPI defines the interface for SSM operations used by this client.
// This allows for easy mocking in tests.
type ssmAPI interface {
	GetParameter(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	GetParametersByPath(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
}

// Client is a wrapper around the AWS SSM client.
type Client struct {
	client ssmAPI
}

// New creates a new SSM client with automatic region detection.
func New(dialTimeout time.Duration) (*Client, error) {
	ctx := context.Background()

	// Use provided timeout or fall back to default
	if dialTimeout == 0 {
		dialTimeout = 2 * time.Second
	}

	// Attempt to get AWS Region from environment first, then EC2 metadata
	var region string
	if os.Getenv("AWS_REGION") != "" {
		region = os.Getenv("AWS_REGION")
	} else {
		// Try to get region from EC2 metadata with a timeout
		imdsCtx, cancel := context.WithTimeout(ctx, dialTimeout)
		defer cancel()

		imdsClient := imds.New(imds.Options{})
		regionOutput, err := imdsClient.GetRegion(imdsCtx, &imds.GetRegionInput{})
		if err == nil {
			region = regionOutput.Region
		}
	}

	// Build config options
	var optFns []func(*config.LoadOptions) error
	if region != "" {
		optFns = append(optFns, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	log.Debug("Region: %s", cfg.Region)

	// Fail early if no credentials can be found
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}
	if !creds.HasKeys() {
		return nil, fmt.Errorf("no AWS credentials found")
	}

	// Create SSM client with optional local endpoint
	var ssmOpts []func(*ssm.Options)
	if os.Getenv("SSM_LOCAL") != "" {
		log.Debug("SSM_LOCAL is set")
		endpoint := os.Getenv("SSM_ENDPOINT_URL")
		ssmOpts = append(ssmOpts, func(o *ssm.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	client := ssm.NewFromConfig(cfg, ssmOpts...)
	return &Client{client}, nil
}

// GetValues retrieves the values for the given keys from AWS SSM Parameter Store
func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	vars := make(map[string]string)
	for _, key := range keys {
		log.Debug("Processing key=%s", key)
		resp, err := c.getParametersWithPrefix(ctx, key)
		if err != nil {
			return vars, err
		}
		if len(resp) == 0 {
			resp, err = c.getParameter(ctx, key)
			if err != nil {
				// Check if it's a ParameterNotFound error
				var notFoundErr *types.ParameterNotFound
				if !errors.As(err, &notFoundErr) {
					return vars, err
				}
			}
		}
		for k, v := range resp {
			vars[k] = v
		}
	}
	return vars, nil
}

func (c *Client) getParametersWithPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	parameters := make(map[string]string)
	input := &ssm.GetParametersByPathInput{
		Path:           aws.String(prefix),
		Recursive:      aws.Bool(true),
		WithDecryption: aws.Bool(true),
	}

	paginator := ssm.NewGetParametersByPathPaginator(c.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return parameters, err
		}
		for _, p := range page.Parameters {
			parameters[*p.Name] = *p.Value
		}
	}
	return parameters, nil
}

func (c *Client) getParameter(ctx context.Context, name string) (map[string]string, error) {
	parameters := make(map[string]string)
	input := &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	}
	resp, err := c.client.GetParameter(ctx, input)
	if err != nil {
		return parameters, err
	}
	parameters[*resp.Parameter.Name] = *resp.Parameter.Value
	return parameters, nil
}

// WatchPrefix is not implemented
func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	<-stopChan
	return 0, nil
}

// HealthCheck verifies the backend connection is healthy.
// It attempts a simple operation to verify AWS credentials and connectivity.
func (c *Client) HealthCheck(ctx context.Context) error {
	start := time.Now()
	logger := log.With("backend", "ssm")

	// Try to list parameters in root path to verify connectivity
	// This may return empty results but will fail if credentials are invalid
	_, err := c.client.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
		Path:       aws.String("/"),
		Recursive:  aws.Bool(false),
		MaxResults: aws.Int32(1),
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
