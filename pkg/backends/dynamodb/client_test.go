package dynamodb

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// mockDynamoDB implements the dynamoDBAPI interface for testing
type mockDynamoDB struct {
	getItemFunc       func(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	scanFunc          func(ctx context.Context, input *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	describeTableFunc func(ctx context.Context, input *dynamodb.DescribeTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

func (m *mockDynamoDB) GetItem(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(ctx, input, opts...)
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *mockDynamoDB) Scan(ctx context.Context, input *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	if m.scanFunc != nil {
		return m.scanFunc(ctx, input, opts...)
	}
	return &dynamodb.ScanOutput{}, nil
}

func (m *mockDynamoDB) DescribeTable(ctx context.Context, input *dynamodb.DescribeTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if m.describeTableFunc != nil {
		return m.describeTableFunc(ctx, input, opts...)
	}
	return &dynamodb.DescribeTableOutput{}, nil
}

// newTestClient creates a Client with a mock DynamoDB for testing
func newTestClient(mock *mockDynamoDB, table string) *Client {
	return &Client{
		client: mock,
		table:  table,
	}
}

func TestGetValues_SingleItem(t *testing.T) {
	mock := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			keyAttr := input.Key["key"]
			keyStr, ok := keyAttr.(*types.AttributeValueMemberS)
			if !ok {
				return &dynamodb.GetItemOutput{Item: nil}, nil
			}
			if keyStr.Value == "/test/key" {
				return &dynamodb.GetItemOutput{
					Item: map[string]types.AttributeValue{
						"key":   &types.AttributeValueMemberS{Value: "/test/key"},
						"value": &types.AttributeValueMemberS{Value: "test_value"},
					},
				}, nil
			}
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
		scanFunc: func(ctx context.Context, input *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{Items: []map[string]types.AttributeValue{}}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	result, err := client.GetValues(context.Background(), []string{"/test/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{"/test/key": "test_value"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_PrefixScan(t *testing.T) {
	mock := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			// No exact match, return empty
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
		scanFunc: func(ctx context.Context, input *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			// Return items that match the prefix
			return &dynamodb.ScanOutput{
				Items: []map[string]types.AttributeValue{
					{
						"key":   &types.AttributeValueMemberS{Value: "/app/db/host"},
						"value": &types.AttributeValueMemberS{Value: "localhost"},
					},
					{
						"key":   &types.AttributeValueMemberS{Value: "/app/db/port"},
						"value": &types.AttributeValueMemberS{Value: "5432"},
					},
				},
			}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	result, err := client.GetValues(context.Background(), []string{"/app/db"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/app/db/host": "localhost",
		"/app/db/port": "5432",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_MultipleKeys(t *testing.T) {
	mock := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			keyAttr := input.Key["key"]
			keyStr, ok := keyAttr.(*types.AttributeValueMemberS)
			if !ok {
				return &dynamodb.GetItemOutput{Item: nil}, nil
			}
			switch keyStr.Value {
			case "/key1":
				return &dynamodb.GetItemOutput{
					Item: map[string]types.AttributeValue{
						"key":   &types.AttributeValueMemberS{Value: "/key1"},
						"value": &types.AttributeValueMemberS{Value: "value1"},
					},
				}, nil
			case "/key2":
				return &dynamodb.GetItemOutput{
					Item: map[string]types.AttributeValue{
						"key":   &types.AttributeValueMemberS{Value: "/key2"},
						"value": &types.AttributeValueMemberS{Value: "value2"},
					},
				}, nil
			}
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
		scanFunc: func(ctx context.Context, input *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{Items: []map[string]types.AttributeValue{}}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	result, err := client.GetValues(context.Background(), []string{"/key1", "/key2"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/key1": "value1",
		"/key2": "value2",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_MissingKey(t *testing.T) {
	mock := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
		scanFunc: func(ctx context.Context, input *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{Items: []map[string]types.AttributeValue{}}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	result, err := client.GetValues(context.Background(), []string{"/missing/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_GetItemError(t *testing.T) {
	expectedErr := errors.New("dynamodb error")
	mock := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return nil, expectedErr
		},
	}

	client := newTestClient(mock, "test-table")

	_, err := client.GetValues(context.Background(), []string{"/test/key"})
	if err == nil {
		t.Error("GetValues() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("GetValues() error = %v, want %v", err, expectedErr)
	}
}

func TestGetValues_ScanError(t *testing.T) {
	expectedErr := errors.New("scan error")
	mock := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			// No exact match
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
		scanFunc: func(ctx context.Context, input *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return nil, expectedErr
		},
	}

	client := newTestClient(mock, "test-table")

	_, err := client.GetValues(context.Background(), []string{"/test/prefix"})
	if err == nil {
		t.Error("GetValues() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("GetValues() error = %v, want %v", err, expectedErr)
	}
}

func TestGetValues_NonStringValue(t *testing.T) {
	mock := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			// Return item with non-string value (e.g., a number)
			return &dynamodb.GetItemOutput{
				Item: map[string]types.AttributeValue{
					"key":   &types.AttributeValueMemberS{Value: "/test/key"},
					"value": &types.AttributeValueMemberN{Value: "123"}, // Number instead of string
				},
			}, nil
		},
		scanFunc: func(ctx context.Context, input *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{Items: []map[string]types.AttributeValue{}}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	result, err := client.GetValues(context.Background(), []string{"/test/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Non-string values should be skipped (logged as warning)
	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v (non-string values should be skipped)", result, expected)
	}
}

func TestGetValues_EmptyKeys(t *testing.T) {
	mock := &mockDynamoDB{}
	client := newTestClient(mock, "test-table")

	result, err := client.GetValues(context.Background(), []string{})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestWatchPrefix(t *testing.T) {
	mock := &mockDynamoDB{}
	client := newTestClient(mock, "test-table")

	stopChan := make(chan bool, 1)

	// Send stop signal immediately
	go func() {
		stopChan <- true
	}()

	index, err := client.WatchPrefix(context.Background(), "/test", []string{"/test/key"}, 0, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 0 {
		t.Errorf("WatchPrefix() index = %d, want 0", index)
	}
}

func TestWatchPrefix_ContextCancellation(t *testing.T) {
	mock := &mockDynamoDB{}
	client := newTestClient(mock, "test-table")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stopChan := make(chan bool)
	waitIndex := uint64(42)

	index, err := client.WatchPrefix(ctx, "/test", []string{"/test/key"}, waitIndex, stopChan)
	if err != context.Canceled {
		t.Errorf("WatchPrefix() error = %v, want context.Canceled", err)
	}
	if index != waitIndex {
		t.Errorf("WatchPrefix() index = %d, want %d", index, waitIndex)
	}
}

func TestWatchPrefix_ReturnsWaitIndex(t *testing.T) {
	mock := &mockDynamoDB{}
	client := newTestClient(mock, "test-table")

	stopChan := make(chan bool, 1)
	waitIndex := uint64(123)

	go func() {
		stopChan <- true
	}()

	index, err := client.WatchPrefix(context.Background(), "/test", []string{"/test/key"}, waitIndex, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != waitIndex {
		t.Errorf("WatchPrefix() index = %d, want %d", index, waitIndex)
	}
}

func TestHealthCheck_Success(t *testing.T) {
	mock := &mockDynamoDB{
		describeTableFunc: func(ctx context.Context, input *dynamodb.DescribeTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
			return &dynamodb.DescribeTableOutput{}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() unexpected error: %v", err)
	}
}

func TestHealthCheck_Error(t *testing.T) {
	expectedErr := errors.New("table not found")
	mock := &mockDynamoDB{
		describeTableFunc: func(ctx context.Context, input *dynamodb.DescribeTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
			return nil, expectedErr
		},
	}

	client := newTestClient(mock, "test-table")

	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("HealthCheck() error = %v, want %v", err, expectedErr)
	}
}
