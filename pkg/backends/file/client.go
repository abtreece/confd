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

	"github.com/abtreece/confd/pkg/backends/types"
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
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	switch filepath.Ext(path) {
	case ".json":
		fileMap := make(map[string]interface{})
		err = json.Unmarshal(data, &fileMap)
		if err != nil {
			return fmt.Errorf("failed to unmarshal JSON file %s: %w", path, err)
		}
		err = nodeWalk(fileMap, "/", vars)
	case "", ".yml", ".yaml":
		fileMap := make(map[string]interface{})
		err = yaml.Unmarshal(data, &fileMap)
		if err != nil {
			return fmt.Errorf("failed to unmarshal YAML file %s: %w", path, err)
		}
		err = nodeWalk(fileMap, "/", vars)
	default:
		err = fmt.Errorf("invalid file extension, YAML or JSON only")
	}

	if err != nil {
		return fmt.Errorf("failed to process file %s: %w", path, err)
	}
	return nil
}

// matchesAnyPrefix returns true if path has any of the given prefixes.
// Returns false if prefixes is empty.
func matchesAnyPrefix(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	vars := make(map[string]string)
	var filePaths []string
	for _, path := range c.filepath {
		p, err := util.RecursiveFilesLookup(path, c.filter)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup files in %s: %w", path, err)
		}
		filePaths = append(filePaths, p...)
	}

	for _, path := range filePaths {
		err := readFile(path, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration file: %w", err)
		}
	}

for k := range vars {
		if !matchesAnyPrefix(k, keys) {
			delete(vars, k)
		}
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

func (c *Client) watchChanges(ctx context.Context, watcher *fsnotify.Watcher, stopChan chan bool) ResultError {
	// No goroutine needed - just select directly since we only need one result
	for {
		select {
		case <-ctx.Done():
			return ResultError{response: 1, err: nil}
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
		fileInfo, err := os.Stat(path)
		if err != nil {
			duration := time.Since(start)
			logger.ErrorContext(ctx, "Backend health check failed",
				"duration_ms", duration.Milliseconds(),
				"failed_path", path,
				"error", err.Error())
			return fmt.Errorf("file not accessible: %s: %w", path, err)
		}

		// If it's a file (not a directory), verify read permissions by attempting to open it
		if !fileInfo.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				duration := time.Since(start)
				logger.ErrorContext(ctx, "Backend health check failed",
					"duration_ms", duration.Milliseconds(),
					"failed_path", path,
					"error", err.Error())
				return fmt.Errorf("file not readable: %s: %w", path, err)
			}
			f.Close()
		}
	}

	duration := time.Since(start)
	logger.InfoContext(ctx, "Backend health check passed",
		"duration_ms", duration.Milliseconds())
	return nil
}

// HealthCheckDetailed provides detailed health information for the file backend.
func (c *Client) HealthCheckDetailed(ctx context.Context) (*types.HealthResult, error) {
	start := time.Now()

	var totalSize int64
	var fileCount int
	pathList := make([]string, 0, len(c.filepath))

	for _, path := range c.filepath {
		fileInfo, err := os.Stat(path)
		if err != nil {
			duration := time.Since(start)
			return &types.HealthResult{
				Healthy:   false,
				Message:   fmt.Sprintf("file not accessible: %s", path),
				Duration:  types.DurationMillis(duration),
				CheckedAt: time.Now(),
				Details: map[string]string{
					"failed_path": path,
					"error":       err.Error(),
				},
			}, err
		}

		pathList = append(pathList, path)

		// If it's a file (not a directory), add its size and verify read permissions
		if !fileInfo.IsDir() {
			totalSize += fileInfo.Size()
			fileCount++

			f, err := os.Open(path)
			if err != nil {
				duration := time.Since(start)
				return &types.HealthResult{
					Healthy:   false,
					Message:   fmt.Sprintf("file not readable: %s", path),
					Duration:  types.DurationMillis(duration),
					CheckedAt: time.Now(),
					Details: map[string]string{
						"failed_path": path,
						"error":       err.Error(),
					},
				}, err
			}
			f.Close()
		}
	}

	duration := time.Since(start)

	return &types.HealthResult{
		Healthy:   true,
		Message:   "All configured files are accessible and readable",
		Duration:  types.DurationMillis(duration),
		CheckedAt: time.Now(),
		Details: map[string]string{
			"file_count":       fmt.Sprintf("%d", fileCount),
			"total_size_bytes": fmt.Sprintf("%d", totalSize),
			"paths":            strings.Join(pathList, ", "),
		},
	}, nil
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
	output := c.watchChanges(ctx, watcher, stopChan)
	if output.response != 2 {
		return output.response, output.err
	}
	return waitIndex, nil
}
