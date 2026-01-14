package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/log"
	util "github.com/abtreece/confd/pkg/util"
	"github.com/fsnotify/fsnotify"
	yaml "go.yaml.in/yaml/v3"
)

// Client provides a shell for the yaml client
type Client struct {
	filepath []string
	filter   string
}

// ResultError holds a response code and error from file watch operations.
type ResultError struct {
	response uint64
	err      error
}

// NewFileClient creates a new file backend client for reading YAML/JSON files.
func NewFileClient(filepath []string, filter string) (*Client, error) {
	return &Client{filepath: filepath, filter: filter}, nil
}

func readFile(path string, vars map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	switch filepath.Ext(path) {
	case ".json":
		fileMap := make(map[string]interface{})
		err = json.Unmarshal(data, &fileMap)
		if err != nil {
			return err
		}
		err = nodeWalk(fileMap, "/", vars)
	case "", ".yml", ".yaml":
		fileMap := make(map[string]interface{})
		err = yaml.Unmarshal(data, &fileMap)
		if err != nil {
			return err
		}
		err = nodeWalk(fileMap, "/", vars)
	default:
		err = fmt.Errorf("invalid file extension, YAML or JSON only")
	}

	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	vars := make(map[string]string)
	var filePaths []string
	for _, path := range c.filepath {
		p, err := util.RecursiveFilesLookup(path, c.filter)
		if err != nil {
			return nil, err
		}
		filePaths = append(filePaths, p...)
	}

	for _, path := range filePaths {
		err := readFile(path, vars)
		if err != nil {
			return nil, err
		}
	}

VarsLoop:
	for k, _ := range vars {
		for _, key := range keys {
			if strings.HasPrefix(k, key) {
				continue VarsLoop
			}
		}
		delete(vars, k)
	}
	log.Debug("Key Map: %#v", vars)
	return vars, nil
}

// nodeWalk recursively descends nodes, updating vars.
func nodeWalk(node interface{}, key string, vars map[string]string) error {
	switch node.(type) {
	case []interface{}:
		for i, j := range node.([]interface{}) {
			key := path.Join(key, strconv.Itoa(i))
			nodeWalk(j, key, vars)
		}
	case map[string]interface{}:
		for k, v := range node.(map[string]interface{}) {
			key := path.Join(key, k)
			nodeWalk(v, key, vars)
		}
	case string:
		vars[key] = node.(string)
	case int:
		vars[key] = strconv.Itoa(node.(int))
	case bool:
		vars[key] = strconv.FormatBool(node.(bool))
	case float64:
		vars[key] = strconv.FormatFloat(node.(float64), 'f', -1, 64)
	}
	return nil
}

func (c *Client) watchChanges(watcher *fsnotify.Watcher, stopChan chan bool) ResultError {
	// No goroutine needed - just select directly since we only need one result
	for {
		select {
		case event := <-watcher.Events:
			log.Debug("Event: %s", event)
			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Remove == fsnotify.Remove ||
				event.Op&fsnotify.Create == fsnotify.Create {
				return ResultError{response: 1, err: nil}
			}
			// Ignore other events and continue waiting
		case err := <-watcher.Errors:
			return ResultError{response: 0, err: err}
		case <-stopChan:
			return ResultError{response: 1, err: nil}
		}
	}
}

// HealthCheck verifies the backend is healthy.
// It checks that all configured files exist and are readable.
func (c *Client) HealthCheck(ctx context.Context) error {
	start := time.Now()
	logger := log.With("backend", "file", "file_count", len(c.filepath))

	for _, path := range c.filepath {
		if _, err := os.Stat(path); err != nil {
			duration := time.Since(start)
			logger.ErrorContext(ctx, "Backend health check failed",
				"duration_ms", duration.Milliseconds(),
				"failed_path", path,
				"error", err.Error())
			return fmt.Errorf("file not accessible: %s: %w", path, err)
		}
	}

	duration := time.Since(start)
	logger.InfoContext(ctx, "Backend health check passed",
		"duration_ms", duration.Milliseconds())
	return nil
}

func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	if waitIndex == 0 {
		return 1, nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return 0, err
	}
	defer watcher.Close()
	for _, path := range c.filepath {
		isDir, err := util.IsDirectory(path)
		if err != nil {
			return 0, err
		}
		if isDir {
			dirs, err := util.RecursiveDirsLookup(path, "*")
			if err != nil {
				return 0, err
			}
			for _, dir := range dirs {
				err = watcher.Add(dir)
				if err != nil {
					return 0, err
				}
			}
		} else {
			err = watcher.Add(path)
			if err != nil {
				return 0, err
			}
		}
	}
	output := c.watchChanges(watcher, stopChan)
	if output.response != 2 {
		return output.response, output.err
	}
	return waitIndex, nil
}
