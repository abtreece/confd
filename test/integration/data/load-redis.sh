#!/bin/bash
# Load test data into Redis from centralized JSON file
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DATA_FILE="$SCRIPT_DIR/test-data.json"

if [[ -z "$REDIS_HOST" ]]; then
    echo "ERROR: REDIS_HOST must be set" >&2
    exit 1
fi

# Load JSON and iterate through all key-value pairs
load_json() {
    local prefix="$1"
    local json="$2"

    echo "$json" | jq -r 'to_entries | .[] | "\(.key)\t\(.value)"' | while IFS=$'\t' read -r key value; do
        full_key="$prefix/$key"
        if echo "$value" | jq -e 'type == "object"' > /dev/null 2>&1; then
            load_json "$full_key" "$value"
        else
            clean_value=$(echo "$value" | sed 's/^"//;s/"$//')
            redis-cli -h "$REDIS_HOST" set "$full_key" "$clean_value" > /dev/null
        fi
    done
}

# Clear existing data
redis-cli -h "$REDIS_HOST" FLUSHDB > /dev/null

# Load the test data
load_json "" "$(cat "$DATA_FILE")"

echo "Redis data loaded successfully"
