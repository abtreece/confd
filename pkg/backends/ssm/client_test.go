package ssm

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// mockSSM implements the ssmAPI interface for testing
type mockSSM struct {
	getParameterFunc          func(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
	getParametersByPathPagesFunc func(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error
}

func (m *mockSSM) GetParameter(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	if m.getParameterFunc != nil {
		return m.getParameterFunc(input)
	}
	return &ssm.GetParameterOutput{}, nil
}

func (m *mockSSM) GetParametersByPathPages(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
	if m.getParametersByPathPagesFunc != nil {
		return m.getParametersByPathPagesFunc(input, fn)
	}
	return nil
}

// newTestClient creates a Client with a mock SSM for testing
func newTestClient(mock *mockSSM) *Client {
	return &Client{
		client: mock,
	}
}

func TestGetValues_SingleParameter(t *testing.T) {
	mock := &mockSSM{
		getParametersByPathPagesFunc: func(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
			// No results from path search
			fn(&ssm.GetParametersByPathOutput{Parameters: []*ssm.Parameter{}}, true)
			return nil
		},
		getParameterFunc: func(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
			if *input.Name == "/app/config/key" {
				return &ssm.GetParameterOutput{
					Parameter: &ssm.Parameter{
						Name:  aws.String("/app/config/key"),
						Value: aws.String("test_value"),
					},
				}, nil
			}
			return nil, awserr.New(ssm.ErrCodeParameterNotFound, "not found", nil)
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues([]string{"/app/config/key"})
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
		getParametersByPathPagesFunc: func(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
			if *input.Path == "/app/db" {
				fn(&ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{Name: aws.String("/app/db/host"), Value: aws.String("localhost")},
						{Name: aws.String("/app/db/port"), Value: aws.String("5432")},
					},
				}, true)
			}
			return nil
		},
	}

	client := newTestClient(mock)

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

func TestGetValues_PaginatedResults(t *testing.T) {
	callCount := 0
	mock := &mockSSM{
		getParametersByPathPagesFunc: func(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
			// Simulate pagination - first page
			fn(&ssm.GetParametersByPathOutput{
				Parameters: []*ssm.Parameter{
					{Name: aws.String("/app/page1/key1"), Value: aws.String("value1")},
				},
			}, false)
			callCount++

			// Second page (last)
			fn(&ssm.GetParametersByPathOutput{
				Parameters: []*ssm.Parameter{
					{Name: aws.String("/app/page1/key2"), Value: aws.String("value2")},
				},
			}, true)
			callCount++
			return nil
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues([]string{"/app/page1"})
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
		getParametersByPathPagesFunc: func(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
			path := *input.Path
			switch path {
			case "/key1":
				fn(&ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{Name: aws.String("/key1/sub"), Value: aws.String("value1")},
					},
				}, true)
			case "/key2":
				fn(&ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{Name: aws.String("/key2/sub"), Value: aws.String("value2")},
					},
				}, true)
			default:
				fn(&ssm.GetParametersByPathOutput{Parameters: []*ssm.Parameter{}}, true)
			}
			return nil
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues([]string{"/key1", "/key2"})
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
		getParametersByPathPagesFunc: func(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
			// No results from path search
			fn(&ssm.GetParametersByPathOutput{Parameters: []*ssm.Parameter{}}, true)
			return nil
		},
		getParameterFunc: func(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
			return nil, awserr.New(ssm.ErrCodeParameterNotFound, "not found", nil)
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues([]string{"/missing/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_PathError(t *testing.T) {
	expectedErr := awserr.New("InternalServerError", "internal error", nil)
	mock := &mockSSM{
		getParametersByPathPagesFunc: func(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
			return expectedErr
		},
	}

	client := newTestClient(mock)

	_, err := client.GetValues([]string{"/app/config"})
	if err == nil {
		t.Error("GetValues() expected error, got nil")
	}
}

func TestGetValues_GetParameterError(t *testing.T) {
	expectedErr := awserr.New("AccessDeniedException", "access denied", nil)
	mock := &mockSSM{
		getParametersByPathPagesFunc: func(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
			// No results from path search
			fn(&ssm.GetParametersByPathOutput{Parameters: []*ssm.Parameter{}}, true)
			return nil
		},
		getParameterFunc: func(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
			return nil, expectedErr
		},
	}

	client := newTestClient(mock)

	_, err := client.GetValues([]string{"/app/config"})
	if err == nil {
		t.Error("GetValues() expected error, got nil")
	}
}

func TestGetValues_EmptyKeys(t *testing.T) {
	mock := &mockSSM{}
	client := newTestClient(mock)

	result, err := client.GetValues([]string{})
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
		getParametersByPathPagesFunc: func(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
			capturedInput = input
			fn(&ssm.GetParametersByPathOutput{Parameters: []*ssm.Parameter{}}, true)
			return nil
		},
		getParameterFunc: func(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
			return nil, awserr.New(ssm.ErrCodeParameterNotFound, "not found", nil)
		},
	}

	client := newTestClient(mock)
	client.GetValues([]string{"/app"})

	if capturedInput == nil {
		t.Fatal("GetParametersByPathPages was not called")
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

	index, err := client.WatchPrefix("/test", []string{"/test/key"}, 0, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 0 {
		t.Errorf("WatchPrefix() index = %d, want 0", index)
	}
}
