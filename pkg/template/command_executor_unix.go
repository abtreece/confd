//go:build !windows

package template

import (
	"os/exec"
	"syscall"
)

// setupProcessGroup configures the command to run in a new process group on Unix.
// This ensures all child processes are killed when the command is cancelled.
func setupProcessGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		// Kill the process group (negative PID kills the entire group)
		return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	}
}
