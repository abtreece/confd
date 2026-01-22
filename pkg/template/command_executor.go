package template

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"text/template"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/metrics"
)

// commandExecutor handles execution of check and reload commands.
// It encapsulates command template parsing, execution, and rate limiting.
// Command templates are pre-compiled at construction time for efficiency.
type commandExecutor struct {
	checkCmd          string
	reloadCmd         string
	checkCmdTmpl      *template.Template // pre-compiled check command template
	reloadCmdTmpl     *template.Template // pre-compiled reload command template
	minReloadInterval time.Duration
	lastReloadTime    *time.Time // pointer to share state with TemplateResource
	syncOnly          bool
	ctx               context.Context
	checkCmdTimeout   time.Duration
	reloadCmdTimeout  time.Duration
	dest              string // destination path for metrics labeling
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
	Dest              string
}

// newCommandExecutor creates a new commandExecutor instance.
// It pre-compiles command templates for efficiency and returns an error if
// template parsing fails, enabling early detection of invalid command templates.
func newCommandExecutor(config commandExecutorConfig) (*commandExecutor, error) {
	ctx := config.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	e := &commandExecutor{
		checkCmd:          config.CheckCmd,
		reloadCmd:         config.ReloadCmd,
		minReloadInterval: config.MinReloadInterval,
		lastReloadTime:    config.LastReloadTime,
		syncOnly:          config.SyncOnly,
		ctx:               ctx,
		checkCmdTimeout:   config.CheckCmdTimeout,
		reloadCmdTimeout:  config.ReloadCmdTimeout,
		dest:              config.Dest,
	}

	// Pre-compile check command template
	if e.checkCmd != "" {
		tmpl, err := template.New("checkcmd").Parse(e.checkCmd)
		if err != nil {
			return nil, fmt.Errorf("invalid check_cmd template: %w", err)
		}
		e.checkCmdTmpl = tmpl
	}

	// Pre-compile reload command template
	if e.reloadCmd != "" {
		tmpl, err := template.New("reloadcmd").Parse(e.reloadCmd)
		if err != nil {
			return nil, fmt.Errorf("invalid reload_cmd template: %w", err)
		}
		e.reloadCmdTmpl = tmpl
	}

	return e, nil
}

// executeCheck executes the check command to validate the staged configuration.
// The command template can reference {{.src}} which is substituted with the
// staged file path.
// It returns an error if the check command fails or times out.
func (e *commandExecutor) executeCheck(stagePath string) error {
	if e.checkCmd == "" || e.syncOnly {
		return nil
	}

	start := time.Now()
	var cmdBuffer bytes.Buffer
	data := map[string]string{"src": stagePath}
	if err := e.checkCmdTmpl.Execute(&cmdBuffer, data); err != nil {
		return err
	}

	cmdErr := e.runCommandWithTimeout(cmdBuffer.String(), e.checkCmdTimeout)

	// Record metrics
	if metrics.Enabled() {
		duration := time.Since(start).Seconds()
		metrics.CommandDuration.WithLabelValues("check", e.dest).Observe(duration)
		metrics.CommandTotal.WithLabelValues("check", e.dest).Inc()
		if cmdErr != nil {
			metrics.CommandExitCodes.WithLabelValues("check", "1").Inc()
		} else {
			metrics.CommandExitCodes.WithLabelValues("check", "0").Inc()
		}
	}

	if cmdErr != nil {
		return fmt.Errorf("config check failed: %w", cmdErr)
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

	start := time.Now()
	var cmdBuffer bytes.Buffer
	data := map[string]string{"src": stagePath, "dest": destPath}
	if err := e.reloadCmdTmpl.Execute(&cmdBuffer, data); err != nil {
		return err
	}

	cmdErr := e.runCommandWithTimeout(cmdBuffer.String(), e.reloadCmdTimeout)

	// Record metrics
	if metrics.Enabled() {
		duration := time.Since(start).Seconds()
		metrics.CommandDuration.WithLabelValues("reload", e.dest).Observe(duration)
		metrics.CommandTotal.WithLabelValues("reload", e.dest).Inc()
		if cmdErr != nil {
			metrics.CommandExitCodes.WithLabelValues("reload", "1").Inc()
		} else {
			metrics.CommandExitCodes.WithLabelValues("reload", "0").Inc()
		}
	}

	if cmdErr != nil {
		return cmdErr
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
