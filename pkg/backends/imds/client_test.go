package imds

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

// mockIMDS implements imdsAPI interface for testing
type mockIMDS struct {
	getMetadataFunc    func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error)
	getDynamicDataFunc func(ctx context.Context, params *imds.GetDynamicDataInput, optFns ...func(*imds.Options)) (*imds.GetDynamicDataOutput, error)
	getUserDataFunc    func(ctx context.Context, params *imds.GetUserDataInput, optFns ...func(*imds.Options)) (*imds.GetUserDataOutput, error)
}

func (m *mockIMDS) GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
	if m.getMetadataFunc != nil {
		return m.getMetadataFunc(ctx, params, optFns...)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockIMDS) GetDynamicData(ctx context.Context, params *imds.GetDynamicDataInput, optFns ...func(*imds.Options)) (*imds.GetDynamicDataOutput, error) {
	if m.getDynamicDataFunc != nil {
		return m.getDynamicDataFunc(ctx, params, optFns...)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockIMDS) GetUserData(ctx context.Context, params *imds.GetUserDataInput, optFns ...func(*imds.Options)) (*imds.GetUserDataOutput, error) {
	if m.getUserDataFunc != nil {
		return m.getUserDataFunc(ctx, params, optFns...)
	}
	return nil, fmt.Errorf("not implemented")
}

// newTestClient creates a client with mock IMDS API
func newTestClient(mock *mockIMDS) *Client {
	return &Client{
		client:   mock,
		cache:    newMetadataCache(),
		cacheTTL: 60 * time.Second,
	}
}

// mockResponse creates a mock GetMetadataOutput with the given content
func mockResponse(content string) *imds.GetMetadataOutput {
	return &imds.GetMetadataOutput{
		Content: io.NopCloser(strings.NewReader(content)),
	}
}

// mockDynamicResponse creates a mock GetDynamicDataOutput with the given content
func mockDynamicResponse(content string) *imds.GetDynamicDataOutput {
	return &imds.GetDynamicDataOutput{
		Content: io.NopCloser(strings.NewReader(content)),
	}
}

// mockUserDataResponse creates a mock GetUserDataOutput with the given content
func mockUserDataResponse(content string) *imds.GetUserDataOutput {
	return &imds.GetUserDataOutput{
		Content: io.NopCloser(strings.NewReader(content)),
	}
}

func TestGetValues_SingleLeafValue(t *testing.T) {
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			if params.Path == "instance-id" {
				return mockResponse("i-1234567890abcdef0"), nil
			}
			return nil, fmt.Errorf("path not found: %s", params.Path)
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	values, err := client.GetValues(ctx, []string{"/meta-data/instance-id"})
	if err != nil {
		t.Fatalf("GetValues failed: %v", err)
	}

	expected := "i-1234567890abcdef0"
	if values["/meta-data/instance-id"] != expected {
		t.Errorf("Expected %s, got %s", expected, values["/meta-data/instance-id"])
	}
}

func TestGetValues_DirectoryListing(t *testing.T) {
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			switch params.Path {
			case "tags/instance":
				// Directory listing
				return mockResponse("Name\nEnvironment\n"), nil
			case "tags/instance/Name":
				return mockResponse("web-server"), nil
			case "tags/instance/Environment":
				return mockResponse("production"), nil
			default:
				return nil, fmt.Errorf("path not found: %s", params.Path)
			}
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	values, err := client.GetValues(ctx, []string{"/meta-data/tags/instance/"})
	if err != nil {
		t.Fatalf("GetValues failed: %v", err)
	}

	if values["/meta-data/tags/instance/Name"] != "web-server" {
		t.Errorf("Expected web-server, got %s", values["/meta-data/tags/instance/Name"])
	}
	if values["/meta-data/tags/instance/Environment"] != "production" {
		t.Errorf("Expected production, got %s", values["/meta-data/tags/instance/Environment"])
	}
}

func TestGetValues_NetworkInterfaces(t *testing.T) {
	mac := "02:00:00:00:00:00"
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			switch params.Path {
			case "network/interfaces/macs":
				return mockResponse(mac + "/\n"), nil
			case "network/interfaces/macs/" + mac:
				return mockResponse("local-ipv4\nsubnet-id\n"), nil
			case "network/interfaces/macs/" + mac + "/local-ipv4":
				return mockResponse("10.0.1.5"), nil
			case "network/interfaces/macs/" + mac + "/subnet-id":
				return mockResponse("subnet-12345"), nil
			default:
				return nil, fmt.Errorf("path not found: %s", params.Path)
			}
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	values, err := client.GetValues(ctx, []string{"/meta-data/network/interfaces/macs/"})
	if err != nil {
		t.Fatalf("GetValues failed: %v", err)
	}

	expectedIP := "10.0.1.5"
	key := "/meta-data/network/interfaces/macs/" + mac + "/local-ipv4"
	if values[key] != expectedIP {
		t.Errorf("Expected %s for key %s, got %s", expectedIP, key, values[key])
	}

	expectedSubnet := "subnet-12345"
	key = "/meta-data/network/interfaces/macs/" + mac + "/subnet-id"
	if values[key] != expectedSubnet {
		t.Errorf("Expected %s for key %s, got %s", expectedSubnet, key, values[key])
	}
}

func TestGetValues_DynamicData(t *testing.T) {
	jsonDoc := `{"instanceId":"i-1234567890abcdef0","region":"us-east-1"}`
	mock := &mockIMDS{
		getDynamicDataFunc: func(ctx context.Context, params *imds.GetDynamicDataInput, optFns ...func(*imds.Options)) (*imds.GetDynamicDataOutput, error) {
			if params.Path == "instance-identity/document" {
				return mockDynamicResponse(jsonDoc), nil
			}
			return nil, fmt.Errorf("path not found: %s", params.Path)
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	values, err := client.GetValues(ctx, []string{"/dynamic/instance-identity/document"})
	if err != nil {
		t.Fatalf("GetValues failed: %v", err)
	}

	if values["/dynamic/instance-identity/document"] != jsonDoc {
		t.Errorf("Expected JSON document, got %s", values["/dynamic/instance-identity/document"])
	}
}

func TestGetValues_UserData(t *testing.T) {
	userData := "#!/bin/bash\necho 'Hello World'"
	mock := &mockIMDS{
		getUserDataFunc: func(ctx context.Context, params *imds.GetUserDataInput, optFns ...func(*imds.Options)) (*imds.GetUserDataOutput, error) {
			return mockUserDataResponse(userData), nil
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	values, err := client.GetValues(ctx, []string{"/user-data"})
	if err != nil {
		t.Fatalf("GetValues failed: %v", err)
	}

	if values["/user-data"] != userData {
		t.Errorf("Expected user data script, got %s", values["/user-data"])
	}
}

func TestGetValues_MultipleKeys(t *testing.T) {
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			switch params.Path {
			case "instance-id":
				return mockResponse("i-1234567890abcdef0"), nil
			case "instance-type":
				return mockResponse("t3.micro"), nil
			case "placement/availability-zone":
				return mockResponse("us-east-1a"), nil
			default:
				return nil, fmt.Errorf("path not found: %s", params.Path)
			}
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	keys := []string{
		"/meta-data/instance-id",
		"/meta-data/instance-type",
		"/meta-data/placement/availability-zone",
	}

	values, err := client.GetValues(ctx, keys)
	if err != nil {
		t.Fatalf("GetValues failed: %v", err)
	}

	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}

	if values["/meta-data/instance-id"] != "i-1234567890abcdef0" {
		t.Errorf("Unexpected instance-id: %s", values["/meta-data/instance-id"])
	}
	if values["/meta-data/instance-type"] != "t3.micro" {
		t.Errorf("Unexpected instance-type: %s", values["/meta-data/instance-type"])
	}
	if values["/meta-data/placement/availability-zone"] != "us-east-1a" {
		t.Errorf("Unexpected availability-zone: %s", values["/meta-data/placement/availability-zone"])
	}
}

func TestGetValues_CacheHit(t *testing.T) {
	callCount := 0
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			callCount++
			if params.Path == "instance-id" {
				return mockResponse("i-1234567890abcdef0"), nil
			}
			return nil, fmt.Errorf("path not found: %s", params.Path)
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	// First call - should hit IMDS
	_, err := client.GetValues(ctx, []string{"/meta-data/instance-id"})
	if err != nil {
		t.Fatalf("First GetValues failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 IMDS call, got %d", callCount)
	}

	// Second call - should hit cache
	_, err = client.GetValues(ctx, []string{"/meta-data/instance-id"})
	if err != nil {
		t.Fatalf("Second GetValues failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 IMDS call (cached), got %d", callCount)
	}
}

func TestGetValues_CacheExpiration(t *testing.T) {
	callCount := 0
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			callCount++
			if params.Path == "instance-id" {
				return mockResponse("i-1234567890abcdef0"), nil
			}
			return nil, fmt.Errorf("path not found: %s", params.Path)
		},
	}

	// Create client with very short TTL
	client := newTestClient(mock)
	client.cacheTTL = 10 * time.Millisecond
	ctx := context.Background()

	// First call
	_, err := client.GetValues(ctx, []string{"/meta-data/instance-id"})
	if err != nil {
		t.Fatalf("First GetValues failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 IMDS call, got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(20 * time.Millisecond)

	// Second call - cache should be expired
	_, err = client.GetValues(ctx, []string{"/meta-data/instance-id"})
	if err != nil {
		t.Fatalf("Second GetValues failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 IMDS calls (cache expired), got %d", callCount)
	}
}

func TestGetValues_PathNormalization(t *testing.T) {
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			if params.Path == "instance-id" {
				return mockResponse("i-1234567890abcdef0"), nil
			}
			return nil, fmt.Errorf("path not found: %s", params.Path)
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	testCases := []string{
		"/meta-data/instance-id",
		"/latest/meta-data/instance-id",
		"meta-data/instance-id",
		"latest/meta-data/instance-id",
	}

	for _, key := range testCases {
		values, err := client.GetValues(ctx, []string{key})
		if err != nil {
			t.Fatalf("GetValues failed for key %s: %v", key, err)
		}

		// The result should always be under the normalized key format
		found := false
		for k := range values {
			if strings.Contains(k, "instance-id") {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Expected to find instance-id in result for key %s, got keys: %v", key, values)
		}
	}
}

func TestGetValues_NotFound(t *testing.T) {
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			return nil, fmt.Errorf("path not found: %s", params.Path)
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	values, err := client.GetValues(ctx, []string{"/meta-data/nonexistent"})
	if err != nil {
		t.Fatalf("GetValues failed: %v", err)
	}

	// Should return empty map, not error
	if len(values) != 0 {
		t.Errorf("Expected empty result for nonexistent key, got %v", values)
	}
}

func TestHealthCheck_Success(t *testing.T) {
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			if params.Path == "" {
				return mockResponse("ami-id\ninstance-id\n"), nil
			}
			return nil, fmt.Errorf("path not found")
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	err := client.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestHealthCheck_Unavailable(t *testing.T) {
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			return nil, fmt.Errorf("IMDS unavailable")
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	err := client.HealthCheck(ctx)
	if err == nil {
		t.Error("HealthCheck should have failed")
	}
}

func TestHealthCheckDetailed_Success(t *testing.T) {
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			if params.Path == "" {
				return mockResponse("ami-id\ninstance-id\n"), nil
			}
			return nil, fmt.Errorf("path not found")
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	result, err := client.HealthCheckDetailed(ctx)
	if err != nil {
		t.Fatalf("HealthCheckDetailed failed: %v", err)
	}

	if !result.Healthy {
		t.Error("Expected healthy status")
	}

	if _, ok := result.Details["cache_entries"]; !ok {
		t.Error("Expected cache_entries in details")
	}

	if _, ok := result.Details["cache_ttl"]; !ok {
		t.Error("Expected cache_ttl in details")
	}
}

func TestHealthCheckDetailed_Unavailable(t *testing.T) {
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			return nil, fmt.Errorf("IMDS unavailable")
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	result, err := client.HealthCheckDetailed(ctx)
	if err != nil {
		t.Fatalf("HealthCheckDetailed failed: %v", err)
	}

	if result.Healthy {
		t.Error("Expected unhealthy status")
	}

	if !strings.Contains(result.Message, "IMDS unavailable") {
		t.Errorf("Expected error message in result, got: %s", result.Message)
	}
}

func TestWatchPrefix(t *testing.T) {
	client := newTestClient(&mockIMDS{})
	ctx := context.Background()
	stopChan := make(chan bool)

	// Start WatchPrefix in goroutine
	done := make(chan bool)
	go func() {
		_, err := client.WatchPrefix(ctx, "/meta-data", nil, 0, stopChan)
		if err != nil {
			t.Errorf("WatchPrefix returned error: %v", err)
		}
		done <- true
	}()

	// Signal stop
	stopChan <- true

	// Wait for completion
	<-done
}

func TestConcurrentAccess(t *testing.T) {
	callCount := 0
	mock := &mockIMDS{
		getMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			callCount++
			time.Sleep(1 * time.Millisecond) // Simulate network delay
			if params.Path == "instance-id" {
				return mockResponse("i-1234567890abcdef0"), nil
			}
			return nil, fmt.Errorf("path not found: %s", params.Path)
		},
	}

	client := newTestClient(mock)
	ctx := context.Background()

	// Launch multiple concurrent requests
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := client.GetValues(ctx, []string{"/meta-data/instance-id"})
			if err != nil {
				t.Errorf("GetValues failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// With concurrent requests, all may check cache before any populate it
	// This is expected behavior - cache works for subsequent requests
	// We just verify that the operations complete successfully
	t.Logf("Concurrent requests made %d IMDS calls", callCount)

	// Verify cache works for a subsequent request
	callCountBefore := callCount
	_, err := client.GetValues(ctx, []string{"/meta-data/instance-id"})
	if err != nil {
		t.Errorf("Subsequent GetValues failed: %v", err)
	}
	if callCount > callCountBefore {
		t.Errorf("Cache not working: subsequent request made another IMDS call")
	}
}

func TestClose(t *testing.T) {
	client := newTestClient(&mockIMDS{})
	err := client.Close()
	if err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}
}
