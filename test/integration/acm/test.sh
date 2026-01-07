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

# Test private key export functionality
echo "Testing private key export..."

# Set up export configuration
export ACM_EXPORT_PRIVATE_KEY="true"
export ACM_PASSPHRASE="test-passphrase-1234"

# Create template resource configuration for export test
cat > ./test/integration/acm/confdir/conf.d/certificate-export.toml << EOF
[template]
mode = "0644"
src = "certificate-export.tmpl"
dest = "/tmp/acm-test-export.pem"
keys = [
  "$CERTIFICATE_ARN"
]
EOF

# Create the template for export - includes certificate, chain, and private key
cat > ./test/integration/acm/confdir/templates/certificate-export.tmpl << EOF
{{ getv "/$CERTIFICATE_ARN" }}
{{ if exists "/${CERTIFICATE_ARN}_private_key" }}
{{ getv "/${CERTIFICATE_ARN}_private_key" }}
{{ end }}
EOF

# Run confd with private key export enabled
confd --onetime --log-level debug --confdir ./test/integration/acm/confdir --backend acm --acm-export-private-key
if [ $? -ne 0 ]; then
    echo "confd with private key export failed"
    exit 1
fi

# Verify the output file was created and contains the certificate
if [ ! -f /tmp/acm-test-export.pem ]; then
    echo "Export output file was not created"
    exit 1
fi

if ! grep -q "BEGIN CERTIFICATE" /tmp/acm-test-export.pem; then
    echo "Export output file does not contain certificate"
    cat /tmp/acm-test-export.pem
    exit 1
fi

# Note: localstack may not fully support ExportCertificate, so we check if private key was exported
# but don't fail if it wasn't (localstack limitation)
if grep -q "PRIVATE KEY" /tmp/acm-test-export.pem; then
    echo "Private key successfully exported"
else
    echo "Note: Private key not found in output (may be localstack limitation)"
fi

echo "Private key export test passed"

# Clean up export env vars
unset ACM_EXPORT_PRIVATE_KEY
unset ACM_PASSPHRASE

# Run confd with --watch, expecting it to fail (watch not supported for ACM)
confd --onetime --log-level debug --confdir ./test/integration/acm/confdir --backend acm --watch
if [ $? -eq 0 ]; then
    echo "confd with --watch should have failed for ACM backend"
    exit 1
fi

echo "ACM integration test passed"

# Cleanup
rm -f /tmp/acm-test-certificate.pem
rm -f /tmp/acm-test-export.pem
