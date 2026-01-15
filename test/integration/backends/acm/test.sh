#!/bin/bash
set -e

export HOSTNAME="localhost"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-1}"
export AWS_REGION="${AWS_REGION:-us-east-1}"
export ACM_LOCAL="true"
export ACM_ENDPOINT_URL="${ACM_ENDPOINT_URL:-http://localhost:4566}"

# Wait for LocalStack/ACM to be ready
wait_for_acm() {
    local retries=30
    while ! aws acm list-certificates --endpoint-url "$ACM_ENDPOINT_URL" > /dev/null 2>&1; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            echo "ERROR: ACM not ready after 30 seconds" >&2
            exit 1
        fi
        sleep 1
    done
}

wait_for_acm

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

if [[ -z "$CERTIFICATE_ARN" ]]; then
    echo "ERROR: Failed to import certificate"
    exit 1
fi

echo "Imported certificate with ARN: $CERTIFICATE_ARN"

# Clean up any existing test directories
rm -rf ./test/integration/acm/confdir
rm -rf ./test/integration/acm/confdir-export

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
confd acm --onetime --log-level debug --confdir ./test/integration/acm/confdir

# Verify the output file was created and contains the certificate
if [[ ! -f /tmp/acm-test-certificate.pem ]]; then
    echo "ERROR: Output file was not created"
    exit 1
fi

if ! grep -q "BEGIN CERTIFICATE" /tmp/acm-test-certificate.pem; then
    echo "ERROR: Output file does not contain certificate"
    cat /tmp/acm-test-certificate.pem
    exit 1
fi

echo "Certificate successfully retrieved and written"

# Test private key export functionality (informational only)
# Note: localstack doesn't fully support ExportCertificate for imported certificates
# (only works for AWS Private CA certs). We test that the flag works but don't fail
# if localstack returns an error. Unit tests verify the actual functionality.
echo "Testing private key export configuration..."

# Set up export configuration
export ACM_EXPORT_PRIVATE_KEY="true"
export ACM_PASSPHRASE="test-passphrase-1234"

# Create a separate confdir for export test to avoid affecting other tests
mkdir -p ./test/integration/acm/confdir-export/conf.d
mkdir -p ./test/integration/acm/confdir-export/templates

# Create template resource configuration for export test
cat > ./test/integration/acm/confdir-export/conf.d/certificate-export.toml << EOF
[template]
mode = "0644"
src = "certificate-export.tmpl"
dest = "/tmp/acm-test-export.pem"
keys = [
  "$CERTIFICATE_ARN"
]
EOF

# Create the template for export
cat > ./test/integration/acm/confdir-export/templates/certificate-export.tmpl << EOF
{{ getv "/$CERTIFICATE_ARN" }}
{{ if exists "/${CERTIFICATE_ARN}_private_key" }}
{{ getv "/${CERTIFICATE_ARN}_private_key" }}
{{ end }}
EOF

# Run confd with private key export enabled
# This may fail with localstack due to ExportCertificate limitations
if confd acm --onetime --log-level debug --confdir ./test/integration/acm/confdir-export --acm-export-private-key; then
    echo "Private key export succeeded"
    if [[ -f /tmp/acm-test-export.pem ]]; then
        if grep -q "PRIVATE KEY" /tmp/acm-test-export.pem; then
            echo "Private key found in output"
        else
            echo "Certificate exported but no private key (expected for non-Private CA certs)"
        fi
    fi
else
    echo "Note: Private key export not supported by localstack for imported certificates"
    echo "This is expected - ExportCertificate only works for AWS Private CA certificates"
    echo "Unit tests verify the export functionality works correctly"
fi

# Clean up export env vars and files
unset ACM_EXPORT_PRIVATE_KEY
unset ACM_PASSPHRASE
rm -rf ./test/integration/acm/confdir-export

# Run confd with --watch, expecting it to fail (watch not supported for ACM)
if confd acm --onetime --log-level debug --confdir ./test/integration/acm/confdir --watch 2>/dev/null; then
    echo "ERROR: confd with --watch should have failed for ACM backend"
    exit 1
fi
echo "OK: --watch correctly rejected for ACM"

echo "ACM integration test passed"

# Cleanup
rm -f /tmp/acm-test-certificate.pem
rm -f /tmp/acm-test-export.pem
rm -rf ./test/integration/acm/confdir
