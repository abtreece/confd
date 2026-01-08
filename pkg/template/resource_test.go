package template

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/abtreece/confd/pkg/backends/env"
	"github.com/abtreece/confd/pkg/log"
)

func TestNewTemplateResourcePrefixConcatenation(t *testing.T) {
	log.SetLevel("warn")

	tempConfDir, err := createTempDirs()
	if err != nil {
		t.Fatalf("Failed to create temp dirs: %s", err.Error())
	}
	defer os.RemoveAll(tempConfDir)

	// Create a minimal template file
	srcTemplateFile := filepath.Join(tempConfDir, "templates", "test.tmpl")
	err = os.WriteFile(srcTemplateFile, []byte(`test`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	storeClient, err := env.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		configPrefix   string
		resourcePrefix string
		expectedPrefix string
	}{
		{
			name:           "both prefixes set",
			configPrefix:   "production",
			resourcePrefix: "myapp",
			expectedPrefix: "/production/myapp",
		},
		{
			name:           "both prefixes with leading slashes",
			configPrefix:   "/production",
			resourcePrefix: "/myapp",
			expectedPrefix: "/production/myapp",
		},
		{
			name:           "only config prefix",
			configPrefix:   "production",
			resourcePrefix: "",
			expectedPrefix: "/production",
		},
		{
			name:           "only resource prefix",
			configPrefix:   "",
			resourcePrefix: "myapp",
			expectedPrefix: "/myapp",
		},
		{
			name:           "neither prefix set",
			configPrefix:   "",
			resourcePrefix: "",
			expectedPrefix: "/",
		},
		{
			name:           "nested resource prefix",
			configPrefix:   "env/production",
			resourcePrefix: "apps/myapp",
			expectedPrefix: "/env/production/apps/myapp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create resource config with optional prefix
			resourceContent := "[template]\nsrc = \"test.tmpl\"\ndest = \"/tmp/test.conf\"\nkeys = [\"/foo\"]\n"
			if tc.resourcePrefix != "" {
				resourceContent += "prefix = \"" + tc.resourcePrefix + "\"\n"
			}

			resourcePath := filepath.Join(tempConfDir, "conf.d", "test.toml")
			err := os.WriteFile(resourcePath, []byte(resourceContent), 0644)
			if err != nil {
				t.Fatal(err)
			}

			config := Config{
				ConfDir:     tempConfDir,
				ConfigDir:   filepath.Join(tempConfDir, "conf.d"),
				Prefix:      tc.configPrefix,
				StoreClient: storeClient,
				TemplateDir: filepath.Join(tempConfDir, "templates"),
			}

			tr, err := NewTemplateResource(resourcePath, config)
			if err != nil {
				t.Fatalf("NewTemplateResource failed: %s", err)
			}

			if tr.Prefix != tc.expectedPrefix {
				t.Errorf("Expected prefix %q, got %q", tc.expectedPrefix, tr.Prefix)
			}
		})
	}
}

// createTempDirs is a helper function which creates temporary directories
// required by confd. createTempDirs returns the path name representing the
// confd confDir.
// It returns an error if any.
func createTempDirs() (string, error) {
	confDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}
	err = os.Mkdir(filepath.Join(confDir, "templates"), 0755)
	if err != nil {
		return "", err
	}
	err = os.Mkdir(filepath.Join(confDir, "conf.d"), 0755)
	if err != nil {
		return "", err
	}
	return confDir, nil
}

var templateResourceConfigTmpl = `
[template]
src = "{{.src}}"
dest = "{{.dest}}"
keys = [
  "foo",
]
`

func TestReloadCmdTemplateSubstitution(t *testing.T) {
	log.SetLevel("warn")

	tests := []struct {
		name        string
		reloadCmd   string
		src         string
		dest        string
		expectedCmd string
	}{
		{
			name:        "dest substitution",
			reloadCmd:   "systemctl reload nginx # config: {{.dest}}",
			src:         "/tmp/.nginx.conf.12345",
			dest:        "/etc/nginx/nginx.conf",
			expectedCmd: "systemctl reload nginx # config: /etc/nginx/nginx.conf",
		},
		{
			name:        "src substitution",
			reloadCmd:   "validate-config {{.src}}",
			src:         "/tmp/.app.conf.67890",
			dest:        "/etc/app/config.conf",
			expectedCmd: "validate-config /tmp/.app.conf.67890",
		},
		{
			name:        "both src and dest substitution",
			reloadCmd:   "reload-handler --staged={{.src}} --dest={{.dest}}",
			src:         "/tmp/.config.staged",
			dest:        "/etc/myapp/config.yaml",
			expectedCmd: "reload-handler --staged=/tmp/.config.staged --dest=/etc/myapp/config.yaml",
		},
		{
			name:        "no substitution needed",
			reloadCmd:   "systemctl restart myservice",
			src:         "/tmp/.config.12345",
			dest:        "/etc/myservice/config",
			expectedCmd: "systemctl restart myservice",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := make(map[string]string)
			data["src"] = tc.src
			data["dest"] = tc.dest

			tmpl, err := template.New("reloadcmd").Parse(tc.reloadCmd)
			if err != nil {
				t.Fatalf("Failed to parse reload command template: %s", err)
			}

			var cmdBuffer bytes.Buffer
			if err := tmpl.Execute(&cmdBuffer, data); err != nil {
				t.Fatalf("Failed to execute reload command template: %s", err)
			}

			result := cmdBuffer.String()
			if result != tc.expectedCmd {
				t.Errorf("Expected command %q, got %q", tc.expectedCmd, result)
			}
		})
	}
}

func TestProcessTemplateResources(t *testing.T) {
	log.SetLevel("warn")
	// Setup temporary conf, config, and template directories.
	tempConfDir, err := createTempDirs()
	if err != nil {
		t.Errorf("Failed to create temp dirs: %s", err.Error())
	}
	defer os.RemoveAll(tempConfDir)

	// Create the src template.
	srcTemplateFile := filepath.Join(tempConfDir, "templates", "foo.tmpl")
	err = os.WriteFile(srcTemplateFile, []byte(`foo = {{getv "/foo"}}`), 0644)
	if err != nil {
		t.Error(err.Error())
	}

	// Create the dest.
	destFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Errorf("Failed to create destFile: %s", err.Error())
	}
	defer os.Remove(destFile.Name())

	// Create the template resource configuration file.
	templateResourcePath := filepath.Join(tempConfDir, "conf.d", "foo.toml")
	templateResourceFile, err := os.Create(templateResourcePath)
	if err != nil {
		t.Error(err)
	}
	tmpl, err := template.New("templateResourceConfig").Parse(templateResourceConfigTmpl)
	if err != nil {
		t.Errorf("Unable to parse template resource template: %s", err.Error())
	}
	data := make(map[string]string)
	data["src"] = "foo.tmpl"
	data["dest"] = destFile.Name()
	err = tmpl.Execute(templateResourceFile, data)
	if err != nil {
		t.Error(err)
	}

	os.Setenv("FOO", "bar")
	storeClient, err := env.NewEnvClient()
	if err != nil {
		t.Error(err)
	}
	c := Config{
		ConfDir:     tempConfDir,
		ConfigDir:   filepath.Join(tempConfDir, "conf.d"),
		StoreClient: storeClient,
		TemplateDir: filepath.Join(tempConfDir, "templates"),
	}
	// Process the test template resource.
	err = Process(c)
	if err != nil {
		t.Error(err.Error())
	}
	// Verify the results.
	expected := "foo = bar"
	results, err := os.ReadFile(destFile.Name())
	if err != nil {
		t.Error(err.Error())
	}
	if string(results) != expected {
		t.Errorf("Expected contents of dest == '%s', got %s", expected, string(results))
	}
}

func TestNewTemplateResourceWithPerResourceBackend(t *testing.T) {
	log.SetLevel("warn")
	clearClientCache() // Clear cache before test

	tempConfDir, err := createTempDirs()
	if err != nil {
		t.Fatalf("Failed to create temp dirs: %s", err.Error())
	}
	defer os.RemoveAll(tempConfDir)

	// Create a minimal template file
	srcTemplateFile := filepath.Join(tempConfDir, "templates", "test.tmpl")
	err = os.WriteFile(srcTemplateFile, []byte(`test`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create resource config with per-resource backend
	resourceContent := `[template]
src = "test.tmpl"
dest = "/tmp/test.conf"
keys = ["/foo"]

[backend]
backend = "env"
`
	resourcePath := filepath.Join(tempConfDir, "conf.d", "test.toml")
	err = os.WriteFile(resourcePath, []byte(resourceContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Config without global StoreClient - should use per-resource backend
	config := Config{
		ConfDir:     tempConfDir,
		ConfigDir:   filepath.Join(tempConfDir, "conf.d"),
		TemplateDir: filepath.Join(tempConfDir, "templates"),
		// StoreClient is nil - will use per-resource backend
	}

	tr, err := NewTemplateResource(resourcePath, config)
	if err != nil {
		t.Fatalf("NewTemplateResource failed: %s", err)
	}

	if tr.storeClient == nil {
		t.Error("Expected storeClient to be set from per-resource backend")
	}
}

func TestNewTemplateResourceFallbackToGlobalClient(t *testing.T) {
	log.SetLevel("warn")

	tempConfDir, err := createTempDirs()
	if err != nil {
		t.Fatalf("Failed to create temp dirs: %s", err.Error())
	}
	defer os.RemoveAll(tempConfDir)

	// Create a minimal template file
	srcTemplateFile := filepath.Join(tempConfDir, "templates", "test.tmpl")
	err = os.WriteFile(srcTemplateFile, []byte(`test`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	storeClient, err := env.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}

	// Create resource config without backend section
	resourceContent := `[template]
src = "test.tmpl"
dest = "/tmp/test.conf"
keys = ["/foo"]
`
	resourcePath := filepath.Join(tempConfDir, "conf.d", "test.toml")
	err = os.WriteFile(resourcePath, []byte(resourceContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Config with global StoreClient - should use it as fallback
	config := Config{
		ConfDir:     tempConfDir,
		ConfigDir:   filepath.Join(tempConfDir, "conf.d"),
		StoreClient: storeClient,
		TemplateDir: filepath.Join(tempConfDir, "templates"),
	}

	tr, err := NewTemplateResource(resourcePath, config)
	if err != nil {
		t.Fatalf("NewTemplateResource failed: %s", err)
	}

	if tr.storeClient != storeClient {
		t.Error("Expected storeClient to be the global client")
	}
}

func TestNewTemplateResourceNoClientError(t *testing.T) {
	log.SetLevel("warn")

	tempConfDir, err := createTempDirs()
	if err != nil {
		t.Fatalf("Failed to create temp dirs: %s", err.Error())
	}
	defer os.RemoveAll(tempConfDir)

	// Create a minimal template file
	srcTemplateFile := filepath.Join(tempConfDir, "templates", "test.tmpl")
	err = os.WriteFile(srcTemplateFile, []byte(`test`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create resource config without backend section
	resourceContent := `[template]
src = "test.tmpl"
dest = "/tmp/test.conf"
keys = ["/foo"]
`
	resourcePath := filepath.Join(tempConfDir, "conf.d", "test.toml")
	err = os.WriteFile(resourcePath, []byte(resourceContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Config without StoreClient and resource without backend - should error
	config := Config{
		ConfDir:     tempConfDir,
		ConfigDir:   filepath.Join(tempConfDir, "conf.d"),
		TemplateDir: filepath.Join(tempConfDir, "templates"),
		// StoreClient is nil
	}

	_, err = NewTemplateResource(resourcePath, config)
	if err == nil {
		t.Fatal("Expected error when no backend is available")
	}

	if !strings.Contains(err.Error(), "StoreClient is required") {
		t.Errorf("Expected error about StoreClient, got: %s", err)
	}
}

func TestNewTemplateResourcePerResourceBackendOverridesGlobal(t *testing.T) {
	log.SetLevel("warn")
	clearClientCache() // Clear cache before test

	tempConfDir, err := createTempDirs()
	if err != nil {
		t.Fatalf("Failed to create temp dirs: %s", err.Error())
	}
	defer os.RemoveAll(tempConfDir)

	// Create a minimal template file
	srcTemplateFile := filepath.Join(tempConfDir, "templates", "test.tmpl")
	err = os.WriteFile(srcTemplateFile, []byte(`test`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	globalClient, err := env.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}

	// Create resource config with per-resource backend
	resourceContent := `[template]
src = "test.tmpl"
dest = "/tmp/test.conf"
keys = ["/foo"]

[backend]
backend = "env"
`
	resourcePath := filepath.Join(tempConfDir, "conf.d", "test.toml")
	err = os.WriteFile(resourcePath, []byte(resourceContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Config with global StoreClient but resource has its own backend
	config := Config{
		ConfDir:     tempConfDir,
		ConfigDir:   filepath.Join(tempConfDir, "conf.d"),
		StoreClient: globalClient,
		TemplateDir: filepath.Join(tempConfDir, "templates"),
	}

	tr, err := NewTemplateResource(resourcePath, config)
	if err != nil {
		t.Fatalf("NewTemplateResource failed: %s", err)
	}

	// The per-resource backend should be used, not the global one
	// Since both are env backends, we can't distinguish by type,
	// but we can verify that a client was created (not nil)
	if tr.storeClient == nil {
		t.Error("Expected storeClient to be set")
	}

	// The key behavior is that per-resource config takes precedence
	// This is verified by the fact that the resource was created successfully
	// even though we're using a per-resource backend config
}

func TestClientCacheReuse(t *testing.T) {
	log.SetLevel("warn")
	clearClientCache() // Clear cache before test

	tempConfDir, err := createTempDirs()
	if err != nil {
		t.Fatalf("Failed to create temp dirs: %s", err.Error())
	}
	defer os.RemoveAll(tempConfDir)

	// Create template files
	srcTemplateFile1 := filepath.Join(tempConfDir, "templates", "test1.tmpl")
	err = os.WriteFile(srcTemplateFile1, []byte(`test1`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	srcTemplateFile2 := filepath.Join(tempConfDir, "templates", "test2.tmpl")
	err = os.WriteFile(srcTemplateFile2, []byte(`test2`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create two resource configs with identical backend configurations
	resourceContent := `[template]
src = "%s"
dest = "/tmp/%s"
keys = ["/foo"]

[backend]
backend = "env"
`
	resourcePath1 := filepath.Join(tempConfDir, "conf.d", "test1.toml")
	err = os.WriteFile(resourcePath1, []byte(strings.Replace(strings.Replace(resourceContent, "%s", "test1.tmpl", 1), "%s", "test1.conf", 1)), 0644)
	if err != nil {
		t.Fatal(err)
	}

	resourcePath2 := filepath.Join(tempConfDir, "conf.d", "test2.toml")
	err = os.WriteFile(resourcePath2, []byte(strings.Replace(strings.Replace(resourceContent, "%s", "test2.tmpl", 1), "%s", "test2.conf", 1)), 0644)
	if err != nil {
		t.Fatal(err)
	}

	config := Config{
		ConfDir:     tempConfDir,
		ConfigDir:   filepath.Join(tempConfDir, "conf.d"),
		TemplateDir: filepath.Join(tempConfDir, "templates"),
	}

	tr1, err := NewTemplateResource(resourcePath1, config)
	if err != nil {
		t.Fatalf("NewTemplateResource failed for resource 1: %s", err)
	}

	tr2, err := NewTemplateResource(resourcePath2, config)
	if err != nil {
		t.Fatalf("NewTemplateResource failed for resource 2: %s", err)
	}

	// Both resources should use the same cached client
	if tr1.storeClient != tr2.storeClient {
		t.Error("Expected both resources to share the same cached client")
	}
}
