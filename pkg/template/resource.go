package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/memkv"
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
	// Diff settings
	showDiff    bool
	diffContext int
	colorDiff   bool
}

var ErrEmptySrc = errors.New("empty src template")

// NewTemplateResource creates a TemplateResource.
func NewTemplateResource(path string, config Config) (*TemplateResource, error) {
	// Set the default uid and gid so we can determine if it was
	// unset from configuration.
	tc := &TemplateResourceConfig{TemplateResource: TemplateResource{Uid: -1, Gid: -1}}

	log.Debug("Loading template resource from %s", path)
	_, err := toml.DecodeFile(path, &tc)
	if err != nil {
		return nil, fmt.Errorf("Cannot process template resource %s - %s", path, err.Error())
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
	addFuncs(tr.funcMap, tr.store.FuncMap)

	// Determine which backend client to use:
	// 1. Per-resource backend config takes precedence
	// 2. Fall back to global StoreClient from config
	// 3. Error if neither is available
	if tc.BackendConfig != nil && tc.BackendConfig.Backend != "" {
		log.Debug("Using per-resource backend: %s", tc.BackendConfig.Backend)
		client, err := getOrCreateClient(*tc.BackendConfig)
		if err != nil {
			return nil, fmt.Errorf("Cannot create backend client for %s - %s", path, err.Error())
		}
		tr.storeClient = client
	} else if config.StoreClient != nil {
		tr.storeClient = config.StoreClient
	} else {
		return nil, errors.New("A valid StoreClient is required. Either configure a global backend or specify a [backend] section in the template resource.")
	}

	// Concatenate global config prefix with resource prefix.
	// This allows hierarchical prefixes like /production/myapp where
	// "production" comes from confd.toml and "myapp" from the resource.
	if config.Prefix != "" && tr.Prefix != "" {
		tr.Prefix = "/" + strings.Trim(config.Prefix, "/") + "/" + strings.Trim(tr.Prefix, "/")
	} else if config.Prefix != "" {
		tr.Prefix = "/" + strings.Trim(config.Prefix, "/")
	} else if tr.Prefix != "" {
		tr.Prefix = "/" + strings.Trim(tr.Prefix, "/")
	} else {
		tr.Prefix = "/"
	}

	if tr.Src == "" {
		return nil, ErrEmptySrc
	}

	if tr.Uid == -1 {
		if tr.Owner != "" {
			u, err := user.Lookup(tr.Owner)
			if err != nil {
				return nil, fmt.Errorf("Cannot find owner's UID - %s", err.Error())
			}
			tr.Uid, err = strconv.Atoi(u.Uid)
			if err != nil {
				return nil, fmt.Errorf("Cannot convert string to int - %s", err.Error())
			}
		} else {
			tr.Uid = os.Geteuid()
		}
	}

	if tr.Gid == -1 {
		if tr.Group != "" {
			g, err := user.LookupGroup(tr.Group)
			if err != nil {
				return nil, fmt.Errorf("Cannot find group's GID - %s", err.Error())
			}
			tr.Gid, err = strconv.Atoi(g.Gid)
			if err != nil {
				return nil, fmt.Errorf("Cannot convert string to int - %s", err.Error())
			}
		} else {
			tr.Gid = os.Getegid()
		}
	}

	// Parse duration settings
	if tr.MinReloadInterval != "" {
		d, err := time.ParseDuration(tr.MinReloadInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid min_reload_interval %q: %w", tr.MinReloadInterval, err)
		}
		tr.minReloadIntervalDur = d
	}
	if tr.Debounce != "" {
		d, err := time.ParseDuration(tr.Debounce)
		if err != nil {
			return nil, fmt.Errorf("invalid debounce %q: %w", tr.Debounce, err)
		}
		tr.debounceDur = d
	} else if config.Debounce > 0 {
		// Use global debounce if per-resource not set
		tr.debounceDur = config.Debounce
	}

	tr.templateDir = config.TemplateDir
	tr.Src = filepath.Join(config.TemplateDir, tr.Src)
	return &tr, nil
}

// setVars sets the Vars for template resource.
func (t *TemplateResource) setVars() error {
	var err error
	log.Debug("Retrieving keys from store")
	log.Debug("Key prefix set to %s", t.Prefix)

	result, err := t.storeClient.GetValues(context.Background(), util.AppendPrefix(t.Prefix, t.Keys))
	if err != nil {
		return err
	}
	log.Debug("Got the following map from store: %v", result)

	t.store.Purge()

	for k, v := range result {
		t.store.Set(path.Join("/", strings.TrimPrefix(k, t.Prefix)), v)
	}
	return nil
}

// createStageFile stages the src configuration file by processing the src
// template and setting the desired owner, group, and mode. It also sets the
// StageFile for the template resource.
// It returns an error if any.
func (t *TemplateResource) createStageFile() error {
	log.Debug("Using source template %s", t.Src)

	if !util.IsFileExist(t.Src) {
		return errors.New("Missing template: " + t.Src)
	}

	log.Debug("Compiling source template %s", t.Src)

	// Add include function to funcMap for this template
	includeCtx := NewIncludeContext()
	t.funcMap["include"] = NewIncludeFunc(t.templateDir, t.funcMap, includeCtx)

	// Try to get template from cache
	var tmpl *template.Template
	var err error
	tmpl, cacheHit := GetCachedTemplate(t.Src)
	if !cacheHit {
		log.Debug("Template cache miss for %s", t.Src)
		stat, statErr := os.Stat(t.Src)
		if statErr != nil {
			return fmt.Errorf("Unable to stat template %s: %w", t.Src, statErr)
		}
		tmpl, err = template.New(filepath.Base(t.Src)).Funcs(t.funcMap).ParseFiles(t.Src)
		if err != nil {
			return fmt.Errorf("Unable to process template %s, %s", t.Src, err)
		}
		PutCachedTemplate(t.Src, tmpl, stat.ModTime())
	} else {
		log.Debug("Template cache hit for %s", t.Src)
		// Update funcMap with fresh include function (functions resolved at execution time)
		tmpl = tmpl.Funcs(t.funcMap)
	}

	// create TempFile in Dest directory to avoid cross-filesystem issues
	temp, err := os.CreateTemp(filepath.Dir(t.Dest), "."+filepath.Base(t.Dest))
	if err != nil {
		return err
	}

	if err = tmpl.Execute(temp, nil); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return err
	}

	// Validate output format if specified
	if t.OutputFormat != "" {
		// Read the rendered content for validation
		if _, err := temp.Seek(0, 0); err != nil {
			temp.Close()
			os.Remove(temp.Name())
			return fmt.Errorf("failed to seek staged file: %w", err)
		}
		content, err := os.ReadFile(temp.Name())
		if err != nil {
			temp.Close()
			os.Remove(temp.Name())
			return fmt.Errorf("failed to read staged file for validation: %w", err)
		}
		if err := util.ValidateFormat(content, t.OutputFormat); err != nil {
			temp.Close()
			os.Remove(temp.Name())
			return fmt.Errorf("output format validation failed (%s): %w", t.OutputFormat, err)
		}
		log.Debug("Output format validation passed (%s)", t.OutputFormat)
	}

	defer temp.Close()

	// Set the owner, group, and mode on the stage file now to make it easier to
	// compare against the destination configuration file later.
	os.Chmod(temp.Name(), t.FileMode)
	os.Chown(temp.Name(), t.Uid, t.Gid)
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
	if t.keepStageFile {
		log.Info("Keeping staged file: %s", staged)
	} else {
		defer os.Remove(staged)
	}

	log.Debug("Comparing candidate config to %s", t.Dest)
	ok, err := util.IsConfigChanged(staged, t.Dest)
	if err != nil {
		log.Error("%s", err.Error())
	}
	if t.noop {
		log.Warning("Noop mode enabled. %s will not be modified", t.Dest)
		if ok && t.showDiff {
			diff, diffErr := util.GenerateDiff(staged, t.Dest, t.diffContext)
			if diffErr != nil {
				log.Error("Failed to generate diff: %s", diffErr.Error())
			} else if diff != "" {
				if t.colorDiff {
					diff = util.ColorizeDiff(diff)
				}
				fmt.Print(diff)
			}
		}
		return nil
	}
	if ok {
		log.Info("Target config %s out of sync", t.Dest)
		if !t.syncOnly && t.CheckCmd != "" {
			if err := t.check(); err != nil {
				return errors.New("Config check failed: " + err.Error())
			}
		}
		log.Debug("Overwriting target config %s", t.Dest)
		err := os.Rename(staged, t.Dest)
		if err != nil {
			if strings.Contains(err.Error(), "device or resource busy") {
				log.Debug("Rename failed - target is likely a mount. Trying to write instead")
				// try to open the file and write to it
				var contents []byte
				var rerr error
				contents, rerr = os.ReadFile(staged)
				if rerr != nil {
					return rerr
				}
				err := os.WriteFile(t.Dest, contents, t.FileMode)
				// make sure owner and group match the temp file, in case the file was created with WriteFile
				os.Chown(t.Dest, t.Uid, t.Gid)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if !t.syncOnly && t.ReloadCmd != "" {
			if err := t.reload(); err != nil {
				return err
			}
		}
		log.Info("Target config %s has been updated", t.Dest)
	} else {
		log.Debug("Target config %s in sync", t.Dest)
	}
	return nil
}

// check executes the check command to validate the staged config file. The
// command is modified so that any references to src template are substituted
// with a string representing the full path of the staged file. This allows the
// check to be run on the staged file before overwriting the destination config
// file.
// It returns nil if the check command returns 0 and there are no other errors.
func (t *TemplateResource) check() error {
	var cmdBuffer bytes.Buffer
	data := make(map[string]string)
	data["src"] = t.StageFile.Name()
	tmpl, err := template.New("checkcmd").Parse(t.CheckCmd)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(&cmdBuffer, data); err != nil {
		return err
	}
	return runCommand(cmdBuffer.String())
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
	// Check rate limiting
	if t.minReloadIntervalDur > 0 && !t.lastReloadTime.IsZero() {
		elapsed := time.Since(t.lastReloadTime)
		if elapsed < t.minReloadIntervalDur {
			remaining := t.minReloadIntervalDur - elapsed
			log.Warning("Reload throttled for %s (next allowed in %v)", t.Dest, remaining.Round(time.Second))
			return nil
		}
	}

	var cmdBuffer bytes.Buffer
	data := make(map[string]string)
	data["src"] = t.StageFile.Name()
	data["dest"] = t.Dest
	tmpl, err := template.New("reloadcmd").Parse(t.ReloadCmd)
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
	t.lastReloadTime = time.Now()
	return nil
}

// runCommand is a shared function used by check and reload
// to run the given command and log its output.
// It returns nil if the given cmd returns 0.
// The command can be run on unix and windows.
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

// process is a convenience function that wraps calls to the three main tasks
// required to keep local configuration files in sync. First we gather vars
// from the store, then we stage a candidate configuration file, and finally sync
// things up.
// It returns an error if any.
func (t *TemplateResource) process() error {
	if err := t.setFileMode(); err != nil {
		return err
	}
	if err := t.setVars(); err != nil {
		return err
	}
	if err := t.createStageFile(); err != nil {
		return err
	}
	if err := t.sync(); err != nil {
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
	return nil
}
