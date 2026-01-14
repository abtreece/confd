package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"text/template"
	"time"

	"github.com/abtreece/confd/pkg/log"
)

// commandExecutor handles execution of check and reload commands.
// It encapsulates command template parsing, execution, and rate limiting.
type commandExecutor struct {
	checkCmd          string
	reloadCmd         string
	minReloadInterval time.Duration
	lastReloadTime    *time.Time // pointer to share state with TemplateResource
	syncOnly          bool
	ctx               context.Context
	checkCmdTimeout   time.Duration
	reloadCmdTimeout  time.Duration
}

// commandExecutorConfig holds configuration for creating a commandExecutor.
type commandExecutorConfig struct {
	CheckCmd          string
	ReloadCmd         string
	MinReloadInterval time.Duration
	LastReloadTime    *time.Time
	SyncOnly          bool
	Ctx               context.Context
	CheckCmdTimeout   time.Duration
	ReloadCmdTimeout  time.Duration
}

// newCommandExecutor creates a new commandExecutor instance.
func newCommandExecutor(config commandExecutorConfig) *commandExecutor {
	ctx := config.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return &commandExecutor{
		checkCmd:          config.CheckCmd,
		reloadCmd:         config.ReloadCmd,
		minReloadInterval: config.MinReloadInterval,
		lastReloadTime:    config.LastReloadTime,
		syncOnly:          config.SyncOnly,
		ctx:               ctx,
		checkCmdTimeout:   config.CheckCmdTimeout,
		reloadCmdTimeout:  config.ReloadCmdTimeout,
	}
}

// executeCheck executes the check command to validate the staged configuration.
// The command template can reference {{.src}} which is substituted with the
// staged file path.
// It returns an error if the check command fails or times out.
func (e *commandExecutor) executeCheck(stagePath string) error {
	if e.checkCmd == "" || e.syncOnly {
		return nil
	}

	var cmdBuffer bytes.Buffer
	data := map[string]string{"src": stagePath}
	tmpl, err := template.New("checkcmd").Parse(e.checkCmd)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(&cmdBuffer, data); err != nil {
		return err
	}
	if err := e.runCommandWithTimeout(cmdBuffer.String(), e.checkCmdTimeout); err != nil {
		return errors.New("Config check failed: " + err.Error())
	}
	return nil
}

// executeReload executes the reload command to notify the application of config changes.
// The command template can reference {{.src}} (staged file) and {{.dest}} (destination file).
// If minReloadInterval is set, reloads are rate-limited to prevent command spam.
// It returns nil if the reload is skipped due to rate limiting or if the command succeeds.
func (e *commandExecutor) executeReload(stagePath, destPath string) error {
	if e.reloadCmd == "" || e.syncOnly {
		return nil
	}

	// Check rate limiting
	if e.minReloadInterval > 0 && e.lastReloadTime != nil && !e.lastReloadTime.IsZero() {
		elapsed := time.Since(*e.lastReloadTime)
		if elapsed < e.minReloadInterval {
			remaining := e.minReloadInterval - elapsed
			log.Warning("Reload throttled for %s (next allowed in %v)", destPath, remaining.Round(time.Second))
			return nil
		}
	}

	var cmdBuffer bytes.Buffer
	data := map[string]string{"src": stagePath, "dest": destPath}
	tmpl, err := template.New("reloadcmd").Parse(e.reloadCmd)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(&cmdBuffer, data); err != nil {
		return err
	}

	if err := e.runCommandWithTimeout(cmdBuffer.String(), e.reloadCmdTimeout); err != nil {
		return err
	}

	// Update last reload time on success
	if e.lastReloadTime != nil {
		*e.lastReloadTime = time.Now()
	}
	return nil
}

// runCommandWithTimeout executes the given command with the specified timeout.
// If timeout is 0, no timeout is applied (command can run indefinitely).
// It handles cross-platform execution (Windows vs Unix) using exec.CommandContext.
// On Unix systems, it creates a new process group to ensure all child processes
// are killed when the command times out or is cancelled.
// It returns an error if the command fails, times out, or the context is cancelled.
func (e *commandExecutor) runCommandWithTimeout(cmd string, timeout time.Duration) error {
	start := time.Now()
	logger := log.With("command", cmd, "timeout", timeout.String())
	logger.DebugContext(e.ctx, "Starting command execution")

	ctx := e.ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/C", cmd)
	} else {
		c = exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
		// Set up process group handling for proper child process cleanup
		setupProcessGroup(c)
	}

	output, err := c.CombinedOutput()
	duration := time.Since(start)
	outputSize := len(output)

	// Extract exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	// Truncate output for logging if too large
	outputStr := string(output)
	const maxOutputLen = 500
	if len(outputStr) > maxOutputLen {
		outputStr = outputStr[:maxOutputLen] + "... (truncated)"
	}

	if err != nil {
		// Check if it was a timeout or context cancellation
		if ctx.Err() == context.DeadlineExceeded {
			logger.ErrorContext(e.ctx, "Command timed out",
				"exit_code", exitCode,
				"duration_ms", duration.Milliseconds(),
				"output_size_bytes", outputSize,
				"output", outputStr)
			return fmt.Errorf("command timed out after %v", timeout)
		}
		if ctx.Err() == context.Canceled {
			logger.DebugContext(e.ctx, "Command cancelled",
				"exit_code", exitCode,
				"duration_ms", duration.Milliseconds(),
				"output_size_bytes", outputSize)
			return fmt.Errorf("command cancelled")
		}
		logger.ErrorContext(e.ctx, "Command failed",
			"exit_code", exitCode,
			"duration_ms", duration.Milliseconds(),
			"output_size_bytes", outputSize,
			"output", outputStr,
			"error", err.Error())
		return err
	}

	logger.InfoContext(e.ctx, "Command completed successfully",
		"exit_code", exitCode,
		"duration_ms", duration.Milliseconds(),
		"output_size_bytes", outputSize)
	logger.DebugContext(e.ctx, "Command output", "output", outputStr)
	return nil
}

// runCommand executes the given command and logs its output.
// It handles cross-platform execution (Windows vs Unix).
// It returns nil if the command returns 0, otherwise returns the error.
func runCommand(cmd string) error {
	log.Debug("Running %s", cmd)
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/C", cmd)
	} else {
		c = exec.Command("/bin/sh", "-c", cmd)
	}

	output, err := c.CombinedOutput()
	if err != nil {
		log.Error("%q", string(output))
		return err
	}
	log.Debug("%q", string(output))
	return nil
}
