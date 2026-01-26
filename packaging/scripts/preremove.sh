#!/bin/bash
set -e

# Stop and disable the service before removal
if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet confd 2>/dev/null; then
        echo "Stopping confd service..."
        systemctl stop confd
    fi
    if systemctl is-enabled --quiet confd 2>/dev/null; then
        echo "Disabling confd service..."
        systemctl disable confd
    fi
fi
