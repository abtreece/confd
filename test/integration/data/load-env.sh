#!/bin/bash
# Export test data as environment variables
# Source this file: source ./test/integration/data/load-env.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DATA_FILE="$SCRIPT_DIR/test-data.json"

# Load JSON and export as environment variables
# Path /database/host becomes DATABASE_HOST
load_json() {
    local prefix="$1"
    local json="$2"

    echo "$json" | jq -r 'to_entries | .[] | "\(.key)\t\(.value)"' | while IFS=$'\t' read -r key value; do
        full_key="$prefix$key"
        if echo "$value" | jq -e 'type == "object"' > /dev/null 2>&1; then
            load_json "${full_key}_" "$value"
        else
            # Convert path to env var: /database/host -> DATABASE_HOST
            env_var=$(echo "$full_key" | tr '[:lower:]' '[:upper:]' | sed 's/^_//' | tr '/' '_')
            clean_value=$(echo "$value" | sed 's/^"//;s/"$//')
            export "$env_var"="$clean_value"
        fi
    done
}

load_json "" "$(cat "$DATA_FILE")"

echo "Environment variables exported successfully"
