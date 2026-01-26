#!/bin/bash
set -e

# This script runs before package removal or upgrade.
# Arguments:
#   DEB: $1 = "remove" (uninstall) or "upgrade"
#   RPM: $1 = 0 (uninstall) or 1 (upgrade)

# Determine if this is a removal or upgrade
is_removal() {
    case "$1" in
        0|remove|purge)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

if command -v systemctl >/dev/null 2>&1; then
    # Always stop the service (will be restarted by postinstall on upgrade)
    if systemctl is-active --quiet confd 2>/dev/null; then
        echo "Stopping confd service..."
        systemctl stop confd
    fi

    # Only disable on true removal, not on upgrade
    if is_removal "$1"; then
        if systemctl is-enabled --quiet confd 2>/dev/null; then
            echo "Disabling confd service..."
            systemctl disable confd
        fi
    fi
fi
