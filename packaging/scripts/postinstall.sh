#!/bin/bash
set -e

# Create state directory
if [ ! -d /var/lib/confd ]; then
    mkdir -p /var/lib/confd
    chmod 755 /var/lib/confd
fi

# Reload systemd to pick up new service file
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
fi

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
