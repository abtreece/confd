#!/bin/bash
# Load test data into Consul from centralized JSON file
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DATA_FILE="$SCRIPT_DIR/test-data.json"

if [[ -z "$CONSUL_HOST" || -z "$CONSUL_PORT" ]]; then
    echo "ERROR: CONSUL_HOST and CONSUL_PORT must be set" >&2
    exit 1
fi

CONSUL_URL="http://$CONSUL_HOST:$CONSUL_PORT"

# Load JSON and iterate through all key-value pairs
load_json() {
    local prefix="$1"
    local json="$2"

    # Use jq to iterate through the JSON
    echo "$json" | jq -r 'to_entries | .[] | "\(.key)\t\(.value)"' | while IFS=$'\t' read -r key value; do
        full_key="$prefix/$key"
        if echo "$value" | jq -e 'type == "object"' > /dev/null 2>&1; then
            # Recurse into nested objects
            load_json "$full_key" "$value"
        else
            # Write the value (strip quotes from strings)
            clean_value=$(echo "$value" | sed 's/^"//;s/"$//')
            curl -sX PUT "$CONSUL_URL/v1/kv$full_key" -d "$clean_value" > /dev/null
        fi
    done
}

# Clear existing data
curl -sX DELETE "$CONSUL_URL/v1/kv/?recurse=true" > /dev/null 2>&1 || true

# Load the test data
load_json "" "$(cat "$DATA_FILE")"

echo "Consul data loaded successfully"
