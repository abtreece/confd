package template

import (
	"os"
	"testing"
)

func TestNewFormatValidator(t *testing.T) {
	tests := []struct {
		name         string
		outputFormat string
	}{
		{
			name:         "json format",
			outputFormat: "json",
		},
		{
			name:         "yaml format",
			outputFormat: "yaml",
		},
		{
			name:         "toml format",
			outputFormat: "toml",
		},
		{
			name:         "xml format",
			outputFormat: "xml",
		},
		{
			name:         "empty format",
			outputFormat: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := newFormatValidator(tt.outputFormat)
			if validator == nil {
				t.Fatal("newFormatValidator() returned nil")
			}
			if validator.outputFormat != tt.outputFormat {
				t.Errorf("newFormatValidator() outputFormat = %v, want %v", validator.outputFormat, tt.outputFormat)
			}
		})
	}
}

func TestValidateContent_ValidJSON(t *testing.T) {
	validator := newFormatValidator("json")
	validJSON := []byte(`{"key": "value", "number": 42}`)

	err := validator.validateContent(validJSON)
	if err != nil {
		t.Errorf("validateContent() unexpected error for valid JSON: %v", err)
	}
}

func TestValidateContent_InvalidJSON(t *testing.T) {
	validator := newFormatValidator("json")
	invalidJSON := []byte(`{invalid json}`)

	err := validator.validateContent(invalidJSON)
	if err == nil {
		t.Error("validateContent() expected error for invalid JSON, got nil")
	}
}

func TestValidateContent_ValidYAML(t *testing.T) {
	validator := newFormatValidator("yaml")
	validYAML := []byte(`key: value
number: 42`)

	err := validator.validateContent(validYAML)
	if err != nil {
		t.Errorf("validateContent() unexpected error for valid YAML: %v", err)
	}
}

func TestValidateContent_InvalidYAML(t *testing.T) {
	validator := newFormatValidator("yaml")
	invalidYAML := []byte(`key: value
  invalid: indentation
 bad: format`)

	err := validator.validateContent(invalidYAML)
	if err == nil {
		t.Error("validateContent() expected error for invalid YAML, got nil")
	}
}

func TestValidateContent_ValidTOML(t *testing.T) {
	validator := newFormatValidator("toml")
	validTOML := []byte(`key = "value"
number = 42`)

	err := validator.validateContent(validTOML)
	if err != nil {
		t.Errorf("validateContent() unexpected error for valid TOML: %v", err)
	}
}

func TestValidateContent_InvalidTOML(t *testing.T) {
	validator := newFormatValidator("toml")
	invalidTOML := []byte(`[invalid toml syntax`)

	err := validator.validateContent(invalidTOML)
	if err == nil {
		t.Error("validateContent() expected error for invalid TOML, got nil")
	}
}

func TestValidateContent_ValidXML(t *testing.T) {
	validator := newFormatValidator("xml")
	validXML := []byte(`<?xml version="1.0"?>
<root>
  <key>value</key>
</root>`)

	err := validator.validateContent(validXML)
	if err != nil {
		t.Errorf("validateContent() unexpected error for valid XML: %v", err)
	}
}

func TestValidateContent_InvalidXML(t *testing.T) {
	validator := newFormatValidator("xml")
	invalidXML := []byte(`<root><unclosed>`)

	err := validator.validateContent(invalidXML)
	if err == nil {
		t.Error("validateContent() expected error for invalid XML, got nil")
	}
}

func TestValidateContent_EmptyFormat(t *testing.T) {
	validator := newFormatValidator("")
	content := []byte(`this is not valid anything`)

	err := validator.validateContent(content)
	if err != nil {
		t.Errorf("validateContent() with empty format should skip validation, got error: %v", err)
	}
}

func TestValidate_ValidFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "format-validator-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	validJSON := []byte(`{"key": "value"}`)
	if _, err := tmpFile.Write(validJSON); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	validator := newFormatValidator("json")
	err = validator.validate(tmpFile.Name())
	if err != nil {
		t.Errorf("validate() unexpected error for valid file: %v", err)
	}
}

func TestValidate_InvalidFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "format-validator-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	invalidJSON := []byte(`{invalid}`)
	if _, err := tmpFile.Write(invalidJSON); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	validator := newFormatValidator("json")
	err = validator.validate(tmpFile.Name())
	if err == nil {
		t.Error("validate() expected error for invalid file content, got nil")
	}
}

func TestValidate_NonexistentFile(t *testing.T) {
	validator := newFormatValidator("json")
	err := validator.validate("/nonexistent/file.json")
	if err == nil {
		t.Error("validate() expected error for nonexistent file, got nil")
	}
}

func TestValidate_EmptyFormat(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "format-validator-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := []byte(`anything goes`)
	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	validator := newFormatValidator("")
	err = validator.validate(tmpFile.Name())
	if err != nil {
		t.Errorf("validate() with empty format should skip validation, got error: %v", err)
	}
}

func TestValidateContent_UnsupportedFormat(t *testing.T) {
	validator := newFormatValidator("unsupported")
	content := []byte(`some content`)

	err := validator.validateContent(content)
	if err == nil {
		t.Error("validateContent() expected error for unsupported format, got nil")
	}
}
