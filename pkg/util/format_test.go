package util

import (
	"testing"
)

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		format      string
		expectError bool
	}{
		// JSON tests
		{
			name:        "valid JSON object",
			content:     []byte(`{"key": "value", "number": 123}`),
			format:      "json",
			expectError: false,
		},
		{
			name:        "valid JSON array",
			content:     []byte(`[1, 2, 3, "four"]`),
			format:      "json",
			expectError: false,
		},
		{
			name:        "invalid JSON",
			content:     []byte(`{"key": "value",}`),
			format:      "json",
			expectError: true,
		},
		{
			name:        "empty JSON",
			content:     []byte(`{}`),
			format:      "json",
			expectError: false,
		},

		// YAML tests
		{
			name: "valid YAML",
			content: []byte(`
key: value
nested:
  foo: bar
list:
  - item1
  - item2
`),
			format:      "yaml",
			expectError: false,
		},
		{
			name:        "valid YAML with yml format",
			content:     []byte(`key: value`),
			format:      "yml",
			expectError: false,
		},
		{
			name:        "invalid YAML",
			content:     []byte(`key: value\n  bad indent: here`),
			format:      "yaml",
			expectError: true, // Literal \n causes mapping error
		},

		// TOML tests
		{
			name: "valid TOML",
			content: []byte(`
[section]
key = "value"
number = 42

[section.nested]
foo = "bar"
`),
			format:      "toml",
			expectError: false,
		},
		{
			name:        "invalid TOML",
			content:     []byte(`[section]\nkey = `),
			format:      "toml",
			expectError: true,
		},

		// XML tests
		{
			name:        "valid XML",
			content:     []byte(`<?xml version="1.0"?><root><child>value</child></root>`),
			format:      "xml",
			expectError: false,
		},
		{
			name:        "valid simple XML",
			content:     []byte(`<root><item>test</item></root>`),
			format:      "xml",
			expectError: false,
		},
		{
			name:        "invalid XML - unclosed tag",
			content:     []byte(`<root><child>value</root>`),
			format:      "xml",
			expectError: true,
		},

		// Empty format (should skip validation)
		{
			name:        "empty format skips validation",
			content:     []byte(`this is not valid json or yaml`),
			format:      "",
			expectError: false,
		},

		// Unknown format
		{
			name:        "unknown format",
			content:     []byte(`some content`),
			format:      "unknown",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFormat(tt.content, tt.format)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestValidateJSON(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		expectError bool
	}{
		{"valid object", []byte(`{"a": 1}`), false},
		{"valid array", []byte(`[1, 2]`), false},
		{"valid string", []byte(`"hello"`), false},
		{"valid number", []byte(`42`), false},
		{"valid null", []byte(`null`), false},
		{"valid boolean", []byte(`true`), false},
		{"invalid - trailing comma", []byte(`{"a": 1,}`), true},
		{"invalid - unquoted key", []byte(`{a: 1}`), true},
		{"invalid - single quotes", []byte(`{'a': 1}`), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSON(tt.content)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestValidateYAML(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		expectError bool
	}{
		{"valid map", []byte("key: value"), false},
		{"valid list", []byte("- item1\n- item2"), false},
		{"valid nested", []byte("parent:\n  child: value"), false},
		{"empty", []byte(""), false},
		{"just comment", []byte("# comment"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateYAML(tt.content)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestValidateTOML(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		expectError bool
	}{
		{"valid key-value", []byte("key = \"value\""), false},
		{"valid section", []byte("[section]\nkey = \"value\""), false},
		{"valid number", []byte("num = 42"), false},
		{"valid boolean", []byte("enabled = true"), false},
		{"invalid - missing value", []byte("key = "), true},
		{"invalid - bad section", []byte("[section"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTOML(tt.content)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestValidateXML(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		expectError bool
	}{
		{"valid simple", []byte("<root/>"), false},
		{"valid with content", []byte("<root>content</root>"), false},
		{"valid nested", []byte("<root><child>value</child></root>"), false},
		{"valid with declaration", []byte(`<?xml version="1.0"?><root/>`), false},
		{"invalid - unclosed", []byte("<root>"), true},
		{"invalid - mismatch", []byte("<root></other>"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateXML(tt.content)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}
