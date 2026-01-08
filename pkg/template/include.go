package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"text/template"
)

const maxIncludeDepth = 10

// IncludeContext tracks the include stack for cycle detection.
type IncludeContext struct {
	mu       sync.Mutex
	stack    []string
	maxDepth int
}

// NewIncludeContext creates a new include context for tracking nested includes.
func NewIncludeContext() *IncludeContext {
	return &IncludeContext{
		stack:    make([]string, 0),
		maxDepth: maxIncludeDepth,
	}
}

// Push adds a template to the include stack.
// Returns an error if the template is already in the stack (cycle) or max depth exceeded.
func (c *IncludeContext) Push(templatePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check for cycle
	for _, path := range c.stack {
		if path == templatePath {
			return fmt.Errorf("include cycle detected: %s is already being processed", templatePath)
		}
	}

	// Check depth
	if len(c.stack) >= c.maxDepth {
		return fmt.Errorf("maximum include depth (%d) exceeded", c.maxDepth)
	}

	c.stack = append(c.stack, templatePath)
	return nil
}

// Pop removes the most recent template from the include stack.
func (c *IncludeContext) Pop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.stack) > 0 {
		c.stack = c.stack[:len(c.stack)-1]
	}
}

// Depth returns the current include depth.
func (c *IncludeContext) Depth() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.stack)
}

// NewIncludeFunc creates an include function for templates.
// baseDir is the directory where included templates are resolved from.
// funcMap is the function map to use when parsing included templates.
func NewIncludeFunc(baseDir string, funcMap template.FuncMap, ctx *IncludeContext) func(string, ...interface{}) (string, error) {
	return func(name string, data ...interface{}) (string, error) {
		// Resolve path relative to baseDir
		includePath := filepath.Join(baseDir, name)

		// Clean the path to prevent directory traversal
		includePath = filepath.Clean(includePath)

		// Ensure the resolved path is within baseDir
		if !isPathWithin(includePath, baseDir) {
			return "", fmt.Errorf("include path %q is outside template directory", name)
		}

		// Check for cycles and depth
		if err := ctx.Push(includePath); err != nil {
			return "", err
		}
		defer ctx.Pop()

		// Create a new function map that includes the include function itself
		// to support nested includes
		includeFuncMap := make(template.FuncMap)
		for k, v := range funcMap {
			includeFuncMap[k] = v
		}
		// Update the include function to use the same context
		includeFuncMap["include"] = NewIncludeFunc(baseDir, funcMap, ctx)

		// Try cache first
		var tmpl *template.Template
		tmpl, cacheHit := GetCachedTemplate(includePath)
		if !cacheHit {
			// Read the included template
			content, err := os.ReadFile(includePath)
			if err != nil {
				return "", fmt.Errorf("include %s: %w", name, err)
			}

			stat, err := os.Stat(includePath)
			if err != nil {
				return "", fmt.Errorf("include %s: %w", name, err)
			}

			// Parse the included template
			tmpl, err = template.New(filepath.Base(includePath)).Funcs(includeFuncMap).Parse(string(content))
			if err != nil {
				return "", fmt.Errorf("parse include %s: %w", name, err)
			}

			PutCachedTemplate(includePath, tmpl, stat.ModTime())
		} else {
			// Update funcMap on cached template
			tmpl = tmpl.Funcs(includeFuncMap)
		}

		// Execute with provided data or nil
		var buf bytes.Buffer
		var execData interface{}
		if len(data) > 0 {
			execData = data[0]
		}
		if err := tmpl.Execute(&buf, execData); err != nil {
			return "", fmt.Errorf("execute include %s: %w", name, err)
		}

		return buf.String(), nil
	}
}

// isPathWithin checks if path is within the given directory.
func isPathWithin(path, dir string) bool {
	// Clean both paths
	path = filepath.Clean(path)
	dir = filepath.Clean(dir)

	// Get relative path
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}

	// Check if the relative path starts with ".." which means it's outside
	return rel != ".." && !filepath.IsAbs(rel) && len(rel) >= 1 && rel[0] != '.'
}
