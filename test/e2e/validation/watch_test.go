//go:build e2e

package validation

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

// TestWatchMode_IntervalOutOfRangeFails verifies that watch mode rejects an
// --interval that cannot drive the poller (below 1 second, or too large for a
// time.Duration) at startup with an error instead of a panic, and that
// --onetime, which never polls, does not check it.
func TestWatchMode_IntervalOutOfRangeFails(t *testing.T) {
	t.Parallel()

	env := operations.NewTestEnv(t)
	yamlPath := filepath.Join(env.BaseDir, "data.yaml")
	if err := os.WriteFile(yamlPath, []byte("app:\n  name: demo\n"), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}
	env.WriteTemplate("app.tmpl", "static\n")
	env.WriteConfig("app.toml", `[template]
src = "app.tmpl"
dest = "`+env.DestPath("app.conf")+`"
keys = ["/app"]
`)
	confd := operations.BuildConfd(t)

	tests := []struct {
		interval string
		want     string
	}{
		{"0", "invalid --interval 0: must be at least 1 second with --watch"},
		{"-1", "invalid --interval -1: must be at least 1 second with --watch"},
		{"9223372037", "invalid --interval 9223372037: must be at most 9223372036 seconds with --watch"},
	}
	for _, tt := range tests {
		t.Run(tt.interval, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// The file backend can watch, so only --interval can fail startup.
			args := []string{"file", "--file", yamlPath, "--confdir", env.ConfDir, "--watch", "--interval=" + tt.interval}
			output, err := exec.CommandContext(ctx, confd, args...).CombinedOutput()

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("confd %v: want a non-zero exit, got err=%v\nOutput:\n%s", args, err, output)
			}
			if ctx.Err() != nil {
				t.Fatalf("confd %v did not exit at startup: %v\nOutput:\n%s", args, ctx.Err(), output)
			}
			if !strings.Contains(string(output), tt.want) {
				t.Errorf("want %q in the output:\n%s", tt.want, output)
			}
			if strings.Contains(string(output), "panic:") {
				t.Errorf("confd panicked:\n%s", output)
			}
		})
	}

	t.Run("onetime ignores interval", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		args := []string{"file", "--file", yamlPath, "--confdir", env.ConfDir, "--watch", "--onetime", "--interval=0"}
		output, err := exec.CommandContext(ctx, confd, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("confd %v: %v\nOutput:\n%s", args, err, output)
		}
	})
}
