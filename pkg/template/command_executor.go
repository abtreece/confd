package template

import (
	"bytes"
	"errors"
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
}

// commandExecutorConfig holds configuration for creating a commandExecutor.
type commandExecutorConfig struct {
	CheckCmd          string
	ReloadCmd         string
	MinReloadInterval time.Duration
	LastReloadTime    *time.Time
	SyncOnly          bool
}

// newCommandExecutor creates a new commandExecutor instance.
func newCommandExecutor(config commandExecutorConfig) *commandExecutor {
	return &commandExecutor{
		checkCmd:          config.CheckCmd,
		reloadCmd:         config.ReloadCmd,
		minReloadInterval: config.MinReloadInterval,
		lastReloadTime:    config.LastReloadTime,
		syncOnly:          config.SyncOnly,
	}
}

// executeCheck executes the check command to validate the staged configuration.
// The command template can reference {{.src}} which is substituted with the
// staged file path.
// It returns an error if the check command fails.
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
	if err := runCommand(cmdBuffer.String()); err != nil {
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

	if err := runCommand(cmdBuffer.String()); err != nil {
		return err
	}

	// Update last reload time on success
	if e.lastReloadTime != nil {
		*e.lastReloadTime = time.Now()
	}
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
