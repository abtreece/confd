package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/util"
)

// ValidationError represents a validation error with context.
type ValidationError struct {
	File    string
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s: %s", e.File, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Message)
}

// ValidateConfig validates template resource configuration files.
// If resourceFile is empty, all *.toml files in confdir/conf.d are validated.
// If resourceFile is specified, only that file is validated.
func ValidateConfig(confDir string, resourceFile string) error {
	configDir := filepath.Join(confDir, "conf.d")
	templateDir := filepath.Join(confDir, "templates")

	var files []string
	var err error

	if resourceFile != "" {
		// Validate specific resource file
		if !filepath.IsAbs(resourceFile) {
			resourceFile = filepath.Join(configDir, resourceFile)
		}
		if !util.IsFileExist(resourceFile) {
			return fmt.Errorf("resource file not found: %s", resourceFile)
		}
		files = []string{resourceFile}
	} else {
		// Validate all resource files
		if !util.IsFileExist(configDir) {
			return fmt.Errorf("config directory not found: %s", configDir)
		}
		files, err = util.RecursiveFilesLookup(configDir, "*toml")
		if err != nil {
			return fmt.Errorf("failed to list config files: %w", err)
		}
		if len(files) == 0 {
			log.Warning("No configuration files found in %s", configDir)
			return nil
		}
	}

	var validationErrors []error
	validCount := 0

	for _, f := range files {
		errs := validateResourceFile(f, templateDir)
		if len(errs) > 0 {
			for _, e := range errs {
				log.Error("%s", e.Error())
				validationErrors = append(validationErrors, e)
			}
		} else {
			log.Info("OK: %s", f)
			validCount++
		}
	}

	log.Info("Validation complete: %d/%d files passed", validCount, len(files))

	if len(validationErrors) > 0 {
		return errors.Join(validationErrors...)
	}
	return nil
}

// validateResourceFile validates a single template resource TOML file.
func validateResourceFile(path string, templateDir string) []ValidationError {
	var errs []ValidationError

	// Parse TOML
	var tc TemplateResourceConfig
	_, err := toml.DecodeFile(path, &tc)
	if err != nil {
		errs = append(errs, ValidationError{
			File:    path,
			Message: fmt.Sprintf("TOML parse error: %v", err),
		})
		return errs // Can't continue if TOML parsing failed
	}

	tr := tc.TemplateResource

	// Required fields
	if tr.Src == "" {
		errs = append(errs, ValidationError{
			File:    path,
			Field:   "src",
			Message: "required field is missing",
		})
	}

	if tr.Dest == "" {
		errs = append(errs, ValidationError{
			File:    path,
			Field:   "dest",
			Message: "required field is missing",
		})
	}

	if len(tr.Keys) == 0 {
		errs = append(errs, ValidationError{
			File:    path,
			Field:   "keys",
			Message: "required field is missing or empty",
		})
	}

	// Mode validation (should be octal)
	if tr.Mode != "" {
		_, err := strconv.ParseUint(tr.Mode, 8, 32)
		if err != nil {
			// Also try parsing with 0 prefix (auto-detect base)
			_, err = strconv.ParseUint(tr.Mode, 0, 32)
			if err != nil {
				errs = append(errs, ValidationError{
					File:    path,
					Field:   "mode",
					Message: fmt.Sprintf("must be a valid octal value (e.g., \"0644\"), got %q", tr.Mode),
				})
			}
		}
	}

	// Template file exists
	if tr.Src != "" {
		templatePath := filepath.Join(templateDir, tr.Src)
		if !util.IsFileExist(templatePath) {
			errs = append(errs, ValidationError{
				File:    path,
				Field:   "src",
				Message: fmt.Sprintf("template file not found: %s", templatePath),
			})
		}
	}

	// Destination directory exists (or is creatable)
	if tr.Dest != "" {
		destDir := filepath.Dir(tr.Dest)
		if !util.IsFileExist(destDir) {
			errs = append(errs, ValidationError{
				File:    path,
				Field:   "dest",
				Message: fmt.Sprintf("destination directory does not exist: %s", destDir),
			})
		}
	}

	// Keys should not be empty strings
	for i, key := range tr.Keys {
		if strings.TrimSpace(key) == "" {
			errs = append(errs, ValidationError{
				File:    path,
				Field:   fmt.Sprintf("keys[%d]", i),
				Message: "key cannot be empty",
			})
		}
	}

	// Validate output_format if specified
	if tr.OutputFormat != "" {
		validFormats := map[string]bool{
			"json": true,
			"yaml": true,
			"yml":  true,
			"toml": true,
			"xml":  true,
		}
		if !validFormats[tr.OutputFormat] {
			errs = append(errs, ValidationError{
				File:    path,
				Field:   "output_format",
				Message: fmt.Sprintf("unknown format: %q (supported: json, yaml, yml, toml, xml)", tr.OutputFormat),
			})
		}
	}

	// Validate min_reload_interval if specified
	if tr.MinReloadInterval != "" {
		_, err := time.ParseDuration(tr.MinReloadInterval)
		if err != nil {
			errs = append(errs, ValidationError{
				File:    path,
				Field:   "min_reload_interval",
				Message: fmt.Sprintf("invalid duration: %q (use Go duration format, e.g., \"30s\", \"1m\")", tr.MinReloadInterval),
			})
		}
	}

	// Validate debounce if specified
	if tr.Debounce != "" {
		_, err := time.ParseDuration(tr.Debounce)
		if err != nil {
			errs = append(errs, ValidationError{
				File:    path,
				Field:   "debounce",
				Message: fmt.Sprintf("invalid duration: %q (use Go duration format, e.g., \"2s\", \"500ms\")", tr.Debounce),
			})
		}
	}

	// Validate per-resource backend config if present
	if tc.BackendConfig != nil {
		if tc.BackendConfig.Backend == "" {
			errs = append(errs, ValidationError{
				File:    path,
				Field:   "backend.backend",
				Message: "backend type is required when [backend] section is present",
			})
		} else {
			// Validate backend type is known
			validBackends := map[string]bool{
				"consul":         true,
				"etcd":           true,
				"vault":          true,
				"redis":          true,
				"zookeeper":      true,
				"dynamodb":       true,
				"ssm":            true,
				"acm":            true,
				"secretsmanager": true,
				"env":            true,
				"file":           true,
			}
			if !validBackends[tc.BackendConfig.Backend] {
				errs = append(errs, ValidationError{
					File:    path,
					Field:   "backend.backend",
					Message: fmt.Sprintf("unknown backend type: %q", tc.BackendConfig.Backend),
				})
			}
		}
	}

	return errs
}

// ValidateTemplates validates template files by parsing them.
// If mockDataFile is provided, templates are also executed with mock data.
// If resourceFile is empty, all templates are validated.
func ValidateTemplates(confDir string, resourceFile string, mockDataFile string) error {
	configDir := filepath.Join(confDir, "conf.d")
	templateDir := filepath.Join(confDir, "templates")

	var files []string
	var err error

	if resourceFile != "" {
		// Validate specific resource file
		if !filepath.IsAbs(resourceFile) {
			resourceFile = filepath.Join(configDir, resourceFile)
		}
		if !util.IsFileExist(resourceFile) {
			return fmt.Errorf("resource file not found: %s", resourceFile)
		}
		files = []string{resourceFile}
	} else {
		// Validate all resource files
		if !util.IsFileExist(configDir) {
			return fmt.Errorf("config directory not found: %s", configDir)
		}
		files, err = util.RecursiveFilesLookup(configDir, "*toml")
		if err != nil {
			return fmt.Errorf("failed to list config files: %w", err)
		}
		if len(files) == 0 {
			log.Warning("No configuration files found in %s", configDir)
			return nil
		}
	}

	// Load mock data if provided
	var mockData map[string]interface{}
	if mockDataFile != "" {
		data, err := loadMockData(mockDataFile)
		if err != nil {
			return fmt.Errorf("failed to load mock data: %w", err)
		}
		mockData = data
	}

	var validationErrors []error
	validCount := 0

	for _, f := range files {
		errs := validateTemplate(f, templateDir, mockData)
		if len(errs) > 0 {
			for _, e := range errs {
				log.Error("%s", e.Error())
				validationErrors = append(validationErrors, e)
			}
		} else {
			log.Info("OK: %s", f)
			validCount++
		}
	}

	log.Info("Template validation complete: %d/%d passed", validCount, len(files))

	if len(validationErrors) > 0 {
		return errors.Join(validationErrors...)
	}
	return nil
}

// newValidationFuncMap creates a function map for template validation.
// It includes stub implementations of store functions that return empty values.
func newValidationFuncMap() template.FuncMap {
	// Start with base functions
	funcMap := newFuncMap()

	// Add stub implementations of store functions
	// These allow syntax validation without a live backend
	funcMap["getv"] = func(key string, defaultValue ...string) string {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}
	funcMap["gets"] = func(pattern string) []string {
		return []string{}
	}
	funcMap["getvs"] = func(pattern string) []string {
		return []string{}
	}
	funcMap["getenv"] = func(key string, defaultValue ...string) string {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}
	funcMap["ls"] = func(dir string) []string {
		return []string{}
	}
	funcMap["lsdir"] = func(dir string) []string {
		return []string{}
	}
	funcMap["exists"] = func(key string) bool {
		return false
	}
	funcMap["get"] = func(key string) interface{} {
		return nil
	}
	funcMap["getall"] = func(pattern string) map[string]string {
		return map[string]string{}
	}
	funcMap["include"] = func(name string, data ...interface{}) (string, error) {
		return "", nil // Stub for validation
	}

	return funcMap
}

// loadMockData loads mock data from a JSON file.
func loadMockData(path string) (map[string]interface{}, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON in mock data file: %w", err)
	}
	return data, nil
}

// validateTemplate validates a single template by parsing it.
// If mockData is provided, the template is also executed.
func validateTemplate(resourcePath string, templateDir string, mockData map[string]interface{}) []ValidationError {
	var errs []ValidationError

	// Parse resource file
	var tc TemplateResourceConfig
	_, err := toml.DecodeFile(resourcePath, &tc)
	if err != nil {
		errs = append(errs, ValidationError{
			File:    resourcePath,
			Message: fmt.Sprintf("TOML parse error: %v", err),
		})
		return errs
	}

	if tc.TemplateResource.Src == "" {
		errs = append(errs, ValidationError{
			File:    resourcePath,
			Field:   "src",
			Message: "required field is missing",
		})
		return errs
	}

	templatePath := filepath.Join(templateDir, tc.TemplateResource.Src)
	if !util.IsFileExist(templatePath) {
		errs = append(errs, ValidationError{
			File:    resourcePath,
			Field:   "src",
			Message: fmt.Sprintf("template file not found: %s", templatePath),
		})
		return errs
	}

	// Create function map with template functions and stub store functions
	funcMap := newValidationFuncMap()

	// Parse template
	tmpl, err := template.New(filepath.Base(templatePath)).Funcs(funcMap).ParseFiles(templatePath)
	if err != nil {
		errs = append(errs, ValidationError{
			File:    templatePath,
			Message: fmt.Sprintf("template parse error: %v", err),
		})
		return errs
	}

	// Execute template with mock data if provided
	if mockData != nil {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, mockData); err != nil {
			errs = append(errs, ValidationError{
				File:    templatePath,
				Message: fmt.Sprintf("template execution error: %v", err),
			})
			return errs
		}

		// Validate output format if specified
		if tc.TemplateResource.OutputFormat != "" {
			if err := util.ValidateFormat(buf.Bytes(), tc.TemplateResource.OutputFormat); err != nil {
				errs = append(errs, ValidationError{
					File:    templatePath,
					Field:   "output_format",
					Message: fmt.Sprintf("output validation failed: %v", err),
				})
			}
		}
	}

	return errs
}
