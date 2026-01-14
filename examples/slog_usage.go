package main

import (
	"context"
	"time"

	"github.com/abtreece/confd/pkg/log"
)

// Example showing different ways to use the new slog-based logging

func main() {
	// Traditional printf-style logging (backward compatible)
	log.Info("Starting application")
	log.Debug("Configuration loaded from %s", "/etc/confd/confd.toml")
	log.Warning("Using default backend: %s", "etcd")
	log.Error("Failed to connect to backend: %s", "connection refused")

	// Structured logging with context
	ctx := context.Background()
	log.InfoContext(ctx, "Processing template",
		"template", "nginx.conf",
		"backend", "etcd",
		"keys_found", 42,
		"duration_ms", 123)

	// Creating a logger with persistent attributes
	templateLogger := log.With(
		"component", "template-processor",
		"version", "1.0.0",
	)
	templateLogger.Info("Template rendered successfully")

	// Logging errors with structured data
	log.ErrorContext(ctx, "Backend connection failed",
		"backend", "etcd",
		"endpoint", "http://localhost:2379",
		"attempt", 3,
		"retry_after", time.Second*5)

	// Using groups for hierarchical attributes
	backendLogger := log.WithGroup("backend")
	backendLogger.Info("Connection established",
		"type", "etcd",
		"node_count", 3,
		"latency_ms", 12)

	// Setting log format
	log.SetFormat("json") // Switch to JSON format
	log.Info("Now logging in JSON format")

	log.SetFormat("text") // Back to text format
	log.Info("Back to text format")

	// Setting log level
	log.SetLevel("debug")
	log.Debug("Debug messages now visible")
}
