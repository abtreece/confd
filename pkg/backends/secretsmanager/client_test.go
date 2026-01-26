package secretsmanager

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// mockSecretsManager implements the secretsManagerAPI interface for testing
type mockSecretsManager struct {
	getSecretValueFunc func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	listSecretsFunc    func(ctx context.Context, input *secretsmanager.ListSecretsInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
}

func (m *mockSecretsManager) GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	if m.getSecretValueFunc != nil {
		return m.getSecretValueFunc(ctx, input, opts...)
	}
	return &secretsmanager.GetSecretValueOutput{}, nil
}

func (m *mockSecretsManager) ListSecrets(ctx context.Context, input *secretsmanager.ListSecretsInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	if m.listSecretsFunc != nil {
		return m.listSecretsFunc(ctx, input, opts...)
	}
	return &secretsmanager.ListSecretsOutput{}, nil
}

// newTestClient creates a Client with a mock Secrets Manager for testing
func newTestClient(mock *mockSecretsManager, versionStage string, noFlatten bool) *Client {
	if versionStage == "" {
		versionStage = "AWSCURRENT"
	}
	return &Client{
		client:       mock,
		versionStage: versionStage,
		noFlatten:    noFlatten,
	}
}

func TestGetValues_StringSecret(t *testing.T) {
	mock := &mockSecretsManager{
		getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			if *input.SecretId == "myapp/api-key" {
				return &secretsmanager.GetSecretValueOutput{
					SecretString: aws.String("sk-1234567890"),
				}, nil
			}
			return nil, &types.ResourceNotFoundException{}
		},
	}

	client := newTestClient(mock, "", false)

	result, err := client.GetValues(context.Background(), []string{"/myapp/api-key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{"/myapp/api-key": "sk-1234567890"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_JSONSecret(t *testing.T) {
	mock := &mockSecretsManager{
		getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			if *input.SecretId == "myapp/database" {
				return &secretsmanager.GetSecretValueOutput{
					SecretString: aws.String(`{"username":"admin","password":"secret"}`),
				}, nil
			}
			return nil, &types.ResourceNotFoundException{}
		},
	}

	client := newTestClient(mock, "", false)

	result, err := client.GetValues(context.Background(), []string{"/myapp/database"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// JSON should be flattened
	if result["/myapp/database/username"] != "admin" {
		t.Errorf("GetValues()['/myapp/database/username'] = %s, want 'admin'", result["/myapp/database/username"])
	}
	if result["/myapp/database/password"] != "secret" {
		t.Errorf("GetValues()['/myapp/database/password'] = %s, want 'secret'", result["/myapp/database/password"])
	}
}

func TestGetValues_JSONSecret_NoFlatten(t *testing.T) {
	jsonStr := `{"username":"admin","password":"secret"}`
	mock := &mockSecretsManager{
		getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			if *input.SecretId == "myapp/database" {
				return &secretsmanager.GetSecretValueOutput{
					SecretString: aws.String(jsonStr),
				}, nil
			}
			return nil, &types.ResourceNotFoundException{}
		},
	}

	client := newTestClient(mock, "", true) // noFlatten = true

	result, err := client.GetValues(context.Background(), []string{"/myapp/database"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// With noFlatten, should return raw JSON string
	expected := map[string]string{"/myapp/database": jsonStr}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_BinarySecret(t *testing.T) {
	binaryData := []byte("binary-secret-data")
	mock := &mockSecretsManager{
		getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			if *input.SecretId == "myapp/cert" {
				return &secretsmanager.GetSecretValueOutput{
					SecretBinary: binaryData,
				}, nil
			}
			return nil, &types.ResourceNotFoundException{}
		},
	}

	client := newTestClient(mock, "", false)

	result, err := client.GetValues(context.Background(), []string{"/myapp/cert"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expectedBase64 := base64.StdEncoding.EncodeToString(binaryData)
	expected := map[string]string{"/myapp/cert": expectedBase64}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_NotFound(t *testing.T) {
	mock := &mockSecretsManager{
		getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return nil, &types.ResourceNotFoundException{}
		},
	}

	client := newTestClient(mock, "", false)

	result, err := client.GetValues(context.Background(), []string{"/missing/secret"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Missing secrets should return empty map, not error
	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_Error(t *testing.T) {
	expectedErr := errors.New("secretsmanager error")
	mock := &mockSecretsManager{
		getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return nil, expectedErr
		},
	}

	client := newTestClient(mock, "", false)

	_, err := client.GetValues(context.Background(), []string{"/test/secret"})
	if err == nil {
		t.Error("GetValues() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("GetValues() error = %v, want %v", err, expectedErr)
	}
}

func TestGetValues_EmptyKeys(t *testing.T) {
	mock := &mockSecretsManager{}
	client := newTestClient(mock, "", false)

	result, err := client.GetValues(context.Background(), []string{})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_MultipleSecrets(t *testing.T) {
	mock := &mockSecretsManager{
		getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			switch *input.SecretId {
			case "secret1":
				return &secretsmanager.GetSecretValueOutput{
					SecretString: aws.String("value1"),
				}, nil
			case "secret2":
				return &secretsmanager.GetSecretValueOutput{
					SecretString: aws.String("value2"),
				}, nil
			}
			return nil, &types.ResourceNotFoundException{}
		},
	}

	client := newTestClient(mock, "", false)

	result, err := client.GetValues(context.Background(), []string{"/secret1", "/secret2"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/secret1": "value1",
		"/secret2": "value2",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_VersionStage(t *testing.T) {
	mock := &mockSecretsManager{
		getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			// Verify version stage is passed correctly
			if *input.VersionStage != "AWSPREVIOUS" {
				t.Errorf("Expected VersionStage 'AWSPREVIOUS', got '%s'", *input.VersionStage)
			}
			return &secretsmanager.GetSecretValueOutput{
				SecretString: aws.String("previous-value"),
			}, nil
		},
	}

	client := newTestClient(mock, "AWSPREVIOUS", false)

	result, err := client.GetValues(context.Background(), []string{"/test/secret"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{"/test/secret": "previous-value"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_JSONParentLookup(t *testing.T) {
	// Test that /database/host looks up "database" secret and extracts "host" from JSON
	mock := &mockSecretsManager{
		getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			if *input.SecretId == "database" {
				return &secretsmanager.GetSecretValueOutput{
					SecretString: aws.String(`{"host":"127.0.0.1","port":"3306","username":"confd","password":"p@sSw0rd"}`),
				}, nil
			}
			return nil, &types.ResourceNotFoundException{}
		},
	}

	client := newTestClient(mock, "", false)

	// Request keys that don't exist directly, but their parent "database" exists with JSON
	result, err := client.GetValues(context.Background(), []string{
		"/database/host",
		"/database/port",
		"/database/username",
		"/database/password",
	})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/database/host":     "127.0.0.1",
		"/database/port":     "3306",
		"/database/username": "confd",
		"/database/password": "p@sSw0rd",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_MixedSecrets(t *testing.T) {
	// Test mix of direct secrets and JSON parent lookups
	mock := &mockSecretsManager{
		getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			switch *input.SecretId {
			case "database":
				return &secretsmanager.GetSecretValueOutput{
					SecretString: aws.String(`{"host":"db.example.com","user":"admin"}`),
				}, nil
			case "api-key":
				return &secretsmanager.GetSecretValueOutput{
					SecretString: aws.String("sk-1234567890"),
				}, nil
			}
			return nil, &types.ResourceNotFoundException{}
		},
	}

	client := newTestClient(mock, "", false)

	result, err := client.GetValues(context.Background(), []string{
		"/database/host",
		"/database/user",
		"/api-key",
	})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/database/host": "db.example.com",
		"/database/user": "admin",
		"/api-key":       "sk-1234567890",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestWatchPrefix(t *testing.T) {
	mock := &mockSecretsManager{}
	client := newTestClient(mock, "", false)

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
	mock := &mockSecretsManager{}
	client := newTestClient(mock, "", false)

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
	mock := &mockSecretsManager{}
	client := newTestClient(mock, "", false)

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

func TestHealthCheck_Success_NotFound(t *testing.T) {
	// HealthCheck uses ListSecrets - empty list is success (connectivity is working)
	mock := &mockSecretsManager{
		listSecretsFunc: func(ctx context.Context, input *secretsmanager.ListSecretsInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
			return &secretsmanager.ListSecretsOutput{
				SecretList: []types.SecretListEntry{},
			}, nil
		},
	}

	client := newTestClient(mock, "", false)

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() unexpected error: %v", err)
	}
}

func TestHealthCheck_Success_SecretExists(t *testing.T) {
	// HealthCheck uses ListSecrets - non-empty list is also success
	mock := &mockSecretsManager{
		listSecretsFunc: func(ctx context.Context, input *secretsmanager.ListSecretsInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
			return &secretsmanager.ListSecretsOutput{
				SecretList: []types.SecretListEntry{
					{Name: aws.String("test-secret")},
				},
			}, nil
		},
	}

	client := newTestClient(mock, "", false)

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() unexpected error: %v", err)
	}
}

func TestHealthCheck_Error(t *testing.T) {
	// Errors from ListSecrets should be returned
	expectedErr := errors.New("access denied")
	mock := &mockSecretsManager{
		listSecretsFunc: func(ctx context.Context, input *secretsmanager.ListSecretsInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
			return nil, expectedErr
		},
	}

	client := newTestClient(mock, "", false)

	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("HealthCheck() error = %v, want %v", err, expectedErr)
	}
}
