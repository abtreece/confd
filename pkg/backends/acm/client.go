package acm

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"
)

// acmAPI defines the interface for ACM operations used by this client
type acmAPI interface {
	GetCertificate(input *acm.GetCertificateInput) (*acm.GetCertificateOutput, error)
	ListCertificatesPages(input *acm.ListCertificatesInput, fn func(*acm.ListCertificatesOutput, bool) bool) error
}

type Client struct {
	client acmAPI
}

// New initializes the AWS ACM backend for confd
func New() (*Client, error) {
	// Attempt to get AWS Region from ec2metadata with a timeout
	metaSession, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	metaClient := ec2metadata.New(metaSession, aws.NewConfig().WithHTTPClient(&http.Client{Timeout: 2 * time.Second}))
	var region string

	if os.Getenv("AWS_REGION") != "" {
		region = os.Getenv("AWS_REGION")
	} else {
		region, err = metaClient.Region()
		if err != nil {
			return nil, fmt.Errorf("failed to get region from EC2 metadata: %w", err)
		}
	}

	conf := aws.NewConfig().WithRegion(region)

	// Create a session to share configuration, and load external configuration.
	sess := session.Must(session.NewSessionWithOptions(
		session.Options{
			SharedConfigState: session.SharedConfigEnable,
			Config:            *conf,
		},
	))

	log.Debug("Region: %s", aws.StringValue(sess.Config.Region))

	// Fail early, if no credentials can be found
	_, err = sess.Config.Credentials.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS credentials: %w", err)
	}

	var c *aws.Config
	if os.Getenv("ACM_LOCAL") != "" {
		log.Debug("ACM_LOCAL is set")
		endpoint := os.Getenv("ACM_ENDPOINT_URL")
		c = &aws.Config{
			Endpoint: &endpoint,
		}
	}

	svc := acm.New(sess, c)
	return &Client{client: svc}, nil
}

func (c *Client) GetValues(keys []string) (map[string]string, error) {
	vars := make(map[string]string)

	for _, key := range keys {
		// Strip leading slash added by confd's prefix handling
		// ARNs should start with "arn:" not "/arn:"
		arn := strings.TrimPrefix(key, "/")

		input := &acm.GetCertificateInput{
			CertificateArn: aws.String(arn),
		}

		result, err := c.client.GetCertificate(input)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve certificate: %w", err)
		}

		// Use the original key (with prefix) for the return map
		// so confd template functions work correctly
		if result.Certificate != nil {
			vars[key] = *result.Certificate
		}

		if result.CertificateChain != nil {
			vars[key+"_chain"] = *result.CertificateChain
		}

		log.Debug("Retrieved certificate for ARN: %s", arn)
	}

	return vars, nil
}

func (c *Client) ListCertificates() ([]string, error) {
	var certs []string

	input := &acm.ListCertificatesInput{}
	err := c.client.ListCertificatesPages(input, func(page *acm.ListCertificatesOutput, lastPage bool) bool {
		for _, cert := range page.CertificateSummaryList {
			certs = append(certs, aws.StringValue(cert.CertificateArn))
		}
		return !lastPage
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}

	return certs, nil
}

// WatchPrefix is not implemented for ACM
func (c *Client) WatchPrefix(prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	<-stopChan
	return 0, nil
}
