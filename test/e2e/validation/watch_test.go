//go:build e2e

package validation

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/abtreece/confd/test/e2e/operations"
)

// TestWatchMode_GlobalBackendThatCannotWatchFails verifies that watch mode
// still rejects a global backend that cannot watch at startup.
func TestWatchMode_GlobalBackendThatCannotWatchFails(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	env.WriteTemplate("app.tmpl", "name={{ getv \"/app/name\" }}\n")
	env.WriteConfig("app.toml", `[template]
src = "app.tmpl"
dest = "`+env.DestPath("app.conf")+`"
keys = ["/app"]
`)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, operations.BuildConfd(t),
		"env", "--confdir", env.ConfDir, "--watch", "--onetime")
	output, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Expected confd env --watch to exit non-zero, got err=%v\nOutput:\n%s", err, output)
	}
	if ctx.Err() != nil {
		t.Fatalf("confd env --watch did not exit at startup: %v\nOutput:\n%s", ctx.Err(), output)
	}

	const want = "watch mode not supported for env backend"
	if !strings.Contains(string(output), want) {
		t.Errorf("Expected output to contain %q, got:\n%s", want, output)
	}
}
