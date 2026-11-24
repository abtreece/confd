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

// TestIntervalOutOfRangeFails verifies that interval mode and watch mode
// reject an --interval that cannot drive a ticker (below 1 second, or too
// large for a time.Duration) at startup with an error instead of a panic,
// and that --onetime, which never ticks, does not check it.
func TestIntervalOutOfRangeFails(t *testing.T) {
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

	// The file backend can watch, so in watch mode only --interval can fail
	// startup.
	modes := []struct {
		name  string
		flags []string
	}{
		{"interval", nil},
		{"watch", []string{"--watch"}},
	}
	intervals := []struct {
		value string
		want  string
	}{
		{"0", "invalid --interval 0: must be at least 1 second"},
		{"-1", "invalid --interval -1: must be at least 1 second"},
		{"9223372037", "invalid --interval 9223372037: must be at most 9223372036 seconds"},
	}
	for _, mode := range modes {
		for _, interval := range intervals {
			t.Run(mode.name+"/"+interval.value, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				args := append([]string{"file", "--file", yamlPath, "--confdir", env.ConfDir, "--interval=" + interval.value}, mode.flags...)
				output, err := exec.CommandContext(ctx, confd, args...).CombinedOutput()

				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("confd %v: want a non-zero exit, got err=%v\nOutput:\n%s", args, err, output)
				}
				if ctx.Err() != nil {
					t.Fatalf("confd %v did not exit at startup: %v\nOutput:\n%s", args, ctx.Err(), output)
				}
				// The message ends the line: nothing mode-specific follows it.
				if !strings.Contains(string(output), interval.want+"\n") {
					t.Errorf("want a line ending in %q in the output:\n%s", interval.want, output)
				}
				if strings.Contains(string(output), "panic:") {
					t.Errorf("confd panicked:\n%s", output)
				}
			})
		}

		t.Run(mode.name+"/onetime ignores interval", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			args := append([]string{"file", "--file", yamlPath, "--confdir", env.ConfDir, "--onetime", "--interval=0"}, mode.flags...)
			output, err := exec.CommandContext(ctx, confd, args...).CombinedOutput()
			if err != nil {
				t.Fatalf("confd %v: %v\nOutput:\n%s", args, err, output)
			}
		})
	}
}
