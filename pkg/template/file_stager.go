package template

import (
	"context"
	"crypto/md5" // #nosec G501 -- MD5 used for change detection, not security
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/metrics"
	util "github.com/abtreece/confd/pkg/util"
)

// fileStager handles atomic file operations including staging, permission management,
// change detection, and diff generation.
type fileStager struct {
	uid           int
	gid           int
	fileMode      os.FileMode
	keepStageFile bool
	noop          bool
	showDiff      bool
	diffContext   int
	colorDiff     bool
}

// fileStagingConfig holds configuration for creating a fileStager.
type fileStagingConfig struct {
	Uid           int
	Gid           int
	FileMode      os.FileMode
	KeepStageFile bool
	Noop          bool
	ShowDiff      bool
	DiffContext   int
	ColorDiff     bool
}

// newFileStager creates a new fileStager instance.
func newFileStager(config fileStagingConfig) *fileStager {
	return &fileStager{
		uid:           config.Uid,
		gid:           config.Gid,
		fileMode:      config.FileMode,
		keepStageFile: config.KeepStageFile,
		noop:          config.Noop,
		showDiff:      config.ShowDiff,
		diffContext:   config.DiffContext,
		colorDiff:     config.ColorDiff,
	}
}

// updateFileMode updates the file mode to be applied to staged files.
// This is called after setFileMode() determines the final mode.
func (s *fileStager) updateFileMode(mode os.FileMode) {
	s.fileMode = mode
}

// md5Bytes returns the MD5 hex digest of b.
// MD5 is used for fast change detection, not cryptographic security.
func md5Bytes(b []byte) string { // #nosec G401 -- MD5 used for change detection, not security
	h := md5.New()
	h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// computeDestMD5 computes the MD5 hash of a file and returns it as a hex string.
func computeDestMD5(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- path is an operator-configured dest file
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New() // #nosec G401 -- MD5 used for change detection, not security
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// contentUnchanged reports whether rendered content is identical to the dest file,
// including file mode and ownership. Returns false (must write) when dest does not exist.
func (s *fileStager) contentUnchanged(rendered []byte, destPath string) (bool, error) {
	destStat, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Mode mismatch — permissions need updating.
	if destStat.Mode().Perm() != s.fileMode {
		return false, nil
	}

	// Ownership mismatch (platform-specific; always true on Windows).
	if !destOwnershipMatches(destStat, s.uid, s.gid) {
		return false, nil
	}

	// Fast path: size differs means content differs.
	if destStat.Size() != int64(len(rendered)) {
		return false, nil
	}

	// Compare content via MD5.
	destMD5, err := computeDestMD5(destPath)
	if err != nil {
		return false, err
	}

	return md5Bytes(rendered) == destMD5, nil
}

// createStageFile creates a temporary stage file in the destination directory
// with the provided content and applies the configured permissions.
// Returns the created file or an error.
func (s *fileStager) createStageFile(destPath string, content []byte) (*os.File, error) {
	start := time.Now()
	logger := log.With("dest_path", destPath, "content_size_bytes", len(content))
	logger.DebugContext(context.Background(), "Creating stage file")

	// Create temp file in destination directory to avoid cross-filesystem issues
	temp, err := os.CreateTemp(filepath.Dir(destPath), "."+filepath.Base(destPath))
	if err != nil {
		logger.ErrorContext(context.Background(), "Failed to create temp file",
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error())
		return nil, err
	}

	// Write content to temp file
	writeStart := time.Now()
	if _, err = temp.Write(content); err != nil {
		if closeErr := temp.Close(); closeErr != nil {
			log.Error("Failed to close temp file during cleanup: %v", closeErr)
		}
		removeStageFile(temp.Name())
		logger.ErrorContext(context.Background(), "Failed to write to stage file",
			"duration_ms", time.Since(start).Milliseconds(),
			"write_duration_ms", time.Since(writeStart).Milliseconds(),
			"error", err.Error())
		return nil, err
	}
	writeDuration := time.Since(writeStart)

	// Apply permissions to stage file
	permStart := time.Now()
	if err := s.applyPermissions(temp.Name()); err != nil {
		if closeErr := temp.Close(); closeErr != nil {
			log.Error("Failed to close temp file during cleanup: %v", closeErr)
		}
		removeStageFile(temp.Name())
		logger.ErrorContext(context.Background(), "Failed to apply permissions",
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error())
		return nil, err
	}
	permDuration := time.Since(permStart)

	// Close the file before returning - content is flushed to disk
	// The file handle remains valid for Name() calls
	if err := temp.Close(); err != nil {
		removeStageFile(temp.Name())
		return nil, fmt.Errorf("failed to close stage file: %w", err)
	}

	logger.InfoContext(context.Background(), "Stage file created successfully",
		"stage_path", temp.Name(),
		"total_duration_ms", time.Since(start).Milliseconds(),
		"write_duration_ms", writeDuration.Milliseconds(),
		"permission_duration_ms", permDuration.Milliseconds())

	return temp, nil
}

// applyPermissions sets the owner, group, and mode on the specified file.
func (s *fileStager) applyPermissions(filePath string) error {
	if err := os.Chmod(filePath, s.fileMode); err != nil {
		return fmt.Errorf("failed to chmod stage file: %w", err)
	}
	if err := chownFile(filePath, s.uid, s.gid); err != nil {
		return fmt.Errorf("failed to chown stage file: %w", err)
	}
	return nil
}

// isConfigChanged compares the staged file with the destination file
// to determine if they differ.
func (s *fileStager) isConfigChanged(stagePath, destPath string) (bool, error) {
	return util.IsConfigChanged(stagePath, destPath)
}

// syncFiles atomically replaces the destination file with the staged file.
// It handles mount points by falling back to write operations if rename fails.
// If keepStageFile is true, it copies instead of moving the stage file.
// Returns an error if the sync operation fails.
// Note: This method assumes the caller has already checked if files differ
// and handled noop mode. It only performs the actual file sync operation.
func (s *fileStager) syncFiles(stagePath, destPath string) error {
	start := time.Now()
	logger := log.With("stage_path", stagePath, "dest_path", destPath)
	logger.DebugContext(context.Background(), "Starting file sync")

	// Record file sync metrics
	if metrics.Enabled() {
		metrics.FileSyncTotal.WithLabelValues(destPath).Inc()
		metrics.FileChangedTotal.Inc()
	}

	// If keepStageFile is true, we must copy instead of move
	if s.keepStageFile {
		logger.InfoContext(context.Background(), "Keeping staged file, using copy mode")
		if err := s.writeToDestination(stagePath, destPath); err != nil {
			return err
		}
		logger.InfoContext(context.Background(), "File sync completed (copy mode)",
			"duration_ms", time.Since(start).Milliseconds())
		return nil
	}

	// Otherwise, try atomic rename first (moves the file)
	defer removeStageFile(stagePath)

	renameStart := time.Now()
	err := os.Rename(stagePath, destPath)
	renameDuration := time.Since(renameStart)

	if err != nil {
		// If rename fails due to cross-filesystem or mount point, fall back to write
		// EXDEV: cross-device link (more reliable cross-platform detection)
		if errors.Is(err, syscall.EXDEV) || strings.Contains(err.Error(), "device or resource busy") {
			logger.DebugContext(context.Background(), "Rename failed, falling back to write",
				"rename_duration_ms", renameDuration.Milliseconds())

			writeStart := time.Now()
			if err := s.writeToDestination(stagePath, destPath); err != nil {
				logger.ErrorContext(context.Background(), "File sync failed",
					"duration_ms", time.Since(start).Milliseconds(),
					"error", err.Error())
				return err
			}
			writeDuration := time.Since(writeStart)

			logger.InfoContext(context.Background(), "File sync completed (write fallback)",
				"total_duration_ms", time.Since(start).Milliseconds(),
				"write_duration_ms", writeDuration.Milliseconds())
		} else {
			logger.ErrorContext(context.Background(), "File sync failed",
				"duration_ms", time.Since(start).Milliseconds(),
				"error", err.Error())
			return err
		}
	} else {
		logger.InfoContext(context.Background(), "File sync completed (atomic rename)",
			"duration_ms", time.Since(start).Milliseconds(),
			"rename_duration_ms", renameDuration.Milliseconds())
	}

	return nil
}

// writeToDestination writes the staged file content to the destination
// when atomic rename is not possible (e.g., mount points).
func (s *fileStager) writeToDestination(stagePath, destPath string) error {
	start := time.Now()
	logger := log.With("stage_path", stagePath, "dest_path", destPath)

	readStart := time.Now()
	contents, err := os.ReadFile(stagePath) // #nosec G304 -- stagePath is an internal temp file created by confd
	readDuration := time.Since(readStart)

	if err != nil {
		logger.ErrorContext(context.Background(), "Failed to read stage file",
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error())
		return err
	}

	writeStart := time.Now()
	if err := os.WriteFile(destPath, contents, s.fileMode); err != nil { // #nosec G703 -- destPath is operator-configured, not user input
		logger.ErrorContext(context.Background(), "Failed to write to destination",
			"duration_ms", time.Since(start).Milliseconds(),
			"file_size_bytes", len(contents),
			"error", err.Error())
		return fmt.Errorf("failed to write to destination file: %w", err)
	}
	writeDuration := time.Since(writeStart)

	// Ensure owner and group match, in case the file was created with WriteFile
	chownStart := time.Now()
	if err := chownFile(destPath, s.uid, s.gid); err != nil {
		logger.ErrorContext(context.Background(), "Failed to chown destination",
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error())
		return fmt.Errorf("failed to chown destination file: %w", err)
	}
	chownDuration := time.Since(chownStart)

	logger.InfoContext(context.Background(), "Write to destination completed",
		"total_duration_ms", time.Since(start).Milliseconds(),
		"read_duration_ms", readDuration.Milliseconds(),
		"write_duration_ms", writeDuration.Milliseconds(),
		"chown_duration_ms", chownDuration.Milliseconds(),
		"file_size_bytes", len(contents))

	return nil
}

// removeStageFile removes a staged temp file. Cleanup failures are non-fatal
// but logged and counted — they indicate filesystem issues worth observing.
func removeStageFile(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Warning("Failed to remove stage file %s: %v", path, err)
		if metrics.Enabled() {
			metrics.StageFileCleanupErrors.Inc()
		}
	}
}

// showDiffOutput generates and displays a diff between the staged and destination files.
func (s *fileStager) showDiffOutput(stagePath, destPath string) error {
	diff, err := util.GenerateDiff(stagePath, destPath, s.diffContext)
	if err != nil {
		return err
	}

	if diff == "" {
		return nil
	}

	if s.colorDiff {
		diff = util.ColorizeDiff(diff)
	}

	fmt.Print(diff)
	return nil
}
