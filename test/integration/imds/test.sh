#!/bin/bash
set -e

# Integration test for IMDS backend
# This test verifies that confd can successfully use the IMDS backend

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFDIR="${SCRIPT_DIR}/confdir"
OUTPUT_FILE="/tmp/confd-imds-test-instance.conf"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

cleanup() {
    log_info "Cleaning up test artifacts..."
    rm -f "${OUTPUT_FILE}"
}

trap cleanup EXIT

# Check if running on EC2
check_ec2() {
    log_info "Checking if running on EC2 instance..."

    # Prefer IMDSv2 (token-based); fall back to IMDSv1 if token is not available
    local token
    token="$(curl -s -m 2 -X PUT "http://169.254.169.254/latest/api/token" \
        -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null || true)"

    if [[ -n "${token}" ]]; then
        # IMDSv2: use the token to query instance-id
        if curl -s -m 2 -f -H "X-aws-ec2-metadata-token: ${token}" \
            "http://169.254.169.254/latest/meta-data/instance-id" > /dev/null 2>&1; then
            log_info "Running on EC2 instance - IMDSv2 available"
            return 0
        fi
    fi

    # Fallback to IMDSv1 (no token). This may fail if HttpTokens=required.
    if curl -s -m 2 -f "http://169.254.169.254/latest/meta-data/instance-id" > /dev/null 2>&1; then
        log_info "Running on EC2 instance - IMDSv1 available"
        return 0
    fi

    log_warn "Not running on EC2 or IMDS not available"
    return 1
}

# Test basic functionality
test_basic() {
    log_info "Testing basic IMDS backend functionality..."

    # Build confd if not already built
    if [[ ! -f "${SCRIPT_DIR}/../../../bin/confd" ]]; then
        log_info "Building confd..."
        cd "${SCRIPT_DIR}/../../.."
        make build
        cd "${SCRIPT_DIR}"
    fi

    CONFD="${SCRIPT_DIR}/../../../bin/confd"

    # Run confd with IMDS backend in one-time mode
    log_info "Running confd with IMDS backend..."
    "${CONFD}" imds \
        --confdir "${CONFDIR}" \
        --onetime \
        --log-level debug

    # Check if output file was created
    if [[ ! -f "${OUTPUT_FILE}" ]]; then
        log_error "Output file not created: ${OUTPUT_FILE}"
        return 1
    fi

    log_info "Output file created successfully"
    log_info "Contents:"
    cat "${OUTPUT_FILE}"

    # Validate output contains expected fields
    if ! grep -q "instance_id=" "${OUTPUT_FILE}"; then
        log_error "Output file missing instance_id"
        return 1
    fi

    if ! grep -q "instance_type=" "${OUTPUT_FILE}"; then
        log_error "Output file missing instance_type"
        return 1
    fi

    if ! grep -q "availability_zone=" "${OUTPUT_FILE}"; then
        log_error "Output file missing availability_zone"
        return 1
    fi

    log_info "Basic test passed!"
    return 0
}

# Test cache behavior
test_cache() {
    log_info "Testing IMDS cache behavior..."

    CONFD="${SCRIPT_DIR}/../../../bin/confd"

    # Run with very short cache TTL
    log_info "Running with 1s cache TTL..."
    "${CONFD}" imds \
        --confdir "${CONFDIR}" \
        --onetime \
        --imds-cache-ttl 1s \
        --log-level debug

    if [[ ! -f "${OUTPUT_FILE}" ]]; then
        log_error "Output file not created with custom cache TTL"
        return 1
    fi

    log_info "Cache test passed!"
    return 0
}

# Test invalid cache TTL
test_invalid_cache_ttl() {
    log_info "Testing invalid cache TTL handling..."

    CONFD="${SCRIPT_DIR}/../../../bin/confd"

    # This should fail
    if "${CONFD}" imds \
        --confdir "${CONFDIR}" \
        --onetime \
        --imds-cache-ttl "invalid" \
        2>&1 | grep -q "invalid imds-cache-ttl"; then
        log_info "Invalid cache TTL correctly rejected"
        return 0
    else
        log_error "Invalid cache TTL should have been rejected"
        return 1
    fi
}

# Test watch mode rejection
test_watch_mode_rejected() {
    log_info "Testing that watch mode is rejected..."

    CONFD="${SCRIPT_DIR}/../../../bin/confd"

    # This should fail
    if "${CONFD}" imds \
        --confdir "${CONFDIR}" \
        --watch \
        2>&1 | grep -q "watch mode not supported"; then
        log_info "Watch mode correctly rejected"
        return 0
    else
        log_error "Watch mode should have been rejected"
        return 1
    fi
}

# Main test execution
main() {
    log_info "Starting IMDS backend integration tests..."

    # Check if we're on EC2
    if ! check_ec2; then
        log_warn "Skipping IMDS integration tests (not on EC2)"
        log_info "To run these tests, execute on an EC2 instance with IMDSv2 enabled"
        exit 0
    fi

    # Run tests
    test_basic || exit 1
    test_cache || exit 1
    test_invalid_cache_ttl || exit 1
    test_watch_mode_rejected || exit 1

    log_info "All IMDS integration tests passed!"
}

main "$@"
