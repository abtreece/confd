package ssm

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// mockSSM implements the ssmAPI interface for testing
type mockSSM struct {
	getParameterFunc       func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	getParametersByPathFunc func(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
	nextToken              *string
}

func (m *mockSSM) GetParameter(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if m.getParameterFunc != nil {
		return m.getParameterFunc(ctx, input, opts...)
	}
	return &ssm.GetParameterOutput{}, nil
}

func (m *mockSSM) GetParametersByPath(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	if m.getParametersByPathFunc != nil {
		return m.getParametersByPathFunc(ctx, input, opts...)
	}
	return &ssm.GetParametersByPathOutput{}, nil
}

// newTestClient creates a Client with a mock SSM for testing
func newTestClient(mock *mockSSM) *Client {
	return &Client{
		client: mock,
	}
}

func TestGetValues_SingleParameter(t *testing.T) {
	mock := &mockSSM{
		getParametersByPathFunc: func(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			// No results from path search
			return &ssm.GetParametersByPathOutput{Parameters: []types.Parameter{}}, nil
		},
		getParameterFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			if *input.Name == "/app/config/key" {
				return &ssm.GetParameterOutput{
					Parameter: &types.Parameter{
						Name:  aws.String("/app/config/key"),
						Value: aws.String("test_value"),
					},
				}, nil
			}
			return nil, &types.ParameterNotFound{}
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{"/app/config/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{"/app/config/key": "test_value"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_ParametersByPath(t *testing.T) {
	mock := &mockSSM{
		getParametersByPathFunc: func(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			if *input.Path == "/app/db" {
				return &ssm.GetParametersByPathOutput{
					Parameters: []types.Parameter{
						{Name: aws.String("/app/db/host"), Value: aws.String("localhost")},
						{Name: aws.String("/app/db/port"), Value: aws.String("5432")},
					},
				}, nil
			}
			return &ssm.GetParametersByPathOutput{Parameters: []types.Parameter{}}, nil
		},
	}

	client := newTestClient(mock)

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

func TestGetValues_PaginatedResults(t *testing.T) {
	callCount := 0
	mock := &mockSSM{
		getParametersByPathFunc: func(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			callCount++
			if input.NextToken == nil {
				// First page
				return &ssm.GetParametersByPathOutput{
					Parameters: []types.Parameter{
						{Name: aws.String("/app/page1/key1"), Value: aws.String("value1")},
					},
					NextToken: aws.String("token"),
				}, nil
			}
			// Second page (last)
			return &ssm.GetParametersByPathOutput{
				Parameters: []types.Parameter{
					{Name: aws.String("/app/page1/key2"), Value: aws.String("value2")},
				},
				NextToken: nil,
			}, nil
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{"/app/page1"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/app/page1/key1": "value1",
		"/app/page1/key2": "value2",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_MultipleKeys(t *testing.T) {
	mock := &mockSSM{
		getParametersByPathFunc: func(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			path := *input.Path
			switch path {
			case "/key1":
				return &ssm.GetParametersByPathOutput{
					Parameters: []types.Parameter{
						{Name: aws.String("/key1/sub"), Value: aws.String("value1")},
					},
				}, nil
			case "/key2":
				return &ssm.GetParametersByPathOutput{
					Parameters: []types.Parameter{
						{Name: aws.String("/key2/sub"), Value: aws.String("value2")},
					},
				}, nil
			default:
				return &ssm.GetParametersByPathOutput{Parameters: []types.Parameter{}}, nil
			}
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{"/key1", "/key2"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/key1/sub": "value1",
		"/key2/sub": "value2",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_ParameterNotFound(t *testing.T) {
	mock := &mockSSM{
		getParametersByPathFunc: func(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			// No results from path search
			return &ssm.GetParametersByPathOutput{Parameters: []types.Parameter{}}, nil
		},
		getParameterFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return nil, &types.ParameterNotFound{}
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{"/missing/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_PathError(t *testing.T) {
	expectedErr := errors.New("internal error")
	mock := &mockSSM{
		getParametersByPathFunc: func(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			return nil, expectedErr
		},
	}

	client := newTestClient(mock)

	_, err := client.GetValues(context.Background(), []string{"/app/config"})
	if err == nil {
		t.Error("GetValues() expected error, got nil")
	}
}

func TestGetValues_GetParameterError(t *testing.T) {
	expectedErr := errors.New("access denied")
	mock := &mockSSM{
		getParametersByPathFunc: func(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			// No results from path search
			return &ssm.GetParametersByPathOutput{Parameters: []types.Parameter{}}, nil
		},
		getParameterFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return nil, expectedErr
		},
	}

	client := newTestClient(mock)

	_, err := client.GetValues(context.Background(), []string{"/app/config"})
	if err == nil {
		t.Error("GetValues() expected error, got nil")
	}
}

func TestGetValues_EmptyKeys(t *testing.T) {
	mock := &mockSSM{}
	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_RecursiveEnabled(t *testing.T) {
	var capturedInput *ssm.GetParametersByPathInput
	mock := &mockSSM{
		getParametersByPathFunc: func(ctx context.Context, input *ssm.GetParametersByPathInput, opts ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
			capturedInput = input
			return &ssm.GetParametersByPathOutput{Parameters: []types.Parameter{}}, nil
		},
		getParameterFunc: func(ctx context.Context, input *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
			return nil, &types.ParameterNotFound{}
		},
	}

	client := newTestClient(mock)
	client.GetValues(context.Background(), []string{"/app"})

	if capturedInput == nil {
		t.Fatal("GetParametersByPath was not called")
	}
	if !*capturedInput.Recursive {
		t.Error("Recursive should be true")
	}
	if !*capturedInput.WithDecryption {
		t.Error("WithDecryption should be true")
	}
}

func TestWatchPrefix(t *testing.T) {
	mock := &mockSSM{}
	client := newTestClient(mock)

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
