package template

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewFileStager(t *testing.T) {
	stager := newFileStager(fileStagingConfig{
		Uid:           1000,
		Gid:           1000,
		FileMode:      0644,
		KeepStageFile: true,
		Noop:          false,
		ShowDiff:      true,
		DiffContext:   3,
		ColorDiff:     false,
	})

	if stager == nil {
		t.Fatal("newFileStager() returned nil")
	}
	if stager.uid != 1000 {
		t.Errorf("newFileStager() uid = %v, want %v", stager.uid, 1000)
	}
	if stager.gid != 1000 {
		t.Errorf("newFileStager() gid = %v, want %v", stager.gid, 1000)
	}
	if stager.fileMode != 0644 {
		t.Errorf("newFileStager() fileMode = %v, want %v", stager.fileMode, 0644)
	}
	if !stager.keepStageFile {
		t.Error("newFileStager() keepStageFile should be true")
	}
	if stager.diffContext != 3 {
		t.Errorf("newFileStager() diffContext = %v, want %v", stager.diffContext, 3)
	}
}

func TestCreateStageFile_Success(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	destPath := filepath.Join(tmpDir, "test.conf")
	content := []byte("test content")

	stager := newFileStager(fileStagingConfig{
		Uid:      os.Getuid(),
		Gid:      os.Getegid(),
		FileMode: 0644,
	})

	stageFile, err := stager.createStageFile(destPath, content)
	if err != nil {
		t.Errorf("createStageFile() unexpected error: %v", err)
	}
	if stageFile == nil {
		t.Fatal("createStageFile() returned nil file")
	}

	defer os.Remove(stageFile.Name())
	// stageFile.Close() not needed - file is already closed by createStageFile()

	// Verify content
	readContent, err := os.ReadFile(stageFile.Name())
	if err != nil {
		t.Fatalf("Failed to read stage file: %v", err)
	}
	if string(readContent) != string(content) {
		t.Errorf("createStageFile() content = %v, want %v", string(readContent), string(content))
	}

	// Verify file mode
	info, err := os.Stat(stageFile.Name())
	if err != nil {
		t.Fatalf("Failed to stat stage file: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("createStageFile() mode = %v, want %v", info.Mode().Perm(), 0644)
	}
}

func TestCreateStageFile_InvalidDestDir(t *testing.T) {
	content := []byte("test content")
	stager := newFileStager(fileStagingConfig{
		FileMode: 0644,
	})

	_, err := stager.createStageFile("/nonexistent/dir/file.conf", content)
	if err == nil {
		t.Error("createStageFile() expected error for invalid dest dir, got nil")
	}
}

func TestApplyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	tmpFile, err := os.CreateTemp("", "perm-test-")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	stager := newFileStager(fileStagingConfig{
		Uid:      os.Getuid(),
		Gid:      os.Getegid(),
		FileMode: 0600,
	})

	err = stager.applyPermissions(tmpFile.Name())
	if err != nil {
		t.Errorf("applyPermissions() unexpected error: %v", err)
	}

	// Verify mode was applied
	info, err := os.Stat(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("applyPermissions() mode = %v, want %v", info.Mode().Perm(), 0600)
	}
}

func TestIsConfigChanged_FilesIdentical(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("identical content")
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, content, 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, content, 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	stager := newFileStager(fileStagingConfig{})

	changed, err := stager.isConfigChanged(file1, file2)
	if err != nil {
		t.Errorf("isConfigChanged() unexpected error: %v", err)
	}
	if changed {
		t.Error("isConfigChanged() should return false for identical files")
	}
}

func TestIsConfigChanged_FilesDifferent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	stager := newFileStager(fileStagingConfig{})

	changed, err := stager.isConfigChanged(file1, file2)
	if err != nil {
		t.Errorf("isConfigChanged() unexpected error: %v", err)
	}
	if !changed {
		t.Error("isConfigChanged() should return true for different files")
	}
}

func TestSyncFiles_NoChanges(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("same content")
	stagePath := filepath.Join(tmpDir, "stage.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	if err := os.WriteFile(stagePath, content, 0644); err != nil {
		t.Fatalf("Failed to write stage file: %v", err)
	}
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Fatalf("Failed to write dest file: %v", err)
	}

	stager := newFileStager(fileStagingConfig{
		Uid:      os.Getuid(),
		Gid:      os.Getegid(),
		FileMode: 0644,
	})

	err = stager.syncFiles(stagePath, destPath)
	if err != nil {
		t.Errorf("syncFiles() unexpected error: %v", err)
	}

	// Stage file should be removed when no changes
	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Error("syncFiles() should remove stage file when keepStageFile=false")
	}
}

func TestSyncFiles_WithChanges(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stageContent := []byte("new content")
	destContent := []byte("old content")
	stagePath := filepath.Join(tmpDir, "stage.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	if err := os.WriteFile(stagePath, stageContent, 0644); err != nil {
		t.Fatalf("Failed to write stage file: %v", err)
	}
	if err := os.WriteFile(destPath, destContent, 0644); err != nil {
		t.Fatalf("Failed to write dest file: %v", err)
	}

	stager := newFileStager(fileStagingConfig{
		Uid:      os.Getuid(),
		Gid:      os.Getegid(),
		FileMode: 0644,
	})

	err = stager.syncFiles(stagePath, destPath)
	if err != nil {
		t.Errorf("syncFiles() unexpected error: %v", err)
	}

	// Verify dest file was updated
	updatedContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(updatedContent) != string(stageContent) {
		t.Errorf("syncFiles() dest content = %v, want %v", string(updatedContent), string(stageContent))
	}

	// Stage file should be removed
	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Error("syncFiles() should remove stage file when keepStageFile=false")
	}
}

func TestSyncFiles_ActualSync(t *testing.T) {
	// Note: syncFiles() no longer checks noop mode - that's handled by the caller
	// This test verifies syncFiles() actually syncs the files
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stageContent := []byte("new content")
	destContent := []byte("old content")
	stagePath := filepath.Join(tmpDir, "stage.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	if err := os.WriteFile(stagePath, stageContent, 0644); err != nil {
		t.Fatalf("Failed to write stage file: %v", err)
	}
	if err := os.WriteFile(destPath, destContent, 0644); err != nil {
		t.Fatalf("Failed to write dest file: %v", err)
	}

	stager := newFileStager(fileStagingConfig{
		Uid:      os.Getuid(),
		Gid:      os.Getegid(),
		FileMode: 0644,
	})

	err = stager.syncFiles(stagePath, destPath)
	if err != nil {
		t.Errorf("syncFiles() unexpected error: %v", err)
	}

	// Verify dest file WAS updated
	updatedContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(updatedContent) != string(stageContent) {
		t.Error("syncFiles() should sync files when called")
	}
}

func TestSyncFiles_KeepStageFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stageContent := []byte("new content")
	destContent := []byte("old content")
	stagePath := filepath.Join(tmpDir, "stage.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	if err := os.WriteFile(stagePath, stageContent, 0644); err != nil {
		t.Fatalf("Failed to write stage file: %v", err)
	}
	if err := os.WriteFile(destPath, destContent, 0644); err != nil {
		t.Fatalf("Failed to write dest file: %v", err)
	}

	stager := newFileStager(fileStagingConfig{
		KeepStageFile: true,
		Uid:           os.Getuid(),
		Gid:           os.Getegid(),
		FileMode:      0644,
	})

	err = stager.syncFiles(stagePath, destPath)
	if err != nil {
		t.Errorf("syncFiles() unexpected error: %v", err)
	}

	// Stage file should NOT be removed when keepStageFile=true
	if _, err := os.Stat(stagePath); os.IsNotExist(err) {
		t.Error("syncFiles() should keep stage file when keepStageFile=true")
	}
}

func TestWriteToDestination(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("test content")
	stagePath := filepath.Join(tmpDir, "stage.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	if err := os.WriteFile(stagePath, content, 0644); err != nil {
		t.Fatalf("Failed to write stage file: %v", err)
	}

	stager := newFileStager(fileStagingConfig{
		Uid:      os.Getuid(),
		Gid:      os.Getegid(),
		FileMode: 0644,
	})

	err = stager.writeToDestination(stagePath, destPath)
	if err != nil {
		t.Errorf("writeToDestination() unexpected error: %v", err)
	}

	// Verify content was written
	writtenContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(writtenContent) != string(content) {
		t.Errorf("writeToDestination() content = %v, want %v", string(writtenContent), string(content))
	}
}

func TestShowDiffOutput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("line1\nline2\nline3"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("line1\nmodified\nline3"), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	stager := newFileStager(fileStagingConfig{
		ShowDiff:    true,
		DiffContext: 1,
		ColorDiff:   false,
	})

	err = stager.showDiffOutput(file1, file2)
	if err != nil {
		t.Errorf("showDiffOutput() unexpected error: %v", err)
	}
}

func TestShowDiffOutput_NoDifference(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("same content")
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, content, 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, content, 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	stager := newFileStager(fileStagingConfig{
		ShowDiff:    true,
		DiffContext: 1,
	})

	err = stager.showDiffOutput(file1, file2)
	if err != nil {
		t.Errorf("showDiffOutput() unexpected error: %v", err)
	}
}

func TestSyncFiles_NewDestFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("new file content")
	stagePath := filepath.Join(tmpDir, "stage.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	if err := os.WriteFile(stagePath, content, 0644); err != nil {
		t.Fatalf("Failed to write stage file: %v", err)
	}

	stager := newFileStager(fileStagingConfig{
		Uid:      os.Getuid(),
		Gid:      os.Getegid(),
		FileMode: 0644,
	})

	err = stager.syncFiles(stagePath, destPath)
	if err != nil {
		t.Errorf("syncFiles() unexpected error: %v", err)
	}

	// Verify dest file was created
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(destContent) != string(content) {
		t.Errorf("syncFiles() dest content = %v, want %v", string(destContent), string(content))
	}
}

func TestCreateStageFile_EmptyContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	destPath := filepath.Join(tmpDir, "test.conf")
	content := []byte("")

	stager := newFileStager(fileStagingConfig{
		Uid:      os.Getuid(),
		Gid:      os.Getegid(),
		FileMode: 0644,
	})

	stageFile, err := stager.createStageFile(destPath, content)
	if err != nil {
		t.Errorf("createStageFile() unexpected error for empty content: %v", err)
	}
	if stageFile == nil {
		t.Fatal("createStageFile() returned nil file")
	}

	defer os.Remove(stageFile.Name())
	// stageFile.Close() not needed - file is already closed by createStageFile()

	// Verify empty content
	readContent, err := os.ReadFile(stageFile.Name())
	if err != nil {
		t.Fatalf("Failed to read stage file: %v", err)
	}
	if len(readContent) != 0 {
		t.Errorf("createStageFile() should create empty file, got content: %v", string(readContent))
	}
}

func TestSyncFiles_ErrorReading(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-stager-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	destPath := filepath.Join(tmpDir, "dest.txt")
	if err := os.WriteFile(destPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to write dest file: %v", err)
	}

	stager := newFileStager(fileStagingConfig{})

	// Try to sync from nonexistent stage file
	err = stager.syncFiles("/nonexistent/stage.txt", destPath)
	// Should get an error from isConfigChanged
	if err == nil && !strings.Contains(err.Error(), "no such file") {
		t.Error("syncFiles() should handle missing stage file")
	}
}
