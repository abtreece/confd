#!/bin/sh
set -e

# Signal forwarding entrypoint for confd in Docker
# This ensures graceful shutdown and reload work correctly in containers

# Forward SIGTERM and SIGINT to confd for graceful shutdown
trap 'kill -TERM $PID' TERM INT
# Forward SIGHUP to confd for configuration reload
trap 'kill -HUP $PID' HUP

# Start confd in the background
/usr/local/bin/confd "$@" &
PID=$!

# Wait for confd to exit
wait $PID
EXIT_CODE=$?

# Exit with the same code as confd
exit $EXIT_CODE
