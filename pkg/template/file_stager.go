package template

import (
	"context"
	"errors"
	"fmt"
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
		if removeErr := os.Remove(temp.Name()); removeErr != nil {
			log.Error("Failed to remove temp file during cleanup: %v", removeErr)
		}
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
		if removeErr := os.Remove(temp.Name()); removeErr != nil {
			log.Error("Failed to remove temp file during cleanup: %v", removeErr)
		}
		logger.ErrorContext(context.Background(), "Failed to apply permissions",
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error())
		return nil, err
	}
	permDuration := time.Since(permStart)

	// Close the file before returning - content is flushed to disk
	// The file handle remains valid for Name() calls
	if err := temp.Close(); err != nil {
		if removeErr := os.Remove(temp.Name()); removeErr != nil {
			log.Error("Failed to remove temp file during cleanup: %v", removeErr)
		}
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
	if err := os.Chown(filePath, s.uid, s.gid); err != nil {
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
	defer os.Remove(stagePath)

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
	contents, err := os.ReadFile(stagePath)
	readDuration := time.Since(readStart)

	if err != nil {
		logger.ErrorContext(context.Background(), "Failed to read stage file",
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error())
		return err
	}

	writeStart := time.Now()
	if err := os.WriteFile(destPath, contents, s.fileMode); err != nil {
		logger.ErrorContext(context.Background(), "Failed to write to destination",
			"duration_ms", time.Since(start).Milliseconds(),
			"file_size_bytes", len(contents),
			"error", err.Error())
		return fmt.Errorf("failed to write to destination file: %w", err)
	}
	writeDuration := time.Since(writeStart)

	// Ensure owner and group match, in case the file was created with WriteFile
	chownStart := time.Now()
	if err := os.Chown(destPath, s.uid, s.gid); err != nil {
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
