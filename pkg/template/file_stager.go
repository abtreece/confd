package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abtreece/confd/pkg/log"
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
	// Create temp file in destination directory to avoid cross-filesystem issues
	temp, err := os.CreateTemp(filepath.Dir(destPath), "."+filepath.Base(destPath))
	if err != nil {
		return nil, err
	}

	// Write content to temp file
	if _, err = temp.Write(content); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return nil, err
	}

	// Apply permissions to stage file
	if err := s.applyPermissions(temp.Name()); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return nil, err
	}

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
	log.Debug("Overwriting target config %s", destPath)

	// If keepStageFile is true, we must copy instead of move
	if s.keepStageFile {
		log.Info("Keeping staged file: %s", stagePath)
		return s.writeToDestination(stagePath, destPath)
	}

	// Otherwise, try atomic rename first (moves the file)
	defer os.Remove(stagePath)

	err := os.Rename(stagePath, destPath)
	if err != nil {
		// If rename fails due to mount point, fall back to write
		if strings.Contains(err.Error(), "device or resource busy") {
			log.Debug("Rename failed - target is likely a mount. Trying to write instead")
			if err := s.writeToDestination(stagePath, destPath); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// writeToDestination writes the staged file content to the destination
// when atomic rename is not possible (e.g., mount points).
func (s *fileStager) writeToDestination(stagePath, destPath string) error {
	contents, err := os.ReadFile(stagePath)
	if err != nil {
		return err
	}

	if err := os.WriteFile(destPath, contents, s.fileMode); err != nil {
		return fmt.Errorf("failed to write to destination file: %w", err)
	}

	// Ensure owner and group match, in case the file was created with WriteFile
	if err := os.Chown(destPath, s.uid, s.gid); err != nil {
		return fmt.Errorf("failed to chown destination file: %w", err)
	}

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
