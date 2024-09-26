package acm

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/kelseyhightower/confd/log"
)

type Client struct {
	Client *acm.ACM
}

// NewAWSACMBackend initializes the AWS ACM backend for confd
func New() (*Client, error) {
	// Attempt to get AWS Region from ec2metadata. Should determine how to
	// shorten ec2metadata client timeout so it fails fast if not on EC2.
	metaSession, _ := session.NewSession()
	metaClient := ec2metadata.New(metaSession)
	var region string

	if os.Getenv("AWS_REGION") != "" {
		region = os.Getenv("AWS_REGION")
	} else {
		region, _ = metaClient.Region()
	}

	conf := aws.NewConfig().WithRegion(region)

	client := acm.New(sess)
	return &Client{
		Client: client,
	}, nil
}

// GetCertificate retrieves an existing certificate from AWS ACM
func (c *Client) GetCertificate(arn string) (*acm.GetCertificateOutput, error) {
	input := &acm.GetCertificateInput{
		CertificateArn: aws.String(arn),
	}
	result, err := b.Client.GetCertificate(input)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve certificate: %w", err)
	}
	return result, nil
}

// RequestCertificate requests a new certificate from AWS ACM
func (c *Client) RequestCertificate(domainName string, validationMethod string) (*acm.RequestCertificateOutput, error) {
	input := &acm.RequestCertificateInput{
		DomainName:       aws.String(domainName),
		ValidationMethod: aws.String(validationMethod),
	}
	result, err := b.Client.RequestCertificate(input)
	if err != nil {
		return nil, fmt.Errorf("failed to request certificate: %w", err)
	}
	return result, nil
}

// Load loads configuration values using the AWS ACM backend
func (c *Client) Load(ctx context.Context, s template.Store) error {
	certificateARN := s.Get("certificate_arn")
	if certificateARN == "" {
		return fmt.Errorf("certificate ARN not found in store")
	}

	// Get the certificate from ACM
	cert, err := b.GetCertificate(certificateARN)
	if err != nil {
		return fmt.Errorf("failed to load certificate from AWS ACM: %w", err)
	}

	// Store certificate in confd store
	s.Set("tls_certificate", cert.Certificate)
	if cert.CertificateChain != nil {
		s.Set("tls_certificate_chain", *cert.CertificateChain)
	}
	return nil
}

func main() {
	// Example usage of the backend
	backend, err := Client("us-east-1")
	if err != nil {
		log.Fatal(err.Error())
	}

	// Request a certificate (if necessary)
	certOutput, err := backend.RequestCertificate("example.com", "DNS")
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Info("Requested certificate ARN:", *certOutput.CertificateArn)

	// Load the certificate
	store := template.NewMemStore()
	err = backend.Load(context.Background(), store)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Info("Certificate loaded successfully.")
}
