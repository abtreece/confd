package template

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/abtreece/confd/pkg/backends/env"
)

// reloadSrcResult is what reload_cmd saw through {{.src}} when it ran.
type reloadSrcResult struct {
	dest       string
	srcPath    string
	srcContent string
}

// processWithReloadSrcRecorder processes a resource once. Its reload_cmd
// copies {{.src}} and records its path, so it fails, and with it process(),
// when {{.src}} does not name a readable file at the time the command runs.
func processWithReloadSrcRecorder(t *testing.T, config Config) reloadSrcResult {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("reload_cmd in this test is a POSIX shell command")
	}
	t.Setenv("RELOAD_SRC_VALUE", "rendered")

	workDir := t.TempDir()
	confDir := filepath.Join(workDir, "conf.d")
	templateDir := filepath.Join(workDir, "templates")
	for _, dir := range []string{confDir, templateDir} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	dest := filepath.Join(workDir, "app.conf")
	srcCopy := filepath.Join(workDir, "src.copy")
	srcLog := filepath.Join(workDir, "src.log")

	if err := os.WriteFile(filepath.Join(templateDir, "app.tmpl"), []byte(`{{getv "/reload/src/value"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	resourceContent := fmt.Sprintf(`[template]
src = "app.tmpl"
dest = "%s"
keys = ["/reload/src/value"]
reload_cmd = "cp {{.src}} %s && echo {{.src}} >> %s"
`, dest, srcCopy, srcLog)
	resourcePath := filepath.Join(confDir, "app.toml")
	if err := os.WriteFile(resourcePath, []byte(resourceContent), 0644); err != nil {
		t.Fatal(err)
	}

	storeClient, err := env.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	config.ConfDir = workDir
	config.ConfigDir = confDir
	config.TemplateDir = templateDir
	config.StoreClient = storeClient

	resource, err := NewTemplateResource(resourcePath, config)
	if err != nil {
		t.Fatalf("NewTemplateResource failed: %v", err)
	}
	if err := resource.process(); err != nil {
		t.Fatalf("process() returned %v", err)
	}

	logged, err := os.ReadFile(srcLog)
	if err != nil {
		t.Fatal(err)
	}
	srcPaths := strings.Fields(string(logged))
	if len(srcPaths) != 1 {
		t.Fatalf("reload_cmd ran %d time(s), want 1", len(srcPaths))
	}
	copied, err := os.ReadFile(srcCopy)
	if err != nil {
		t.Fatal(err)
	}
	return reloadSrcResult{dest: dest, srcPath: srcPaths[0], srcContent: string(copied)}
}

func TestReloadCmd_SrcIsDestinationAfterMove(t *testing.T) {
	result := processWithReloadSrcRecorder(t, Config{})

	if result.srcPath != result.dest {
		t.Errorf("{{.src}} = %s, want the destination %s", result.srcPath, result.dest)
	}
	if _, err := os.Stat(result.srcPath); err != nil {
		t.Errorf("{{.src}} does not name an existing file: %v", err)
	}
	if result.srcContent != "rendered" {
		t.Errorf("{{.src}} held %q when reload_cmd ran, want %q", result.srcContent, "rendered")
	}
}

func TestReloadCmd_SrcIsStageFileWhenKept(t *testing.T) {
	result := processWithReloadSrcRecorder(t, Config{KeepStageFile: true})

	if result.srcPath == result.dest {
		t.Errorf("{{.src}} = the destination %s, want the kept stage file", result.dest)
	}
	if result.srcContent != "rendered" {
		t.Errorf("{{.src}} held %q when reload_cmd ran, want %q", result.srcContent, "rendered")
	}
	content, err := os.ReadFile(result.srcPath)
	if err != nil {
		t.Fatalf("{{.src}} does not name an existing file: %v", err)
	}
	if string(content) != "rendered" {
		t.Errorf("kept stage file holds %q, want %q", content, "rendered")
	}
}
