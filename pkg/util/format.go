package util

import (
	"encoding/json"
	"encoding/xml"
	"fmt"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v2"
)

// ValidateFormat validates that the content is valid for the specified format.
// Supported formats: json, yaml, toml, xml
// Returns nil if valid, or an error describing the validation failure.
func ValidateFormat(content []byte, format string) error {
	switch format {
	case "json":
		return validateJSON(content)
	case "yaml", "yml":
		return validateYAML(content)
	case "toml":
		return validateTOML(content)
	case "xml":
		return validateXML(content)
	case "":
		// No format specified, skip validation
		return nil
	default:
		return fmt.Errorf("unknown output format: %q (supported: json, yaml, toml, xml)", format)
	}
}

func validateJSON(content []byte) error {
	var v interface{}
	if err := json.Unmarshal(content, &v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func validateYAML(content []byte) error {
	var v interface{}
	if err := yaml.Unmarshal(content, &v); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return nil
}

func validateTOML(content []byte) error {
	var v interface{}
	if err := toml.Unmarshal(content, &v); err != nil {
		return fmt.Errorf("invalid TOML: %w", err)
	}
	return nil
}

func validateXML(content []byte) error {
	// Use a simple struct to validate XML structure
	type xmlValidator struct {
		XMLName xml.Name
		Content string `xml:",innerxml"`
	}

	var v xmlValidator
	if err := xml.Unmarshal(content, &v); err != nil {
		return fmt.Errorf("invalid XML: %w", err)
	}
	return nil
}
