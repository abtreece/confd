package log

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSetTag(t *testing.T) {
	originalTag := tag
	defer func() { tag = originalTag }()

	SetTag("test-app")
	if tag != "test-app" {
		t.Errorf("SetTag() tag = %q, want %q", tag, "test-app")
	}
}

func TestSetLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"warning level", "warning", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLevel(tt.level)
			handler, ok := logger.Handler().(*ConfdHandler)
			if !ok {
				t.Fatal("Expected ConfdHandler")
			}
			if handler.opts.Level.Level() != tt.expected {
				t.Errorf("SetLevel(%q) level = %v, want %v", tt.level, handler.opts.Level.Level(), tt.expected)
			}
		})
	}
}

func TestSetFormat(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		checkJSON bool
	}{
		{"json format", "json", true},
		{"text format", "text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetFormat(tt.format)
			handler := logger.Handler()
			
			// Check the handler type
			_, isJSON := handler.(*slog.JSONHandler)
			_, isConfd := handler.(*ConfdHandler)
			
			if tt.checkJSON {
				if !isJSON {
					t.Errorf("SetFormat(%q) expected JSONHandler, got %T", tt.format, handler)
				}
			} else {
				if !isConfd {
					t.Errorf("SetFormat(%q) expected ConfdHandler, got %T", tt.format, handler)
				}
			}
		})
	}
}

func TestConfdFormatter(t *testing.T) {
	handler := &ConfdHandler{
		opts: slog.HandlerOptions{
			Level: slog.LevelInfo,
		},
		hostname: "testhost",
		w:        &bytes.Buffer{},
	}
	
	var buf bytes.Buffer
	handler.w = &buf
	
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	err := handler.Handle(context.Background(), r)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	outputStr := buf.String()
	if !strings.Contains(outputStr, "INFO") {
		t.Errorf("Format() output should contain 'INFO', got %q", outputStr)
	}
	if !strings.Contains(outputStr, "test message") {
		t.Errorf("Format() output should contain 'test message', got %q", outputStr)
	}
	if !strings.HasSuffix(outputStr, "\n") {
		t.Errorf("Format() output should end with newline, got %q", outputStr)
	}
}

func TestDebug(t *testing.T) {
	var buf bytes.Buffer
	
	hostname, _ := os.Hostname()
	handler := &ConfdHandler{
		opts: slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
		hostname: hostname,
		w:        &buf,
	}
	logger = slog.New(handler)

	Debug("test %s", "debug")

	output := buf.String()
	if !strings.Contains(output, "test debug") {
		t.Errorf("Debug() output = %q, want to contain 'test debug'", output)
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	
	hostname, _ := os.Hostname()
	handler := &ConfdHandler{
		opts: slog.HandlerOptions{
			Level: slog.LevelError,
		},
		hostname: hostname,
		w:        &buf,
	}
	logger = slog.New(handler)

	Error("test %s", "error")

	output := buf.String()
	if !strings.Contains(output, "test error") {
		t.Errorf("Error() output = %q, want to contain 'test error'", output)
	}
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	
	hostname, _ := os.Hostname()
	handler := &ConfdHandler{
		opts: slog.HandlerOptions{
			Level: slog.LevelInfo,
		},
		hostname: hostname,
		w:        &buf,
	}
	logger = slog.New(handler)

	Info("test %s", "info")

	output := buf.String()
	if !strings.Contains(output, "test info") {
		t.Errorf("Info() output = %q, want to contain 'test info'", output)
	}
}

func TestWarning(t *testing.T) {
	var buf bytes.Buffer
	
	hostname, _ := os.Hostname()
	handler := &ConfdHandler{
		opts: slog.HandlerOptions{
			Level: slog.LevelWarn,
		},
		hostname: hostname,
		w:        &buf,
	}
	logger = slog.New(handler)

	Warning("test %s", "warning")

	output := buf.String()
	if !strings.Contains(output, "test warning") {
		t.Errorf("Warning() output = %q, want to contain 'test warning'", output)
	}
}

// Test structured logging methods
func TestStructuredLogging(t *testing.T) {
	var buf bytes.Buffer
	
	hostname, _ := os.Hostname()
	handler := &ConfdHandler{
		opts: slog.HandlerOptions{
			Level: slog.LevelInfo,
		},
		hostname: hostname,
		w:        &buf,
	}
	logger = slog.New(handler)

	InfoContext(context.Background(), "structured message", "key", "value", "count", 42)

	output := buf.String()
	if !strings.Contains(output, "structured message") {
		t.Errorf("InfoContext() output = %q, want to contain 'structured message'", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("InfoContext() output = %q, want to contain 'key=value'", output)
	}
	if !strings.Contains(output, "count=42") {
		t.Errorf("InfoContext() output = %q, want to contain 'count=42'", output)
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	
	hostname, _ := os.Hostname()
	handler := &ConfdHandler{
		opts: slog.HandlerOptions{
			Level: slog.LevelInfo,
		},
		hostname: hostname,
		w:        &buf,
	}
	logger = slog.New(handler)

	childLogger := With("component", "test")
	childLogger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("With() output = %q, want to contain 'test message'", output)
	}
	if !strings.Contains(output, "component=test") {
		t.Errorf("With() output = %q, want to contain 'component=test'", output)
	}
}
