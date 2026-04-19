package awsutil

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

// LoadAWSConfig loads AWS SDK config with automatic region detection.
//
// Region resolution order:
//  1. AWS_REGION environment variable
//  2. EC2 IMDS (if dialTimeout > 0 and AWS_REGION is not set)
//  3. SDK defaults (AWS_DEFAULT_REGION, profile, etc.)
//
// Returns an error if AWS config cannot be loaded or if no credentials are found.
func LoadAWSConfig(ctx context.Context, dialTimeout time.Duration) (aws.Config, error) {
	var region string
	if v := os.Getenv("AWS_REGION"); v != "" {
		region = v
	} else if dialTimeout > 0 {
		imdsCtx, cancel := context.WithTimeout(ctx, dialTimeout)
		defer cancel()
		c := imds.New(imds.Options{})
		if out, err := c.GetRegion(imdsCtx, &imds.GetRegionInput{}); err == nil {
			region = out.Region
		}
		// IMDS failure is silently ignored; SDK may resolve region from other sources.
	}

	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	log.Debug("Region: %s", cfg.Region)

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}
	if !creds.HasKeys() {
		return aws.Config{}, fmt.Errorf("no AWS credentials found")
	}

	return cfg, nil
}
