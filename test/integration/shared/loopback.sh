#!/bin/bash
# Sourced by the backend test scripts (from the repo root) so the shared
# templates can resolve the loopback address on hosts where the interface is
# not called "lo" (macOS and the BSDs name it "lo0").
#
# The templates read CONFD_TEST_LOOPBACK_IFACE and default to "lo". A value
# already set by the caller is left alone.

if [[ -z "${CONFD_TEST_LOOPBACK_IFACE:-}" ]]; then
  if [[ ! -e /sys/class/net/lo ]] && ifconfig lo0 >/dev/null 2>&1; then
    export CONFD_TEST_LOOPBACK_IFACE="lo0"
  fi
fi
