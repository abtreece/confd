package acm

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/acm/types"
)

// mockACM implements the acmAPI interface for testing
type mockACM struct {
	getCertificateFunc    func(ctx context.Context, input *acm.GetCertificateInput, opts ...func(*acm.Options)) (*acm.GetCertificateOutput, error)
	exportCertificateFunc func(ctx context.Context, input *acm.ExportCertificateInput, opts ...func(*acm.Options)) (*acm.ExportCertificateOutput, error)
	listCertificatesFunc  func(ctx context.Context, input *acm.ListCertificatesInput, opts ...func(*acm.Options)) (*acm.ListCertificatesOutput, error)
}

func (m *mockACM) GetCertificate(ctx context.Context, input *acm.GetCertificateInput, opts ...func(*acm.Options)) (*acm.GetCertificateOutput, error) {
	if m.getCertificateFunc != nil {
		return m.getCertificateFunc(ctx, input, opts...)
	}
	return &acm.GetCertificateOutput{}, nil
}

func (m *mockACM) ExportCertificate(ctx context.Context, input *acm.ExportCertificateInput, opts ...func(*acm.Options)) (*acm.ExportCertificateOutput, error) {
	if m.exportCertificateFunc != nil {
		return m.exportCertificateFunc(ctx, input, opts...)
	}
	return &acm.ExportCertificateOutput{}, nil
}

func (m *mockACM) ListCertificates(ctx context.Context, input *acm.ListCertificatesInput, opts ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	if m.listCertificatesFunc != nil {
		return m.listCertificatesFunc(ctx, input, opts...)
	}
	return &acm.ListCertificatesOutput{}, nil
}

// newTestClient creates a Client with a mock ACM for testing
func newTestClient(mock *mockACM) *Client {
	return &Client{
		client:           mock,
		exportPrivateKey: false,
		passphrase:       nil,
	}
}

// newTestClientWithExport creates a Client with export enabled for testing
func newTestClientWithExport(mock *mockACM, passphrase []byte) *Client {
	return &Client{
		client:           mock,
		exportPrivateKey: true,
		passphrase:       passphrase,
	}
}

func TestGetValues_SingleCertificate(t *testing.T) {
	certARN := "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"
	// confd adds a leading "/" prefix to keys
	certKey := "/" + certARN
	certPEM := "-----BEGIN CERTIFICATE-----\nMIIE...\n-----END CERTIFICATE-----"
	chainPEM := "-----BEGIN CERTIFICATE-----\nMIIF...\n-----END CERTIFICATE-----"

	mock := &mockACM{
		getCertificateFunc: func(ctx context.Context, input *acm.GetCertificateInput, opts ...func(*acm.Options)) (*acm.GetCertificateOutput, error) {
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

	result, err := client.GetValues(context.Background(), []string{certKey})
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
		getCertificateFunc: func(ctx context.Context, input *acm.GetCertificateInput, opts ...func(*acm.Options)) (*acm.GetCertificateOutput, error) {
			return &acm.GetCertificateOutput{
				Certificate:      aws.String(certPEM),
				CertificateChain: nil,
			}, nil
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{certKey})
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
		getCertificateFunc: func(ctx context.Context, input *acm.GetCertificateInput, opts ...func(*acm.Options)) (*acm.GetCertificateOutput, error) {
			if *input.CertificateArn == certARN {
				return &acm.GetCertificateOutput{
					Certificate: aws.String(certPEM),
				}, nil
			}
			return nil, errors.New("certificate not found")
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{certARN})
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
		getCertificateFunc: func(ctx context.Context, input *acm.GetCertificateInput, opts ...func(*acm.Options)) (*acm.GetCertificateOutput, error) {
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

	result, err := client.GetValues(context.Background(), []string{certKey1, certKey2})
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
		getCertificateFunc: func(ctx context.Context, input *acm.GetCertificateInput, opts ...func(*acm.Options)) (*acm.GetCertificateOutput, error) {
			return nil, errors.New("ResourceNotFoundException: certificate not found")
		},
	}

	client := newTestClient(mock)

	_, err := client.GetValues(context.Background(), []string{certKey})
	if err == nil {
		t.Fatal("GetValues() expected error, got nil")
	}
}

func TestGetValues_EmptyKeys(t *testing.T) {
	mock := &mockACM{}
	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{})
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
		listCertificatesFunc: func(ctx context.Context, input *acm.ListCertificatesInput, opts ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
			return &acm.ListCertificatesOutput{
				CertificateSummaryList: []types.CertificateSummary{
					{CertificateArn: aws.String(certARN1)},
					{CertificateArn: aws.String(certARN2)},
				},
			}, nil
		},
	}

	client := newTestClient(mock)

	result, err := client.ListCertificates(context.Background())
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

	mock := &mockACM{
		listCertificatesFunc: func(ctx context.Context, input *acm.ListCertificatesInput, opts ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
			if input.NextToken == nil {
				return &acm.ListCertificatesOutput{
					CertificateSummaryList: []types.CertificateSummary{
						{CertificateArn: aws.String(certARN1)},
					},
					NextToken: aws.String("token"),
				}, nil
			}
			return &acm.ListCertificatesOutput{
				CertificateSummaryList: []types.CertificateSummary{
					{CertificateArn: aws.String(certARN2)},
				},
				NextToken: nil,
			}, nil
		},
	}

	client := newTestClient(mock)

	result, err := client.ListCertificates(context.Background())
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
		listCertificatesFunc: func(ctx context.Context, input *acm.ListCertificatesInput, opts ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
			return &acm.ListCertificatesOutput{
				CertificateSummaryList: []types.CertificateSummary{},
			}, nil
		},
	}

	client := newTestClient(mock)

	result, err := client.ListCertificates(context.Background())
	if err != nil {
		t.Fatalf("ListCertificates() unexpected error: %v", err)
	}

	if result != nil && len(result) != 0 {
		t.Errorf("ListCertificates() = %v, want empty slice", result)
	}
}

func TestListCertificates_Error(t *testing.T) {
	mock := &mockACM{
		listCertificatesFunc: func(ctx context.Context, input *acm.ListCertificatesInput, opts ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	client := newTestClient(mock)

	_, err := client.ListCertificates(context.Background())
	if err == nil {
		t.Fatal("ListCertificates() expected error, got nil")
	}
}

func TestWatchPrefix(t *testing.T) {
	mock := &mockACM{}
	client := newTestClient(mock)

	stopChan := make(chan bool, 1)
	stopChan <- true

	waitIndex, err := client.WatchPrefix(context.Background(), "/prefix", []string{"key"}, 0, stopChan)
	if err != nil {
		t.Fatalf("WatchPrefix() unexpected error: %v", err)
	}

	if waitIndex != 0 {
		t.Errorf("WatchPrefix() waitIndex = %d, want 0", waitIndex)
	}
}

func TestGetValues_ExportPrivateKey_Success(t *testing.T) {
	certARN := "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"
	certKey := "/" + certARN
	certPEM := "-----BEGIN CERTIFICATE-----\nMIIE...\n-----END CERTIFICATE-----"
	chainPEM := "-----BEGIN CERTIFICATE-----\nMIIF...\n-----END CERTIFICATE-----"
	privateKeyPEM := "-----BEGIN ENCRYPTED PRIVATE KEY-----\nMIIE...\n-----END ENCRYPTED PRIVATE KEY-----"
	passphrase := []byte("test-passphrase")

	mock := &mockACM{
		exportCertificateFunc: func(ctx context.Context, input *acm.ExportCertificateInput, opts ...func(*acm.Options)) (*acm.ExportCertificateOutput, error) {
			// Verify the correct ARN is passed (without leading /)
			if *input.CertificateArn != certARN {
				return nil, errors.New("unexpected ARN")
			}
			// Verify passphrase is passed
			if string(input.Passphrase) != string(passphrase) {
				return nil, errors.New("unexpected passphrase")
			}
			return &acm.ExportCertificateOutput{
				Certificate:      aws.String(certPEM),
				CertificateChain: aws.String(chainPEM),
				PrivateKey:       aws.String(privateKeyPEM),
			}, nil
		},
	}

	client := newTestClientWithExport(mock, passphrase)

	result, err := client.GetValues(context.Background(), []string{certKey})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		certKey:                  certPEM,
		certKey + "_chain":       chainPEM,
		certKey + "_private_key": privateKeyPEM,
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_ExportPrivateKey_WithoutChain(t *testing.T) {
	certARN := "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"
	certKey := "/" + certARN
	certPEM := "-----BEGIN CERTIFICATE-----\nMIIE...\n-----END CERTIFICATE-----"
	privateKeyPEM := "-----BEGIN ENCRYPTED PRIVATE KEY-----\nMIIE...\n-----END ENCRYPTED PRIVATE KEY-----"
	passphrase := []byte("test-passphrase")

	mock := &mockACM{
		exportCertificateFunc: func(ctx context.Context, input *acm.ExportCertificateInput, opts ...func(*acm.Options)) (*acm.ExportCertificateOutput, error) {
			return &acm.ExportCertificateOutput{
				Certificate:      aws.String(certPEM),
				CertificateChain: nil,
				PrivateKey:       aws.String(privateKeyPEM),
			}, nil
		},
	}

	client := newTestClientWithExport(mock, passphrase)

	result, err := client.GetValues(context.Background(), []string{certKey})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		certKey:                  certPEM,
		certKey + "_private_key": privateKeyPEM,
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_ExportPrivateKey_Error(t *testing.T) {
	certKey := "/arn:aws:acm:us-east-1:123456789012:certificate/nonexistent"
	passphrase := []byte("test-passphrase")

	mock := &mockACM{
		exportCertificateFunc: func(ctx context.Context, input *acm.ExportCertificateInput, opts ...func(*acm.Options)) (*acm.ExportCertificateOutput, error) {
			return nil, errors.New("ResourceNotFoundException: certificate not found")
		},
	}

	client := newTestClientWithExport(mock, passphrase)

	_, err := client.GetValues(context.Background(), []string{certKey})
	if err == nil {
		t.Fatal("GetValues() expected error, got nil")
	}
}

func TestGetValues_ExportPrivateKey_MultipleCertificates(t *testing.T) {
	certARN1 := "arn:aws:acm:us-east-1:123456789012:certificate/cert-1"
	certARN2 := "arn:aws:acm:us-east-1:123456789012:certificate/cert-2"
	certKey1 := "/" + certARN1
	certKey2 := "/" + certARN2
	passphrase := []byte("test-passphrase")

	mock := &mockACM{
		exportCertificateFunc: func(ctx context.Context, input *acm.ExportCertificateInput, opts ...func(*acm.Options)) (*acm.ExportCertificateOutput, error) {
			switch *input.CertificateArn {
			case certARN1:
				return &acm.ExportCertificateOutput{
					Certificate: aws.String("cert1"),
					PrivateKey:  aws.String("key1"),
				}, nil
			case certARN2:
				return &acm.ExportCertificateOutput{
					Certificate: aws.String("cert2"),
					PrivateKey:  aws.String("key2"),
				}, nil
			}
			return nil, errors.New("certificate not found")
		},
	}

	client := newTestClientWithExport(mock, passphrase)

	result, err := client.GetValues(context.Background(), []string{certKey1, certKey2})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		certKey1:                  "cert1",
		certKey1 + "_private_key": "key1",
		certKey2:                  "cert2",
		certKey2 + "_private_key": "key2",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}
