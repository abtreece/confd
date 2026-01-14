package template

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/memkv"
	"github.com/abtreece/confd/pkg/metrics"
	util "github.com/abtreece/confd/pkg/util"
)

type Config struct {
	ConfDir       string `toml:"confdir"`
	ConfigDir     string
	KeepStageFile bool
	Noop          bool   `toml:"noop"`
	Prefix        string `toml:"prefix"`
	StoreClient   backends.StoreClient
	SyncOnly      bool `toml:"sync-only"`
	TemplateDir   string
	// Diff settings for noop mode
	ShowDiff    bool
	DiffContext int
	ColorDiff   bool
	// Watch mode settings
	Debounce      time.Duration // Global debounce for all templates
	BatchInterval time.Duration // Batch processing interval
	// Context for cancellation and timeouts
	Ctx              context.Context
	BackendTimeout   time.Duration // Timeout for backend operations
	CheckCmdTimeout  time.Duration // Default timeout for check commands
	ReloadCmdTimeout time.Duration // Default timeout for reload commands
}

// TemplateResourceConfig holds the parsed template resource.
type TemplateResourceConfig struct {
	TemplateResource TemplateResource `toml:"template"`
	BackendConfig    *backends.Config `toml:"backend"`
}

// TemplateResource is the representation of a parsed template resource.
type TemplateResource struct {
	CheckCmd          string `toml:"check_cmd"`
	Dest              string
	FileMode          os.FileMode
	Gid               int
	Group             string
	Keys              []string
	Mode              string
	OutputFormat      string `toml:"output_format"`       // json, yaml, toml, xml
	MinReloadInterval string `toml:"min_reload_interval"` // e.g., "30s", "1m"
	Debounce          string `toml:"debounce"`            // e.g., "2s", "500ms"
	CheckCmdTimeout   string `toml:"check_cmd_timeout"`   // e.g., "30s", "1m"
	ReloadCmdTimeout  string `toml:"reload_cmd_timeout"`  // e.g., "60s", "2m"
	Owner             string
	Prefix            string
	ReloadCmd         string `toml:"reload_cmd"`
	Src               string
	StageFile         *os.File
	Uid               int
	funcMap           map[string]interface{}
	lastIndex         uint64
	keepStageFile     bool
	noop              bool
	store             *memkv.Store
	storeClient       backends.StoreClient
	syncOnly          bool
	templateDir       string
	// Parsed duration values
	minReloadIntervalDur time.Duration
	debounceDur          time.Duration
	lastReloadTime       time.Time
	checkCmdTimeoutDur   time.Duration
	reloadCmdTimeoutDur  time.Duration
	// Diff settings
	showDiff    bool
	diffContext int
	colorDiff   bool
	// Context for cancellation and timeouts
	ctx            context.Context
	backendTimeout time.Duration
	// Command execution
	cmdExecutor *commandExecutor
	// Format validation
	fmtValidator *formatValidator
	// Backend data fetching
	bkndFetcher *backendFetcher
	// Template rendering
	tmplRenderer *templateRenderer
	// File staging and syncing
	fileStgr *fileStager
}

var ErrEmptySrc = errors.New("empty src template")

// resolveOwnership resolves the UID and GID for the template resource.
// If Owner/Group are specified, it looks them up. Otherwise uses effective UID/GID.
func resolveOwnership(owner, group string, uid, gid int) (int, int, error) {
	// Resolve UID
	if uid == -1 {
		if owner != "" {
			u, err := user.Lookup(owner)
			if err != nil {
				return 0, 0, fmt.Errorf("cannot find owner's UID: %w", err)
			}
			uid, err = strconv.Atoi(u.Uid)
			if err != nil {
				return 0, 0, fmt.Errorf("cannot convert string to int: %w", err)
			}
		} else {
			uid = os.Geteuid()
		}
	}

	// Resolve GID
	if gid == -1 {
		if group != "" {
			g, err := user.LookupGroup(group)
			if err != nil {
				return 0, 0, fmt.Errorf("Cannot find group's GID - %s", err.Error())
			}
			gid, err = strconv.Atoi(g.Gid)
			if err != nil {
				return 0, 0, fmt.Errorf("Cannot convert string to int - %s", err.Error())
			}
		} else {
			gid = os.Getegid()
		}
	}

	return uid, gid, nil
}

// normalizePrefix concatenates global config prefix with resource prefix.
// This allows hierarchical prefixes like /production/myapp where
// "production" comes from confd.toml and "myapp" from the resource.
func normalizePrefix(globalPrefix, resourcePrefix string) string {
	if globalPrefix != "" && resourcePrefix != "" {
		return "/" + strings.Trim(globalPrefix, "/") + "/" + strings.Trim(resourcePrefix, "/")
	} else if globalPrefix != "" {
		return "/" + strings.Trim(globalPrefix, "/")
	} else if resourcePrefix != "" {
		return "/" + strings.Trim(resourcePrefix, "/")
	}
	return "/"
}

// parseDurations parses the MinReloadInterval and Debounce duration strings.
// Returns parsed durations or error if parsing fails.
func parseDurations(minReloadInterval, debounce string, globalDebounce time.Duration) (time.Duration, time.Duration, error) {
	var minReloadDur, debounceDur time.Duration

	if minReloadInterval != "" {
		d, err := time.ParseDuration(minReloadInterval)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid min_reload_interval %q: %w", minReloadInterval, err)
		}
		minReloadDur = d
	}

	if debounce != "" {
		d, err := time.ParseDuration(debounce)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid debounce %q: %w", debounce, err)
		}
		debounceDur = d
	} else if globalDebounce > 0 {
		// Use global debounce if per-resource not set
		debounceDur = globalDebounce
	}

	return minReloadDur, debounceDur, nil
}

// parseTimeoutDurations parses command timeout strings with fallback to global defaults.
// If a per-resource timeout is specified (non-empty string), it takes precedence.
// Otherwise, the global default is used.
func parseTimeoutDurations(checkCmdTimeout, reloadCmdTimeout string,
	globalCheckTimeout, globalReloadTimeout time.Duration) (time.Duration, time.Duration, error) {

	var checkDur, reloadDur time.Duration

	if checkCmdTimeout != "" {
		d, err := time.ParseDuration(checkCmdTimeout)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid check_cmd_timeout %q: %w", checkCmdTimeout, err)
		}
		checkDur = d
	} else {
		checkDur = globalCheckTimeout
	}

	if reloadCmdTimeout != "" {
		d, err := time.ParseDuration(reloadCmdTimeout)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid reload_cmd_timeout %q: %w", reloadCmdTimeout, err)
		}
		reloadDur = d
	} else {
		reloadDur = globalReloadTimeout
	}

	return checkDur, reloadDur, nil
}

// NewTemplateResource creates a TemplateResource.
func NewTemplateResource(path string, config Config) (*TemplateResource, error) {
	// Set the default uid and gid so we can determine if it was
	// unset from configuration.
	tc := &TemplateResourceConfig{TemplateResource: TemplateResource{Uid: -1, Gid: -1}}

	log.Debug("Loading template resource from %s", path)
	_, err := toml.DecodeFile(path, &tc)
	if err != nil {
		return nil, fmt.Errorf("cannot process template resource %s: %w", path, err)
	}

	tr := tc.TemplateResource
	tr.keepStageFile = config.KeepStageFile
	tr.noop = config.Noop
	tr.funcMap = newFuncMap()
	tr.store = memkv.New()
	tr.syncOnly = config.SyncOnly
	tr.showDiff = config.ShowDiff
	tr.diffContext = config.DiffContext
	tr.colorDiff = config.ColorDiff
	tr.ctx = config.Ctx
	tr.backendTimeout = config.BackendTimeout
	addFuncs(tr.funcMap, tr.store.FuncMap)

	// Determine which backend client to use:
	// 1. Per-resource backend config takes precedence
	// 2. Fall back to global StoreClient from config
	// 3. Error if neither is available
	if tc.BackendConfig != nil && tc.BackendConfig.Backend != "" {
		log.Debug("Using per-resource backend: %s", tc.BackendConfig.Backend)
		client, err := getOrCreateClient(*tc.BackendConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create backend client for %s: %w", path, err)
		}
		tr.storeClient = client
	} else if config.StoreClient != nil {
		tr.storeClient = config.StoreClient
	} else {
		return nil, errors.New("A valid StoreClient is required. Either configure a global backend or specify a [backend] section in the template resource.")
	}

	// Normalize prefix (hierarchical: global + resource)
	tr.Prefix = normalizePrefix(config.Prefix, tr.Prefix)

	if tr.Src == "" {
		return nil, ErrEmptySrc
	}

	// Resolve UID/GID from owner/group or use effective IDs
	tr.Uid, tr.Gid, err = resolveOwnership(tr.Owner, tr.Group, tr.Uid, tr.Gid)
	if err != nil {
		return nil, err
	}

	// Parse duration settings
	tr.minReloadIntervalDur, tr.debounceDur, err = parseDurations(tr.MinReloadInterval, tr.Debounce, config.Debounce)
	if err != nil {
		return nil, err
	}

	// Parse command timeout settings
	tr.checkCmdTimeoutDur, tr.reloadCmdTimeoutDur, err = parseTimeoutDurations(
		tr.CheckCmdTimeout, tr.ReloadCmdTimeout,
		config.CheckCmdTimeout, config.ReloadCmdTimeout,
	)
	if err != nil {
		return nil, err
	}

	tr.templateDir = config.TemplateDir
	tr.Src = filepath.Join(config.TemplateDir, tr.Src)

	// Initialize command executor
	tr.cmdExecutor = newCommandExecutor(commandExecutorConfig{
		CheckCmd:          tr.CheckCmd,
		ReloadCmd:         tr.ReloadCmd,
		MinReloadInterval: tr.minReloadIntervalDur,
		LastReloadTime:    &tr.lastReloadTime,
		SyncOnly:          tr.syncOnly,
		Ctx:               tr.ctx,
		CheckCmdTimeout:   tr.checkCmdTimeoutDur,
		ReloadCmdTimeout:  tr.reloadCmdTimeoutDur,
		Dest:              tr.Dest,
	})

	// Initialize format validator
	tr.fmtValidator = newFormatValidator(tr.OutputFormat)

	// Initialize backend fetcher
	tr.bkndFetcher = newBackendFetcher(backendFetcherConfig{
		StoreClient:    tr.storeClient,
		Store:          tr.store,
		Prefix:         tr.Prefix,
		Ctx:            tr.ctx,
		BackendTimeout: tr.backendTimeout,
	})

	// Initialize template renderer
	tr.tmplRenderer = newTemplateRenderer(templateRendererConfig{
		TemplateDir: tr.templateDir,
		FuncMap:     tr.funcMap,
		Store:       tr.store,
	})

	// Initialize file stager
	tr.fileStgr = newFileStager(fileStagingConfig{
		Uid:           tr.Uid,
		Gid:           tr.Gid,
		FileMode:      tr.FileMode,
		KeepStageFile: tr.keepStageFile,
		Noop:          tr.noop,
		ShowDiff:      tr.showDiff,
		DiffContext:   tr.diffContext,
		ColorDiff:     tr.colorDiff,
	})

	return &tr, nil
}

// setVars sets the Vars for template resource.
func (t *TemplateResource) setVars() error {
	return t.bkndFetcher.fetchValues(t.Keys)
}

// createStageFile stages the src configuration file by processing the src
// template and setting the desired owner, group, and mode. It also sets the
// StageFile for the template resource.
// It returns an error if any.
func (t *TemplateResource) createStageFile() error {
	// Ensure FileMode is set and fileStager is updated.
	// This defensive check is needed for backward compatibility with tests that:
	// 1. Call createStageFile() directly without going through process()
	// 2. Set FileMode directly on TemplateResource after construction
	// TODO: Refactor tests to use process() or a test helper to avoid this check
	if t.FileMode == 0 || (t.fileStgr != nil && t.fileStgr.fileMode != t.FileMode) {
		if err := t.setFileMode(); err != nil {
			return err
		}
	}

	// Render the template to bytes
	rendered, err := t.tmplRenderer.render(t.Src)
	if err != nil {
		return err
	}

	// Create stage file with rendered content
	temp, err := t.fileStgr.createStageFile(t.Dest, rendered)
	if err != nil {
		return err
	}

	// Validate output format if specified
	if err := t.fmtValidator.validate(temp.Name()); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return err
	}

	t.StageFile = temp
	return nil
}

// sync compares the staged and dest config files and attempts to sync them
// if they differ. sync will run a config check command if set before
// overwriting the target config file. Finally, sync will run a reload command
// if set to have the application or service pick up the changes.
// It returns an error if any.
func (t *TemplateResource) sync() error {
	staged := t.StageFile.Name()

	// Initialize fileStager if not already set (for backward compatibility with tests)
	if t.fileStgr == nil {
		t.fileStgr = newFileStager(fileStagingConfig{
			Uid:           t.Uid,
			Gid:           t.Gid,
			FileMode:      t.FileMode,
			KeepStageFile: t.keepStageFile,
			Noop:          t.noop,
			ShowDiff:      t.showDiff,
			DiffContext:   t.diffContext,
			ColorDiff:     t.colorDiff,
		})
	}

	// Check if config has changed
	changed, err := t.fileStgr.isConfigChanged(staged, t.Dest)
	if err != nil {
		return fmt.Errorf("failed to check config changes: %w", err)
	}

	// Handle noop mode - just show diff and return
	if t.noop {
		log.Warning("Noop mode enabled. %s will not be modified", t.Dest)
		if changed && t.showDiff {
			if err := t.fileStgr.showDiffOutput(staged, t.Dest); err != nil {
				log.Error("Failed to generate diff: %s", err.Error())
			}
		}
		// Clean up stage file in noop mode
		if !t.keepStageFile {
			os.Remove(staged)
		}
		return nil
	}

	// If no changes, clean up and return
	if !changed {
		log.Debug("Target config %s in sync", t.Dest)
		if !t.keepStageFile {
			os.Remove(staged)
		}
		return nil
	}

	// Config has changed - run check command before syncing
	log.Info("Target config %s out of sync", t.Dest)
	if !t.syncOnly && t.CheckCmd != "" {
		if err := t.check(); err != nil {
			if !t.keepStageFile {
				os.Remove(staged)
			}
			return fmt.Errorf("config check failed: %w", err)
		}
	}

	// Sync the files
	if err := t.fileStgr.syncFiles(staged, t.Dest); err != nil {
		return err
	}

	// Run reload command after successful sync
	if !t.syncOnly && t.ReloadCmd != "" {
		if err := t.reload(); err != nil {
			return err
		}
	}

	log.Info("Target config %s has been updated", t.Dest)
	return nil
}

// check executes the check command to validate the staged config file. The
// command is modified so that any references to src template are substituted
// with a string representing the full path of the staged file. This allows the
// check to be run on the staged file before overwriting the destination config
// file.
// It returns nil if the check command returns 0 and there are no other errors.
func (t *TemplateResource) check() error {
	// Initialize cmdExecutor if not already set (for backward compatibility with tests)
	if t.cmdExecutor == nil {
		t.cmdExecutor = newCommandExecutor(commandExecutorConfig{
			CheckCmd:          t.CheckCmd,
			ReloadCmd:         t.ReloadCmd,
			MinReloadInterval: t.minReloadIntervalDur,
			LastReloadTime:    &t.lastReloadTime,
			SyncOnly:          t.syncOnly,
			Ctx:               t.ctx,
			CheckCmdTimeout:   t.checkCmdTimeoutDur,
			ReloadCmdTimeout:  t.reloadCmdTimeoutDur,
			Dest:              t.Dest,
		})
	}
	return t.cmdExecutor.executeCheck(t.StageFile.Name())
}

// reload executes the reload command. The command is modified so that any
// references to src template are substituted with a string representing the
// full path of the staged file, and any references to dest are substituted
// with the full path of the destination file. This allows the reload command
// to reference the relevant file paths.
// It returns nil if the reload command returns 0.
// If min_reload_interval is set and not enough time has passed since the last
// reload, the reload is skipped and a warning is logged.
func (t *TemplateResource) reload() error {
	// Initialize cmdExecutor if not already set (for backward compatibility with tests)
	if t.cmdExecutor == nil {
		t.cmdExecutor = newCommandExecutor(commandExecutorConfig{
			CheckCmd:          t.CheckCmd,
			ReloadCmd:         t.ReloadCmd,
			MinReloadInterval: t.minReloadIntervalDur,
			LastReloadTime:    &t.lastReloadTime,
			SyncOnly:          t.syncOnly,
			Ctx:               t.ctx,
			CheckCmdTimeout:   t.checkCmdTimeoutDur,
			ReloadCmdTimeout:  t.reloadCmdTimeoutDur,
			Dest:              t.Dest,
		})
	}
	return t.cmdExecutor.executeReload(t.StageFile.Name(), t.Dest)
}


// process is a convenience function that wraps calls to the three main tasks
// required to keep local configuration files in sync. First we gather vars
// from the store, then we stage a candidate configuration file, and finally sync
// things up.
// It returns an error if any.
func (t *TemplateResource) process() error {
	start := time.Now()
	var err error
	defer func() {
		if metrics.Enabled() {
			duration := time.Since(start).Seconds()
			metrics.TemplateProcessDuration.WithLabelValues(t.Dest).Observe(duration)
			if err != nil {
				metrics.TemplateProcessTotal.WithLabelValues(t.Dest, "error").Inc()
			} else {
				metrics.TemplateProcessTotal.WithLabelValues(t.Dest, "success").Inc()
			}
		}
	}()

	if err = t.setFileMode(); err != nil {
		return err
	}
	if err = t.setVars(); err != nil {
		return err
	}
	if err = t.createStageFile(); err != nil {
		return err
	}
	if err = t.sync(); err != nil {
		return err
	}
	return nil
}

// setFileMode sets the FileMode.
func (t *TemplateResource) setFileMode() error {
	if t.Mode == "" {
		if !util.IsFileExist(t.Dest) {
			t.FileMode = 0644
		} else {
			fi, err := os.Stat(t.Dest)
			if err != nil {
				return err
			}
			t.FileMode = fi.Mode()
		}
	} else {
		mode, err := strconv.ParseUint(t.Mode, 0, 32)
		if err != nil {
			return err
		}
		t.FileMode = os.FileMode(mode)
	}

	// Update fileStager with the determined file mode
	if t.fileStgr != nil {
		t.fileStgr.updateFileMode(t.FileMode)
	}

	return nil
}
