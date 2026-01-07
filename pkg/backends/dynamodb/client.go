package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/abtreece/confd/pkg/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// dynamoDBAPI defines the interface for DynamoDB operations used by this client.
// This allows for easy mocking in tests.
type dynamoDBAPI interface {
	GetItem(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	Scan(ctx context.Context, input *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	DescribeTable(ctx context.Context, input *dynamodb.DescribeTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

// Client is a wrapper around the DynamoDB client
// and also holds the table to lookup key value pairs from
type Client struct {
	client dynamoDBAPI
	table  string
}

// NewDynamoDBClient returns an *dynamodb.Client with a connection to the region
// configured via the AWS_REGION environment variable.
// It returns an error if the connection cannot be made or the table does not exist.
func NewDynamoDBClient(table string) (*Client, error) {
	ctx := context.Background()

	// Build config options
	var optFns []func(*config.LoadOptions) error

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Fail early if no credentials can be found
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}
	if !creds.HasKeys() {
		return nil, errors.New("no AWS credentials found")
	}

	// Create DynamoDB client with optional local endpoint
	var ddbOpts []func(*dynamodb.Options)
	if os.Getenv("DYNAMODB_LOCAL") != "" {
		log.Debug("DYNAMODB_LOCAL is set")
		endpoint := os.Getenv("DYNAMODB_ENDPOINT_URL")
		ddbOpts = append(ddbOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	d := dynamodb.NewFromConfig(cfg, ddbOpts...)

	// Check if the table exists
	_, err = d.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &table})
	if err != nil {
		return nil, fmt.Errorf("failed to describe table %s: %w", table, err)
	}

	return &Client{d, table}, nil
}

// GetValues retrieves the values for the given keys from DynamoDB
func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	vars := make(map[string]string)
	for _, key := range keys {
		// Check if we can find the single item
		m := map[string]types.AttributeValue{
			"key": &types.AttributeValueMemberS{Value: key},
		}
		g, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{Key: m, TableName: &c.table})
		if err != nil {
			return vars, err
		}

		if g.Item != nil {
			if val, ok := g.Item["value"]; ok {
				if s, ok := val.(*types.AttributeValueMemberS); ok {
					vars[key] = s.Value
				} else {
					log.Warning("Skipping key '%s'. 'value' is not of type 'string'.", key)
				}
				continue
			}
		}

		// Check for nested keys
		q, err := c.client.Scan(ctx, &dynamodb.ScanInput{
			FilterExpression:     aws.String("begins_with(#k, :prefix)"),
			ProjectionExpression: aws.String("#k, #v"),
			ExpressionAttributeNames: map[string]string{
				"#k": "key",
				"#v": "value",
			},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":prefix": &types.AttributeValueMemberS{Value: key},
			},
			TableName: aws.String(c.table),
		})

		if err != nil {
			return vars, err
		}

		for _, item := range q.Items {
			keyAttr, keyOk := item["key"]
			valAttr, valOk := item["value"]
			if !keyOk || !valOk {
				continue
			}

			keyStr, keyIsStr := keyAttr.(*types.AttributeValueMemberS)
			valStr, valIsStr := valAttr.(*types.AttributeValueMemberS)

			if keyIsStr && valIsStr {
				vars[keyStr.Value] = valStr.Value
			} else if keyIsStr {
				log.Warning("Skipping key '%s'. 'value' is not of type 'string'.", keyStr.Value)
			}
		}
	}
	return vars, nil
}

// WatchPrefix is not implemented
func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	<-stopChan
	return 0, nil
}
