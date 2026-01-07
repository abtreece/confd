package secretsmanager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// secretsManagerAPI defines the interface for Secrets Manager operations.
// This allows for easy mocking in tests.
type secretsManagerAPI interface {
	GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// Client is a wrapper around the AWS Secrets Manager client.
type Client struct {
	client       secretsManagerAPI
	versionStage string
	noFlatten    bool
}

// New creates a new Secrets Manager client with automatic region detection.
func New(versionStage string, noFlatten bool) (*Client, error) {
	ctx := context.Background()

	// Default version stage to AWSCURRENT
	if versionStage == "" {
		versionStage = "AWSCURRENT"
	}

	// Attempt to get AWS Region from environment first, then EC2 metadata
	var region string
	if os.Getenv("AWS_REGION") != "" {
		region = os.Getenv("AWS_REGION")
	} else {
		// Try to get region from EC2 metadata with a timeout
		imdsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		imdsClient := imds.New(imds.Options{})
		regionOutput, err := imdsClient.GetRegion(imdsCtx, &imds.GetRegionInput{})
		if err == nil {
			region = regionOutput.Region
		}
	}

	if region == "" {
		return nil, errors.New("AWS region not found. Set AWS_REGION environment variable or run on EC2")
	}

	// Build config options
	var optFns []func(*config.LoadOptions) error
	optFns = append(optFns, config.WithRegion(region))

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
		return nil, errors.New("no AWS credentials found")
	}

	// Create Secrets Manager client with optional local endpoint
	var smOpts []func(*secretsmanager.Options)
	if os.Getenv("SECRETSMANAGER_LOCAL") != "" {
		log.Debug("SECRETSMANAGER_LOCAL is set")
		endpoint := os.Getenv("SECRETSMANAGER_ENDPOINT_URL")
		smOpts = append(smOpts, func(o *secretsmanager.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	if noFlatten {
		log.Debug("JSON flattening disabled")
	}

	client := secretsmanager.NewFromConfig(cfg, smOpts...)
	return &Client{
		client:       client,
		versionStage: versionStage,
		noFlatten:    noFlatten,
	}, nil
}

// GetValues retrieves the values for the given keys from AWS Secrets Manager.
func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	vars := make(map[string]string)

	for _, key := range keys {
		log.Debug("Processing key=%s", key)

		// Remove leading slash for secret name
		secretName := strings.TrimPrefix(key, "/")

		input := &secretsmanager.GetSecretValueInput{
			SecretId:     aws.String(secretName),
			VersionStage: aws.String(c.versionStage),
		}

		resp, err := c.client.GetSecretValue(ctx, input)
		if err != nil {
			// Handle ResourceNotFoundException gracefully
			var notFoundErr *types.ResourceNotFoundException
			if errors.As(err, &notFoundErr) {
				log.Debug("Secret not found: %s", secretName)
				continue
			}
			return vars, err
		}

		if resp.SecretString != nil {
			// Try to parse as JSON first (unless noFlatten is set)
			if !c.noFlatten {
				var jsonSecret map[string]interface{}
				if err := json.Unmarshal([]byte(*resp.SecretString), &jsonSecret); err == nil {
					// Flatten JSON: /secret/key -> value
					for k, v := range jsonSecret {
						vars[key+"/"+k] = fmt.Sprintf("%v", v)
					}
					continue
				}
			}
			// Plain string secret (or noFlatten enabled, or not valid JSON)
			vars[key] = *resp.SecretString
		} else if resp.SecretBinary != nil {
			// Binary secret - base64 encode
			vars[key] = base64.StdEncoding.EncodeToString(resp.SecretBinary)
		}
	}

	return vars, nil
}

// WatchPrefix is not implemented for Secrets Manager.
// Secrets Manager does not support streaming/watching for changes.
func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	<-stopChan
	return 0, nil
}
