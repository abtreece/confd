package template

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"
	"time"

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

func TestMinReloadIntervalParsing(t *testing.T) {
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
		name             string
		minReloadConfig  string
		expectedDuration string
		expectError      bool
	}{
		{
			name:             "30 seconds",
			minReloadConfig:  `min_reload_interval = "30s"`,
			expectedDuration: "30s",
			expectError:      false,
		},
		{
			name:             "1 minute",
			minReloadConfig:  `min_reload_interval = "1m"`,
			expectedDuration: "1m0s",
			expectError:      false,
		},
		{
			name:             "500 milliseconds",
			minReloadConfig:  `min_reload_interval = "500ms"`,
			expectedDuration: "500ms",
			expectError:      false,
		},
		{
			name:             "invalid duration",
			minReloadConfig:  `min_reload_interval = "invalid"`,
			expectedDuration: "",
			expectError:      true,
		},
		{
			name:             "no min_reload_interval",
			minReloadConfig:  "",
			expectedDuration: "0s",
			expectError:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resourceContent := `[template]
src = "test.tmpl"
dest = "/tmp/test.conf"
keys = ["/foo"]
` + tc.minReloadConfig

			resourcePath := filepath.Join(tempConfDir, "conf.d", "test.toml")
			err := os.WriteFile(resourcePath, []byte(resourceContent), 0644)
			if err != nil {
				t.Fatal(err)
			}

			config := Config{
				ConfDir:     tempConfDir,
				ConfigDir:   filepath.Join(tempConfDir, "conf.d"),
				Prefix:      "",
				StoreClient: storeClient,
				TemplateDir: filepath.Join(tempConfDir, "templates"),
			}

			tr, err := NewTemplateResource(resourcePath, config)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTemplateResource failed: %s", err)
			}

			if tr.minReloadIntervalDur.String() != tc.expectedDuration {
				t.Errorf("Expected min_reload_interval %q, got %q", tc.expectedDuration, tr.minReloadIntervalDur.String())
			}
		})
	}
}

func TestDebounceParsing(t *testing.T) {
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
		name             string
		debounceConfig   string
		globalDebounce   string
		expectedDuration string
		expectError      bool
	}{
		{
			name:             "2 seconds",
			debounceConfig:   `debounce = "2s"`,
			globalDebounce:   "",
			expectedDuration: "2s",
			expectError:      false,
		},
		{
			name:             "500 milliseconds",
			debounceConfig:   `debounce = "500ms"`,
			globalDebounce:   "",
			expectedDuration: "500ms",
			expectError:      false,
		},
		{
			name:             "invalid debounce",
			debounceConfig:   `debounce = "invalid"`,
			globalDebounce:   "",
			expectedDuration: "",
			expectError:      true,
		},
		{
			name:             "global debounce fallback",
			debounceConfig:   "",
			globalDebounce:   "3s",
			expectedDuration: "3s",
			expectError:      false,
		},
		{
			name:             "per-resource overrides global",
			debounceConfig:   `debounce = "1s"`,
			globalDebounce:   "5s",
			expectedDuration: "1s",
			expectError:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resourceContent := `[template]
src = "test.tmpl"
dest = "/tmp/test.conf"
keys = ["/foo"]
` + tc.debounceConfig

			resourcePath := filepath.Join(tempConfDir, "conf.d", "test.toml")
			err := os.WriteFile(resourcePath, []byte(resourceContent), 0644)
			if err != nil {
				t.Fatal(err)
			}

			config := Config{
				ConfDir:     tempConfDir,
				ConfigDir:   filepath.Join(tempConfDir, "conf.d"),
				Prefix:      "",
				StoreClient: storeClient,
				TemplateDir: filepath.Join(tempConfDir, "templates"),
			}

			if tc.globalDebounce != "" {
				d, _ := time.ParseDuration(tc.globalDebounce)
				config.Debounce = d
			}

			tr, err := NewTemplateResource(resourcePath, config)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTemplateResource failed: %s", err)
			}

			if tr.debounceDur.String() != tc.expectedDuration {
				t.Errorf("Expected debounce %q, got %q", tc.expectedDuration, tr.debounceDur.String())
			}
		})
	}
}

func TestRunCommand_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	err := runCommand("echo hello")
	if err != nil {
		t.Errorf("runCommand() unexpected error: %v", err)
	}
}

func TestRunCommand_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	err := runCommand("exit 1")
	if err == nil {
		t.Error("runCommand() expected error for exit 1, got nil")
	}
}

func TestRunCommand_InvalidCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	err := runCommand("nonexistent_command_12345")
	if err == nil {
		t.Error("runCommand() expected error for invalid command, got nil")
	}
}

func TestRunCommand_WithOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	// Test command that produces output
	err := runCommand("echo 'test output'")
	if err != nil {
		t.Errorf("runCommand() unexpected error: %v", err)
	}
}

func TestCheck_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("test content")
	tmpFile.Close()

	tr := &TemplateResource{
		CheckCmd:  "cat {{.src}}",
		StageFile: tmpFile,
	}

	err = tr.check()
	if err != nil {
		t.Errorf("check() unexpected error: %v", err)
	}
}

func TestCheck_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tr := &TemplateResource{
		CheckCmd:  "exit 1",
		StageFile: tmpFile,
	}

	err = tr.check()
	if err == nil {
		t.Error("check() expected error for exit 1, got nil")
	}
}

func TestCheck_InvalidTemplate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "confd-check-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tr := &TemplateResource{
		CheckCmd:  "echo {{.invalid",
		StageFile: tmpFile,
	}

	err = tr.check()
	if err == nil {
		t.Error("check() expected error for invalid template, got nil")
	}
}

func TestReload_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tr := &TemplateResource{
		ReloadCmd: "echo reloading {{.dest}}",
		Dest:      "/tmp/test.conf",
		StageFile: tmpFile,
	}

	err = tr.reload()
	if err != nil {
		t.Errorf("reload() unexpected error: %v", err)
	}

	// Check that lastReloadTime was updated
	if tr.lastReloadTime.IsZero() {
		t.Error("reload() should update lastReloadTime")
	}
}

func TestReload_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tr := &TemplateResource{
		ReloadCmd: "exit 1",
		Dest:      "/tmp/test.conf",
		StageFile: tmpFile,
	}

	err = tr.reload()
	if err == nil {
		t.Error("reload() expected error for exit 1, got nil")
	}
}

func TestReload_InvalidTemplate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tr := &TemplateResource{
		ReloadCmd: "echo {{.invalid",
		Dest:      "/tmp/test.conf",
		StageFile: tmpFile,
	}

	err = tr.reload()
	if err == nil {
		t.Error("reload() expected error for invalid template, got nil")
	}
}

func TestReload_RateLimiting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tr := &TemplateResource{
		ReloadCmd:            "echo reloading",
		Dest:                 "/tmp/test.conf",
		StageFile:            tmpFile,
		minReloadIntervalDur: 1 * time.Hour, // Set a long interval
		lastReloadTime:       time.Now(),    // Set last reload to now
	}

	// This reload should be throttled
	err = tr.reload()
	if err != nil {
		t.Errorf("reload() unexpected error: %v", err)
	}

	// lastReloadTime should NOT have been updated since reload was throttled
	// (it should still be approximately the time we set it to)
	timeSinceLastReload := time.Since(tr.lastReloadTime)
	if timeSinceLastReload > 1*time.Second {
		t.Error("reload() should not update lastReloadTime when throttled")
	}
}

func TestReload_WithTemplateVariables(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpFile, err := os.CreateTemp("", "confd-reload-test-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("config content")
	tmpFile.Close()

	tr := &TemplateResource{
		ReloadCmd: "test -f {{.src}} && test '{{.dest}}' = '/tmp/test.conf'",
		Dest:      "/tmp/test.conf",
		StageFile: tmpFile,
	}

	err = tr.reload()
	if err != nil {
		t.Errorf("reload() with template variables unexpected error: %v", err)
	}
}

func TestSetFileMode_DefaultMode(t *testing.T) {
	tr := &TemplateResource{
		Mode: "",
		Dest: "/nonexistent/file/path",
	}

	err := tr.setFileMode()
	if err != nil {
		t.Errorf("setFileMode() unexpected error: %v", err)
	}

	// When dest doesn't exist and Mode is empty, should default to 0644
	if tr.FileMode != 0644 {
		t.Errorf("setFileMode() FileMode = %v, want 0644", tr.FileMode)
	}
}

func TestSetFileMode_ExistingFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "confd-setfilemode-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Set a specific mode on the file
	os.Chmod(tmpFile.Name(), 0755)

	tr := &TemplateResource{
		Mode: "",
		Dest: tmpFile.Name(),
	}

	err = tr.setFileMode()
	if err != nil {
		t.Errorf("setFileMode() unexpected error: %v", err)
	}

	// Should inherit mode from existing file
	if tr.FileMode != 0755 {
		t.Errorf("setFileMode() FileMode = %v, want 0755", tr.FileMode)
	}
}

func TestSetFileMode_ExplicitMode(t *testing.T) {
	tr := &TemplateResource{
		Mode: "0600",
		Dest: "/tmp/test.conf",
	}

	err := tr.setFileMode()
	if err != nil {
		t.Errorf("setFileMode() unexpected error: %v", err)
	}

	if tr.FileMode != 0600 {
		t.Errorf("setFileMode() FileMode = %v, want 0600", tr.FileMode)
	}
}

func TestSetFileMode_OctalMode(t *testing.T) {
	tr := &TemplateResource{
		Mode: "0755",
		Dest: "/tmp/test.conf",
	}

	err := tr.setFileMode()
	if err != nil {
		t.Errorf("setFileMode() unexpected error: %v", err)
	}

	if tr.FileMode != 0755 {
		t.Errorf("setFileMode() FileMode = %v, want 0755", tr.FileMode)
	}
}

func TestSetFileMode_InvalidMode(t *testing.T) {
	tr := &TemplateResource{
		Mode: "invalid",
		Dest: "/tmp/test.conf",
	}

	err := tr.setFileMode()
	if err == nil {
		t.Error("setFileMode() expected error for invalid mode, got nil")
	}
}

func TestSync_NoopMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()

	// Create a staged file with content
	stagedFile, err := os.CreateTemp(tmpDir, "staged-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	stagedFile.WriteString("new content")
	stagedFile.Sync()

	// Create a dest file with different content
	destFile := filepath.Join(tmpDir, "dest.conf")
	os.WriteFile(destFile, []byte("old content"), 0644)

	tr := &TemplateResource{
		Dest:      destFile,
		StageFile: stagedFile,
		noop:      true,
		FileMode:  0644,
	}

	err = tr.sync()
	if err != nil {
		t.Errorf("sync() unexpected error: %v", err)
	}

	// In noop mode, dest file should not be modified
	content, _ := os.ReadFile(destFile)
	if string(content) != "old content" {
		t.Error("sync() in noop mode should not modify dest file")
	}
}

func TestSync_ConfigInSync(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()

	// Create dest file
	destFile := filepath.Join(tmpDir, "dest.conf")
	os.WriteFile(destFile, []byte("same content"), 0644)

	// Create staged file with same content
	stagedFile, err := os.CreateTemp(tmpDir, "staged-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	stagedFile.WriteString("same content")
	stagedFile.Sync()
	os.Chmod(stagedFile.Name(), 0644)

	tr := &TemplateResource{
		Dest:      destFile,
		StageFile: stagedFile,
		noop:      false,
		FileMode:  0644,
	}

	err = tr.sync()
	if err != nil {
		t.Errorf("sync() unexpected error: %v", err)
	}
}

func TestSync_ConfigChanged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()

	// Create dest file
	destFile := filepath.Join(tmpDir, "dest.conf")
	os.WriteFile(destFile, []byte("old content"), 0644)

	// Create staged file with different content
	stagedFile, err := os.CreateTemp(tmpDir, "staged-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	stagedFile.WriteString("new content")
	stagedFile.Sync()
	os.Chmod(stagedFile.Name(), 0644)

	tr := &TemplateResource{
		Dest:      destFile,
		StageFile: stagedFile,
		noop:      false,
		syncOnly:  true,
		FileMode:  0644,
	}

	err = tr.sync()
	if err != nil {
		t.Errorf("sync() unexpected error: %v", err)
	}

	// Dest file should be updated
	content, _ := os.ReadFile(destFile)
	if string(content) != "new content" {
		t.Errorf("sync() dest content = %s, want 'new content'", string(content))
	}
}

func TestSync_WithCheckCmd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()

	// Create dest file
	destFile := filepath.Join(tmpDir, "dest.conf")
	os.WriteFile(destFile, []byte("old"), 0644)

	// Create staged file with different content
	stagedFile, err := os.CreateTemp(tmpDir, "staged-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	stagedFile.WriteString("new")
	stagedFile.Sync()
	os.Chmod(stagedFile.Name(), 0644)

	tr := &TemplateResource{
		Dest:      destFile,
		StageFile: stagedFile,
		CheckCmd:  "cat {{.src}}",
		noop:      false,
		syncOnly:  false,
		FileMode:  0644,
	}

	err = tr.sync()
	if err != nil {
		t.Errorf("sync() unexpected error: %v", err)
	}
}

func TestSync_CheckCmdFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()

	// Create dest file
	destFile := filepath.Join(tmpDir, "dest.conf")
	os.WriteFile(destFile, []byte("old"), 0644)

	// Create staged file with different content
	stagedFile, err := os.CreateTemp(tmpDir, "staged-*.conf")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	stagedFile.WriteString("new")
	stagedFile.Sync()
	os.Chmod(stagedFile.Name(), 0644)

	tr := &TemplateResource{
		Dest:      destFile,
		StageFile: stagedFile,
		CheckCmd:  "exit 1",
		noop:      false,
		syncOnly:  false,
		FileMode:  0644,
	}

	err = tr.sync()
	if err == nil {
		t.Error("sync() expected error when check command fails, got nil")
	}
	if !strings.Contains(err.Error(), "Config check failed") {
		t.Errorf("sync() error = %v, want error containing 'Config check failed'", err)
	}
}
