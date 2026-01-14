package template

import (
	"fmt"
	"os"

	"github.com/abtreece/confd/pkg/log"
	util "github.com/abtreece/confd/pkg/util"
)

// formatValidator handles validation of rendered template output against specified formats.
// It wraps the util.ValidateFormat function with additional error handling and logging.
type formatValidator struct {
	outputFormat string
}

// newFormatValidator creates a new formatValidator instance.
// If outputFormat is empty, validation is skipped.
func newFormatValidator(outputFormat string) *formatValidator {
	return &formatValidator{
		outputFormat: outputFormat,
	}
}

// validate validates the content of a file against the configured output format.
// Supported formats: json, yaml, toml, xml.
// Returns nil if outputFormat is empty (validation disabled) or if validation succeeds.
// Returns an error if the file cannot be read or format validation fails.
func (v *formatValidator) validate(filePath string) error {
	if v.outputFormat == "" {
		return nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file for validation: %w", err)
	}

	return v.validateContent(content)
}

// validateContent validates byte content against the configured output format.
// Returns nil if outputFormat is empty or if validation succeeds.
// Returns an error if format validation fails.
func (v *formatValidator) validateContent(content []byte) error {
	if v.outputFormat == "" {
		return nil
	}

	if err := util.ValidateFormat(content, v.outputFormat); err != nil {
		return fmt.Errorf("output format validation failed (%s): %w", v.outputFormat, err)
	}

	log.Debug("Output format validation passed (%s)", v.outputFormat)
	return nil
}
