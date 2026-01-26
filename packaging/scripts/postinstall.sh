#!/bin/bash
set -e

# This script runs after package installation or upgrade.
# Arguments:
#   DEB: $1 = "configure" (fresh install or upgrade)
#   RPM: $1 = 1 (fresh install) or 2+ (upgrade)

# Create state directory
if [ ! -d /var/lib/confd ]; then
    mkdir -p /var/lib/confd
    chmod 755 /var/lib/confd
fi

# Reload systemd to pick up new/updated service file
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload

    # On upgrade, restart the service if it was enabled
    # (preremove stopped it, but didn't disable it on upgrade)
    if systemctl is-enabled --quiet confd 2>/dev/null; then
        echo "Restarting confd service..."
        systemctl restart confd
    fi
fi

# Only show setup instructions on fresh install
# DEB: $1 is "configure" for both install and upgrade, but we can check if service exists
# RPM: $1 = 1 for fresh install, 2+ for upgrade
is_fresh_install() {
    case "$1" in
        1)
            # RPM fresh install
            return 0
            ;;
        configure)
            # DEB - check if service was already enabled (indicates upgrade)
            if systemctl is-enabled --quiet confd 2>/dev/null; then
                return 1  # upgrade
            fi
            return 0  # fresh install
            ;;
        *)
            return 1
            ;;
    esac
}

if is_fresh_install "$1"; then
    echo ""
    echo "confd has been installed."
    echo ""
    echo "To configure confd:"
    echo "  1. Edit /etc/default/confd (or /etc/sysconfig/confd on RHEL)"
    echo "     - Set CONFD_BACKEND to your backend (etcd, consul, vault, etc.)"
    echo "     - Set CONFD_OPTS with connection and runtime options"
    echo ""
    echo "  2. Create template resources in /etc/confd/conf.d/"
    echo ""
    echo "  3. Create templates in /etc/confd/templates/"
    echo ""
    echo "  Documentation: https://github.com/abtreece/confd#documentation"
    echo ""
    echo "  4. Start the service:"
    echo "     systemctl enable confd"
    echo "     systemctl start confd"
    echo ""
fi
