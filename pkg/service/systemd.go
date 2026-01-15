//go:build linux
// +build linux

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/coreos/go-systemd/v22/daemon"
)

// SystemdNotifier handles systemd notification protocol.
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

// NotifyReady sends READY=1 to systemd.
func (s *SystemdNotifier) NotifyReady() error {
	if !s.enabled {
		return nil
	}

	sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		return fmt.Errorf("failed to notify systemd (ready): %w", err)
	}
	if sent {
		log.Info("Notified systemd: READY=1")
	}
	return nil
}

// NotifyReloading sends RELOADING=1 to systemd.
func (s *SystemdNotifier) NotifyReloading() error {
	if !s.enabled {
		return nil
	}

	sent, err := daemon.SdNotify(false, daemon.SdNotifyReloading)
	if err != nil {
		return fmt.Errorf("failed to notify systemd (reloading): %w", err)
	}
	if sent {
		log.Info("Notified systemd: RELOADING=1")
	}
	return nil
}

// NotifyStopping sends STOPPING=1 to systemd.
func (s *SystemdNotifier) NotifyStopping() error {
	if !s.enabled {
		return nil
	}

	sent, err := daemon.SdNotify(false, daemon.SdNotifyStopping)
	if err != nil {
		return fmt.Errorf("failed to notify systemd (stopping): %w", err)
	}
	if sent {
		log.Info("Notified systemd: STOPPING=1")
	}
	return nil
}

// StartWatchdog starts the systemd watchdog heartbeat goroutine.
// It pings systemd at the specified interval until the context is cancelled.
func (s *SystemdNotifier) StartWatchdog(ctx context.Context) {
	if !s.enabled || s.watchdogInterval <= 0 {
		return
	}

	log.Info("Starting systemd watchdog (interval: %v)", s.watchdogInterval)

	go func() {
		ticker := time.NewTicker(s.watchdogInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Debug("Watchdog stopped (context cancelled)")
				return
			case <-ticker.C:
				sent, err := daemon.SdNotify(false, daemon.SdNotifyWatchdog)
				if err != nil {
					log.Error("Failed to send watchdog ping: %v", err)
				} else if sent {
					log.Debug("Sent watchdog ping to systemd")
				}
			}
		}
	}()
}
