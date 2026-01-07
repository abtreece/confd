package acm

import (
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"
)

// mockACM implements the acmAPI interface for testing
type mockACM struct {
	getCertificateFunc        func(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error)
	listCertificatesPagesFunc func(input *acm.ListCertificatesInput, fn func(*acm.ListCertificatesOutput, bool) bool) error
}

func (m *mockACM) GetCertificate(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error) {
	if m.getCertificateFunc != nil {
		return m.getCertificateFunc(input)
	}
	return &acm.GetCertificateOutput{}, nil
}

func (m *mockACM) ListCertificatesPages(input *acm.ListCertificatesInput, fn func(*acm.ListCertificatesOutput, bool) bool) error {
	if m.listCertificatesPagesFunc != nil {
		return m.listCertificatesPagesFunc(input, fn)
	}
	return nil
}

// newTestClient creates a Client with a mock ACM for testing
func newTestClient(mock *mockACM) *Client {
	return &Client{
		client: mock,
	}
}

func TestGetValues_SingleCertificate(t *testing.T) {
	certARN := "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"
	// confd adds a leading "/" prefix to keys
	certKey := "/" + certARN
	certPEM := "-----BEGIN CERTIFICATE-----\nMIIE...\n-----END CERTIFICATE-----"
	chainPEM := "-----BEGIN CERTIFICATE-----\nMIIF...\n-----END CERTIFICATE-----"

	mock := &mockACM{
		getCertificateFunc: func(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error) {
			// The client should strip the leading "/" before calling AWS
			if *input.CertificateArn == certARN {
				return &acm.GetCertificateOutput{
					Certificate:      aws.String(certPEM),
					CertificateChain: aws.String(chainPEM),
				}, nil
			}
			return nil, errors.New("certificate not found")
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues([]string{certKey})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Result keys should preserve the original key (with prefix)
	expected := map[string]string{
		certKey:            certPEM,
		certKey + "_chain": chainPEM,
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_CertificateWithoutChain(t *testing.T) {
	certARN := "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"
	certKey := "/" + certARN
	certPEM := "-----BEGIN CERTIFICATE-----\nMIIE...\n-----END CERTIFICATE-----"

	mock := &mockACM{
		getCertificateFunc: func(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error) {
			return &acm.GetCertificateOutput{
				Certificate:      aws.String(certPEM),
				CertificateChain: nil,
			}, nil
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues([]string{certKey})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		certKey: certPEM,
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_WithoutPrefix(t *testing.T) {
	// Test that ARNs without leading "/" also work
	certARN := "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"
	certPEM := "-----BEGIN CERTIFICATE-----\nMIIE...\n-----END CERTIFICATE-----"

	mock := &mockACM{
		getCertificateFunc: func(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error) {
			if *input.CertificateArn == certARN {
				return &acm.GetCertificateOutput{
					Certificate: aws.String(certPEM),
				}, nil
			}
			return nil, errors.New("certificate not found")
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues([]string{certARN})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		certARN: certPEM,
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_MultipleCertificates(t *testing.T) {
	certARN1 := "arn:aws:acm:us-east-1:123456789012:certificate/cert-1"
	certARN2 := "arn:aws:acm:us-east-1:123456789012:certificate/cert-2"
	certKey1 := "/" + certARN1
	certKey2 := "/" + certARN2
	certPEM1 := "-----BEGIN CERTIFICATE-----\ncert1\n-----END CERTIFICATE-----"
	certPEM2 := "-----BEGIN CERTIFICATE-----\ncert2\n-----END CERTIFICATE-----"

	mock := &mockACM{
		getCertificateFunc: func(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error) {
			switch *input.CertificateArn {
			case certARN1:
				return &acm.GetCertificateOutput{
					Certificate: aws.String(certPEM1),
				}, nil
			case certARN2:
				return &acm.GetCertificateOutput{
					Certificate: aws.String(certPEM2),
				}, nil
			}
			return nil, errors.New("certificate not found")
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues([]string{certKey1, certKey2})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		certKey1: certPEM1,
		certKey2: certPEM2,
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_Error(t *testing.T) {
	certKey := "/arn:aws:acm:us-east-1:123456789012:certificate/nonexistent"

	mock := &mockACM{
		getCertificateFunc: func(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error) {
			return nil, errors.New("ResourceNotFoundException: certificate not found")
		},
	}

	client := newTestClient(mock)

	_, err := client.GetValues([]string{certKey})
	if err == nil {
		t.Fatal("GetValues() expected error, got nil")
	}
}

func TestGetValues_EmptyKeys(t *testing.T) {
	mock := &mockACM{}
	client := newTestClient(mock)

	result, err := client.GetValues([]string{})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("GetValues() = %v, want empty map", result)
	}
}

func TestListCertificates(t *testing.T) {
	certARN1 := "arn:aws:acm:us-east-1:123456789012:certificate/cert-1"
	certARN2 := "arn:aws:acm:us-east-1:123456789012:certificate/cert-2"

	mock := &mockACM{
		listCertificatesPagesFunc: func(input *acm.ListCertificatesInput, fn func(*acm.ListCertificatesOutput, bool) bool) error {
			fn(&acm.ListCertificatesOutput{
				CertificateSummaryList: []*acm.CertificateSummary{
					{CertificateArn: aws.String(certARN1)},
					{CertificateArn: aws.String(certARN2)},
				},
			}, true)
			return nil
		},
	}

	client := newTestClient(mock)

	result, err := client.ListCertificates()
	if err != nil {
		t.Fatalf("ListCertificates() unexpected error: %v", err)
	}

	expected := []string{certARN1, certARN2}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("ListCertificates() = %v, want %v", result, expected)
	}
}

func TestListCertificates_Pagination(t *testing.T) {
	certARN1 := "arn:aws:acm:us-east-1:123456789012:certificate/cert-1"
	certARN2 := "arn:aws:acm:us-east-1:123456789012:certificate/cert-2"

	callCount := 0
	mock := &mockACM{
		listCertificatesPagesFunc: func(input *acm.ListCertificatesInput, fn func(*acm.ListCertificatesOutput, bool) bool) error {
			// Simulate pagination with two pages
			fn(&acm.ListCertificatesOutput{
				CertificateSummaryList: []*acm.CertificateSummary{
					{CertificateArn: aws.String(certARN1)},
				},
			}, false)
			callCount++
			fn(&acm.ListCertificatesOutput{
				CertificateSummaryList: []*acm.CertificateSummary{
					{CertificateArn: aws.String(certARN2)},
				},
			}, true)
			callCount++
			return nil
		},
	}

	client := newTestClient(mock)

	result, err := client.ListCertificates()
	if err != nil {
		t.Fatalf("ListCertificates() unexpected error: %v", err)
	}

	expected := []string{certARN1, certARN2}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("ListCertificates() = %v, want %v", result, expected)
	}
}

func TestListCertificates_Empty(t *testing.T) {
	mock := &mockACM{
		listCertificatesPagesFunc: func(input *acm.ListCertificatesInput, fn func(*acm.ListCertificatesOutput, bool) bool) error {
			fn(&acm.ListCertificatesOutput{
				CertificateSummaryList: []*acm.CertificateSummary{},
			}, true)
			return nil
		},
	}

	client := newTestClient(mock)

	result, err := client.ListCertificates()
	if err != nil {
		t.Fatalf("ListCertificates() unexpected error: %v", err)
	}

	if result != nil && len(result) != 0 {
		t.Errorf("ListCertificates() = %v, want empty slice", result)
	}
}

func TestListCertificates_Error(t *testing.T) {
	mock := &mockACM{
		listCertificatesPagesFunc: func(input *acm.ListCertificatesInput, fn func(*acm.ListCertificatesOutput, bool) bool) error {
			return errors.New("access denied")
		},
	}

	client := newTestClient(mock)

	_, err := client.ListCertificates()
	if err == nil {
		t.Fatal("ListCertificates() expected error, got nil")
	}
}

func TestWatchPrefix(t *testing.T) {
	mock := &mockACM{}
	client := newTestClient(mock)

	stopChan := make(chan bool, 1)
	stopChan <- true

	waitIndex, err := client.WatchPrefix("/prefix", []string{"key"}, 0, stopChan)
	if err != nil {
		t.Fatalf("WatchPrefix() unexpected error: %v", err)
	}

	if waitIndex != 0 {
		t.Errorf("WatchPrefix() waitIndex = %d, want 0", waitIndex)
	}
}
