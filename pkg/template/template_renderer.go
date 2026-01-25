package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/memkv"
	util "github.com/abtreece/confd/pkg/util"
)

// templateRenderer handles template compilation, caching, and rendering.
// It manages the template function map and include function for nested templates.
type templateRenderer struct {
	templateDir string
	funcMap     map[string]interface{}
	store       *memkv.Store
}

// templateRendererConfig holds configuration for creating a templateRenderer.
type templateRendererConfig struct {
	TemplateDir string
	FuncMap     map[string]interface{}
	Store       *memkv.Store
}

// newTemplateRenderer creates a new templateRenderer instance.
func newTemplateRenderer(config templateRendererConfig) *templateRenderer {
	return &templateRenderer{
		templateDir: config.TemplateDir,
		funcMap:     config.FuncMap,
		store:       config.Store,
	}
}

// render compiles and executes a template, returning the rendered content as bytes.
// It handles template caching, include function setup, and error handling.
// The srcPath should be the full path to the template file.
// Returns the rendered content or an error if compilation or execution fails.
func (r *templateRenderer) render(srcPath string) ([]byte, error) {
	log.Debug("Using source template %s", srcPath)

	if !util.IsFileExist(srcPath) {
		return nil, fmt.Errorf("missing template: %s", srcPath)
	}

	log.Debug("Compiling source template %s", srcPath)

	// Add include function to funcMap for this template
	includeCtx := NewIncludeContext()
	r.funcMap["include"] = NewIncludeFunc(r.templateDir, r.funcMap, includeCtx)

	// Try to get template from cache
	var tmpl *template.Template
	var err error
	tmpl, cacheHit := GetCachedTemplate(srcPath)
	if !cacheHit {
		log.Debug("Template cache miss for %s", srcPath)
		stat, statErr := os.Stat(srcPath)
		if statErr != nil {
			return nil, fmt.Errorf("Unable to stat template %s: %w", srcPath, statErr)
		}
		tmpl, err = template.New(filepath.Base(srcPath)).Funcs(r.funcMap).ParseFiles(srcPath)
		if err != nil {
			return nil, fmt.Errorf("unable to process template %s: %w", srcPath, err)
		}
		PutCachedTemplate(srcPath, tmpl, stat.ModTime())
	} else {
		log.Debug("Template cache hit for %s", srcPath)
		// Clone the cached template to avoid concurrent Funcs() race
		// Must clone because Funcs() mutates the template object
		cloned, cloneErr := tmpl.Clone()
		if cloneErr != nil {
			return nil, fmt.Errorf("failed to clone cached template %s: %w", srcPath, cloneErr)
		}
		tmpl = cloned.Funcs(r.funcMap)
	}

	// Execute template to buffer
	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, nil); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
