//go:build windows

package template

import (
	"os/exec"
)

// setupProcessGroup is a no-op on Windows.
// Windows doesn't support Unix-style process groups in the same way.
// The default CommandContext behavior will handle process termination.
func setupProcessGroup(c *exec.Cmd) {
	// No-op on Windows
}
