/*
Package log provides support for logging to stdout and stderr.

Log entries will be logged in the following format:

    timestamp hostname tag[pid]: SEVERITY Message
*/
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// tag represents the application name generating the log message. The tag
// string will appear in all log entries.
var tag string

// logger is the global slog.Logger instance
var logger *slog.Logger

// ConfdHandler is a custom slog.Handler that formats logs in confd's traditional format
type ConfdHandler struct {
	opts     slog.HandlerOptions
	hostname string
	w        io.Writer
	attrs    []slog.Attr
	groups   []string
}

func (h *ConfdHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *ConfdHandler) Handle(_ context.Context, r slog.Record) error {
	timestamp := r.Time.Format(time.RFC3339)
	level := strings.ToUpper(r.Level.String())
	
	// Build the base message
	buf := fmt.Sprintf("%s %s %s[%d]: %s %s",
		timestamp, h.hostname, tag, os.Getpid(), level, r.Message)
	
	// Add attributes if present
	if r.NumAttrs() > 0 || len(h.attrs) > 0 {
		var attrs []string
		
		// Add handler-level attributes
		for _, a := range h.attrs {
			attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
		}
		
		// Add record attributes
		r.Attrs(func(a slog.Attr) bool {
			attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
			return true
		})
		
		if len(attrs) > 0 {
			buf += " " + strings.Join(attrs, " ")
		}
	}
	
	buf += "\n"
	_, err := h.w.Write([]byte(buf))
	return err
}

func (h *ConfdHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := *h
	newHandler.attrs = append([]slog.Attr{}, h.attrs...)
	newHandler.attrs = append(newHandler.attrs, attrs...)
	return &newHandler
}

func (h *ConfdHandler) WithGroup(name string) slog.Handler {
	newHandler := *h
	newHandler.groups = append([]string{}, h.groups...)
	newHandler.groups = append(newHandler.groups, name)
	return &newHandler
}

func init() {
	tag = os.Args[0]
	hostname, _ := os.Hostname()
	
	handler := &ConfdHandler{
		opts: slog.HandlerOptions{
			Level: slog.LevelInfo,
		},
		hostname: hostname,
		w:        os.Stdout,
	}
	
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// SetTag sets the tag.
func SetTag(t string) {
	tag = t
}

// SetLevel sets the log level. Valid levels are panic, fatal, error, warn, info and debug.
func SetLevel(level string) {
	var slogLevel slog.Level
	
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	case "fatal", "panic":
		// slog doesn't have fatal/panic levels, use error
		slogLevel = slog.LevelError
	default:
		Fatal("not a valid level: %q", level)
		return
	}
	
	hostname, _ := os.Hostname()
	handler := &ConfdHandler{
		opts: slog.HandlerOptions{
			Level: slogLevel,
		},
		hostname: hostname,
		w:        os.Stdout,
	}
	
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// SetFormat sets the log format. Valid formats are "text" and "json".
func SetFormat(format string) {
	var handler slog.Handler
	var currentLevel slog.Level = slog.LevelInfo
	
	// Try to get current level from handler
	if currentHandler, ok := logger.Handler().(*ConfdHandler); ok {
		if currentHandler.opts.Level != nil {
			currentLevel = currentHandler.opts.Level.Level()
		}
	}
	
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: currentLevel,
		})
	case "text":
		hostname, _ := os.Hostname()
		handler = &ConfdHandler{
			opts: slog.HandlerOptions{
				Level: currentLevel,
			},
			hostname: hostname,
			w:        os.Stdout,
		}
	default:
		Fatal("not a valid log format: %q", format)
		return
	}
	
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// Debug logs a message with severity DEBUG.
func Debug(format string, v ...interface{}) {
	logger.Debug(fmt.Sprintf(format, v...))
}

// Error logs a message with severity ERROR.
func Error(format string, v ...interface{}) {
	logger.Error(fmt.Sprintf(format, v...))
}

// Fatal logs a message with severity ERROR followed by a call to os.Exit().
func Fatal(format string, v ...interface{}) {
	logger.Error(fmt.Sprintf(format, v...))
	os.Exit(1)
}

// Info logs a message with severity INFO.
func Info(format string, v ...interface{}) {
	logger.Info(fmt.Sprintf(format, v...))
}

// Warning logs a message with severity WARNING.
func Warning(format string, v ...interface{}) {
	logger.Warn(fmt.Sprintf(format, v...))
}

// Structured logging methods using slog's native API

// DebugContext logs a message with severity DEBUG and context.
func DebugContext(ctx context.Context, msg string, args ...any) {
	logger.DebugContext(ctx, msg, args...)
}

// InfoContext logs a message with severity INFO and context.
func InfoContext(ctx context.Context, msg string, args ...any) {
	logger.InfoContext(ctx, msg, args...)
}

// WarnContext logs a message with severity WARN and context.
func WarnContext(ctx context.Context, msg string, args ...any) {
	logger.WarnContext(ctx, msg, args...)
}

// ErrorContext logs a message with severity ERROR and context.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	logger.ErrorContext(ctx, msg, args...)
}

// With returns a new logger with the given attributes.
func With(args ...any) *slog.Logger {
	return logger.With(args...)
}

// WithGroup returns a new logger with the given group.
func WithGroup(name string) *slog.Logger {
	return logger.WithGroup(name)
}

// Logger returns the underlying slog.Logger for advanced usage.
func Logger() *slog.Logger {
	return logger
}
