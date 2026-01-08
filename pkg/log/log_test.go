package log

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
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
		expected logrus.Level
	}{
		{"debug level", "debug", logrus.DebugLevel},
		{"info level", "info", logrus.InfoLevel},
		{"warn level", "warn", logrus.WarnLevel},
		{"warning level", "warning", logrus.WarnLevel},
		{"error level", "error", logrus.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLevel(tt.level)
			if logrus.GetLevel() != tt.expected {
				t.Errorf("SetLevel(%q) level = %v, want %v", tt.level, logrus.GetLevel(), tt.expected)
			}
		})
	}
}

func TestSetFormat(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		expectedFormat string
	}{
		{"json format", "json", "*logrus.JSONFormatter"},
		{"text format", "text", "*log.ConfdFormatter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetFormat(tt.format)
			formatter := logrus.StandardLogger().Formatter
			typeName := strings.Replace(
				strings.Replace(fmt.Sprintf("%T", formatter), "github.com/sirupsen/logrus", "logrus", 1),
				"github.com/abtreece/confd/pkg/log", "log", 1,
			)
			if typeName != tt.expectedFormat {
				t.Errorf("SetFormat(%q) formatter type = %v, want %v", tt.format, typeName, tt.expectedFormat)
			}
		})
	}
}

func TestConfdFormatter(t *testing.T) {
	formatter := &ConfdFormatter{}
	entry := &logrus.Entry{
		Level:   logrus.InfoLevel,
		Message: "test message",
	}

	output, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	outputStr := string(output)
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
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.DebugLevel)
	defer logrus.SetOutput(nil)

	Debug("test %s", "debug")

	output := buf.String()
	if !strings.Contains(output, "test debug") {
		t.Errorf("Debug() output = %q, want to contain 'test debug'", output)
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.ErrorLevel)
	defer logrus.SetOutput(nil)

	Error("test %s", "error")

	output := buf.String()
	if !strings.Contains(output, "test error") {
		t.Errorf("Error() output = %q, want to contain 'test error'", output)
	}
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.InfoLevel)
	defer logrus.SetOutput(nil)

	Info("test %s", "info")

	output := buf.String()
	if !strings.Contains(output, "test info") {
		t.Errorf("Info() output = %q, want to contain 'test info'", output)
	}
}

func TestWarning(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.WarnLevel)
	defer logrus.SetOutput(nil)

	Warning("test %s", "warning")

	output := buf.String()
	if !strings.Contains(output, "test warning") {
		t.Errorf("Warning() output = %q, want to contain 'test warning'", output)
	}
}
