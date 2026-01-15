//go:build !linux
// +build !linux

package service

import (
	"context"
	"time"
)

// SystemdNotifier handles systemd notification protocol.
// This is a no-op stub for non-Linux platforms.
type SystemdNotifier struct {
	enabled          bool
	watchdogInterval time.Duration
}

// NewSystemdNotifier creates a new systemd notifier.
func NewSystemdNotifier(enabled bool, watchdogInterval time.Duration) *SystemdNotifier {
	return &SystemdNotifier{
		enabled:          enabled,
		watchdogInterval: watchdogInterval,
	}
}

// NotifyReady sends READY=1 to systemd (no-op on non-Linux).
func (s *SystemdNotifier) NotifyReady() error {
	return nil
}

// NotifyReloading sends RELOADING=1 to systemd (no-op on non-Linux).
func (s *SystemdNotifier) NotifyReloading() error {
	return nil
}

// NotifyStopping sends STOPPING=1 to systemd (no-op on non-Linux).
func (s *SystemdNotifier) NotifyStopping() error {
	return nil
}

// StartWatchdog starts the systemd watchdog heartbeat goroutine (no-op on non-Linux).
func (s *SystemdNotifier) StartWatchdog(ctx context.Context) {
	// No-op on non-Linux platforms
}
