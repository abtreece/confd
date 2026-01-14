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
		return nil, fmt.Errorf("no AWS credentials found")
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
// For JSON secrets, keys like /database/host will look up the "database" secret
// and extract the "host" field from the JSON.
func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	vars := make(map[string]string)

	// Cache for fetched secrets to avoid duplicate API calls
	secretCache := make(map[string]*secretsmanager.GetSecretValueOutput)
	// Track errors separately from not-found
	errorCache := make(map[string]error)

	for _, key := range keys {
		log.Debug("Processing key=%s", key)

		// Remove leading slash for secret name
		secretName := strings.TrimPrefix(key, "/")

		// First, try direct secret lookup
		val, found, err := c.fetchAndProcessSecret(ctx, secretName, key, secretCache, errorCache, vars)
		if err != nil {
			return vars, err
		}
		if found {
			vars[key] = val
			continue
		}

		// If not found and JSON flattening is enabled, try parent lookups
		// For a key like /database/host, try fetching "database" and look for "host" in JSON
		if !c.noFlatten {
			parts := strings.Split(secretName, "/")
			for i := len(parts) - 1; i > 0; i-- {
				parentName := strings.Join(parts[:i], "/")
				childPath := strings.Join(parts[i:], "/")

				resp, err := c.getSecretCached(ctx, parentName, secretCache, errorCache)
				if err != nil {
					return vars, err
				}
				if resp == nil {
					continue
				}

				if resp.SecretString != nil {
					var jsonSecret map[string]interface{}
					if err := json.Unmarshal([]byte(*resp.SecretString), &jsonSecret); err == nil {
						// Look for the child key in the JSON
						if val, ok := jsonSecret[childPath]; ok {
							vars[key] = fmt.Sprintf("%v", val)
							log.Debug("Found key %s in JSON secret %s", childPath, parentName)
							break
						}
					}
				}
			}
		}
	}

	return vars, nil
}

// getSecretCached retrieves a secret, using the cache if available.
func (c *Client) getSecretCached(ctx context.Context, secretName string, cache map[string]*secretsmanager.GetSecretValueOutput, errorCache map[string]error) (*secretsmanager.GetSecretValueOutput, error) {
	// Check error cache first
	if err, ok := errorCache[secretName]; ok {
		return nil, err
	}
	// Check success cache
	if resp, ok := cache[secretName]; ok {
		return resp, nil
	}

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String(c.versionStage),
	}

	resp, err := c.client.GetSecretValue(ctx, input)
	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			log.Debug("Secret not found: %s", secretName)
			cache[secretName] = nil
			return nil, nil
		}
		// Cache other errors to propagate them
		errorCache[secretName] = err
		return nil, err
	}

	cache[secretName] = resp
	return resp, nil
}

// fetchAndProcessSecret attempts to fetch a secret directly and process it.
// Returns the value, whether it was found, and any error.
func (c *Client) fetchAndProcessSecret(ctx context.Context, secretName, key string, cache map[string]*secretsmanager.GetSecretValueOutput, errorCache map[string]error, vars map[string]string) (string, bool, error) {
	resp, err := c.getSecretCached(ctx, secretName, cache, errorCache)
	if err != nil {
		return "", false, err
	}
	if resp == nil {
		return "", false, nil
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
				// Return empty since we populated vars directly for flattened keys
				return "", false, nil
			}
		}
		// Plain string secret (or noFlatten enabled, or not valid JSON)
		return *resp.SecretString, true, nil
	} else if resp.SecretBinary != nil {
		// Binary secret - base64 encode
		return base64.StdEncoding.EncodeToString(resp.SecretBinary), true, nil
	}

	return "", false, nil
}

// WatchPrefix is not implemented for Secrets Manager.
// Secrets Manager does not support streaming/watching for changes.
func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	<-stopChan
	return 0, nil
}

// HealthCheck verifies the backend connection is healthy.
// It attempts to get a non-existent secret to verify AWS credentials and connectivity.
// A "not found" error is expected and indicates successful connectivity.
func (c *Client) HealthCheck(ctx context.Context) error {
	start := time.Now()
	logger := log.With("backend", "secretsmanager")

	_, err := c.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String("confd-health-check-nonexistent"),
	})

	duration := time.Since(start)
	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			// Not found is expected and indicates connectivity is working
			logger.InfoContext(ctx, "Backend health check passed",
				"duration_ms", duration.Milliseconds())
			return nil
		}
		logger.ErrorContext(ctx, "Backend health check failed",
			"duration_ms", duration.Milliseconds(),
			"error", err.Error())
		return err
	}

	logger.InfoContext(ctx, "Backend health check passed",
		"duration_ms", duration.Milliseconds())
	return nil
}
