package template

import (
	"os"
	"path/filepath"
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
		t.Errorf(err.Error())
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
		t.Errorf(err.Error())
	}

	os.Setenv("FOO", "bar")
	storeClient, err := env.NewEnvClient()
	if err != nil {
		t.Errorf(err.Error())
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
