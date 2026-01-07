#!/bin/bash -x

export HOSTNAME="localhost"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-1}"
export AWS_REGION="${AWS_REGION:-us-east-1}"
export ACM_LOCAL="true"
export ACM_ENDPOINT_URL="${ACM_ENDPOINT_URL:-http://localhost:4566}"

# Create a temporary directory for certificates
CERT_DIR=$(mktemp -d)
trap "rm -rf $CERT_DIR" EXIT

# Generate a self-signed certificate for testing
openssl req -x509 -newkey rsa:2048 -keyout "$CERT_DIR/key.pem" -out "$CERT_DIR/cert.pem" -days 1 -nodes \
    -subj "/C=US/ST=Test/L=Test/O=Test/CN=test.example.com" 2>/dev/null

# Import the certificate into ACM
CERTIFICATE_ARN=$(aws acm import-certificate \
    --certificate fileb://"$CERT_DIR/cert.pem" \
    --private-key fileb://"$CERT_DIR/key.pem" \
    --endpoint-url "$ACM_ENDPOINT_URL" \
    --query 'CertificateArn' \
    --output text)

if [ -z "$CERTIFICATE_ARN" ]; then
    echo "Failed to import certificate"
    exit 1
fi

echo "Imported certificate with ARN: $CERTIFICATE_ARN"

# Create the confd configuration directory
mkdir -p ./test/integration/acm/confdir/conf.d
mkdir -p ./test/integration/acm/confdir/templates

# Create template resource configuration with the certificate ARN
cat > ./test/integration/acm/confdir/conf.d/certificate.toml << EOF
[template]
mode = "0644"
src = "certificate.tmpl"
dest = "/tmp/acm-test-certificate.pem"
keys = [
  "$CERTIFICATE_ARN"
]
EOF

# Create the template - use the ARN key directly
cat > ./test/integration/acm/confdir/templates/certificate.tmpl << EOF
{{ getv "/$CERTIFICATE_ARN" }}
EOF

# Run confd, expect it to work
confd --onetime --log-level debug --confdir ./test/integration/acm/confdir --backend acm
if [ $? -ne 0 ]; then
    echo "confd failed"
    exit 1
fi

# Verify the output file was created and contains the certificate
if [ ! -f /tmp/acm-test-certificate.pem ]; then
    echo "Output file was not created"
    exit 1
fi

if ! grep -q "BEGIN CERTIFICATE" /tmp/acm-test-certificate.pem; then
    echo "Output file does not contain certificate"
    cat /tmp/acm-test-certificate.pem
    exit 1
fi

echo "Certificate successfully retrieved and written"

# Run confd with --watch, expecting it to fail (watch not supported for ACM)
confd --onetime --log-level debug --confdir ./test/integration/acm/confdir --backend acm --watch
if [ $? -eq 0 ]; then
    echo "confd with --watch should have failed for ACM backend"
    exit 1
fi

echo "ACM integration test passed"

# Cleanup
rm -f /tmp/acm-test-certificate.pem
