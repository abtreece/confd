package dynamodb

import (
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// mockDynamoDB implements the dynamoDBAPI interface for testing
type mockDynamoDB struct {
	getItemFunc      func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error)
	scanFunc         func(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error)
	describeTableFunc func(input *dynamodb.DescribeTableInput) (*dynamodb.DescribeTableOutput, error)
}

func (m *mockDynamoDB) GetItem(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(input)
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *mockDynamoDB) Scan(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
	if m.scanFunc != nil {
		return m.scanFunc(input)
	}
	return &dynamodb.ScanOutput{}, nil
}

func (m *mockDynamoDB) DescribeTable(input *dynamodb.DescribeTableInput) (*dynamodb.DescribeTableOutput, error) {
	if m.describeTableFunc != nil {
		return m.describeTableFunc(input)
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
		getItemFunc: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
			key := *input.Key["key"].S
			if key == "/test/key" {
				return &dynamodb.GetItemOutput{
					Item: map[string]*dynamodb.AttributeValue{
						"key":   {S: aws.String("/test/key")},
						"value": {S: aws.String("test_value")},
					},
				}, nil
			}
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
		scanFunc: func(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{Items: []map[string]*dynamodb.AttributeValue{}}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	result, err := client.GetValues([]string{"/test/key"})
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
		getItemFunc: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
			// No exact match, return empty
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
		scanFunc: func(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
			// Return items that match the prefix
			return &dynamodb.ScanOutput{
				Items: []map[string]*dynamodb.AttributeValue{
					{
						"key":   {S: aws.String("/app/db/host")},
						"value": {S: aws.String("localhost")},
					},
					{
						"key":   {S: aws.String("/app/db/port")},
						"value": {S: aws.String("5432")},
					},
				},
			}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	result, err := client.GetValues([]string{"/app/db"})
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
		getItemFunc: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
			key := *input.Key["key"].S
			switch key {
			case "/key1":
				return &dynamodb.GetItemOutput{
					Item: map[string]*dynamodb.AttributeValue{
						"key":   {S: aws.String("/key1")},
						"value": {S: aws.String("value1")},
					},
				}, nil
			case "/key2":
				return &dynamodb.GetItemOutput{
					Item: map[string]*dynamodb.AttributeValue{
						"key":   {S: aws.String("/key2")},
						"value": {S: aws.String("value2")},
					},
				}, nil
			}
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
		scanFunc: func(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{Items: []map[string]*dynamodb.AttributeValue{}}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	result, err := client.GetValues([]string{"/key1", "/key2"})
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
		getItemFunc: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
		scanFunc: func(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{Items: []map[string]*dynamodb.AttributeValue{}}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	result, err := client.GetValues([]string{"/missing/key"})
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
		getItemFunc: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
			return nil, expectedErr
		},
	}

	client := newTestClient(mock, "test-table")

	_, err := client.GetValues([]string{"/test/key"})
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
		getItemFunc: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
			// No exact match
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
		scanFunc: func(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
			return nil, expectedErr
		},
	}

	client := newTestClient(mock, "test-table")

	_, err := client.GetValues([]string{"/test/prefix"})
	if err == nil {
		t.Error("GetValues() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("GetValues() error = %v, want %v", err, expectedErr)
	}
}

func TestGetValues_NonStringValue(t *testing.T) {
	mock := &mockDynamoDB{
		getItemFunc: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
			// Return item with non-string value (e.g., a number)
			return &dynamodb.GetItemOutput{
				Item: map[string]*dynamodb.AttributeValue{
					"key":   {S: aws.String("/test/key")},
					"value": {N: aws.String("123")}, // Number instead of string
				},
			}, nil
		},
		scanFunc: func(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
			return &dynamodb.ScanOutput{Items: []map[string]*dynamodb.AttributeValue{}}, nil
		},
	}

	client := newTestClient(mock, "test-table")

	result, err := client.GetValues([]string{"/test/key"})
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

	result, err := client.GetValues([]string{})
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

	index, err := client.WatchPrefix("/test", []string{"/test/key"}, 0, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 0 {
		t.Errorf("WatchPrefix() index = %d, want 0", index)
	}
}
