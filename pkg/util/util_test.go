package util

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/abtreece/confd/pkg/log"
)

// createDirStructure() creates the following directory structure:
// ├── other
// │   ├── sym1.toml
// │   └── sym2.toml
// └── root
//
//	├── root.other1
//	├── root.toml
//	├── subDir1
//	│   ├── sub1.other
//	│   ├── sub1.toml
//	│   └── sub12.toml
//	├── subDir2
//	│   ├── sub2.other
//	│   ├── sub2.toml
//	│   ├── sub22.toml
//	│   └── subSubDir
//	│       ├── subsub.other
//	│       ├── subsub.toml
//	│       ├── subsub2.toml
//	│       └── sym2.toml -> ../../../other/sym2.toml
//	└── sym1.toml -> ../other/sym1.toml
func createDirStructure() (string, error) {
	mod := os.FileMode(0755)
	flag := os.O_RDWR | os.O_CREATE | os.O_EXCL
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	otherDir := filepath.Join(tmpDir, "other")
	err = os.Mkdir(otherDir, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(otherDir+"/sym1.toml", flag, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(otherDir+"/sym2.toml", flag, mod)
	if err != nil {
		return "", err
	}

	rootDir := filepath.Join(tmpDir, "root")
	err = os.Mkdir(rootDir, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(rootDir+"/root.toml", flag, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(rootDir+"/root.other1", flag, mod)
	if err != nil {
		return "", err
	}
	err = os.Symlink(otherDir+"/sym1.toml", rootDir+"/sym1.toml")
	if err != nil {
		return "", err
	}
	err = os.Symlink(otherDir, rootDir+"/other")
	if err != nil {
		return "", err
	}

	subDir := filepath.Join(rootDir, "subDir1")
	err = os.Mkdir(subDir, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(subDir+"/sub1.toml", flag, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(subDir+"/sub12.toml", flag, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(subDir+"/sub1.other", flag, mod)
	if err != nil {
		return "", err
	}
	subDir2 := filepath.Join(rootDir, "subDir2")
	err = os.Mkdir(subDir2, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(subDir2+"/sub2.toml", flag, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(subDir2+"/sub22.toml", flag, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(subDir2+"/sub2.other", flag, mod)
	if err != nil {
		return "", err
	}
	subSubDir := filepath.Join(subDir2, "subSubDir")
	err = os.Mkdir(subSubDir, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(subSubDir+"/subsub.toml", flag, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(subSubDir+"/subsub2.toml", flag, mod)
	if err != nil {
		return "", err
	}
	_, err = os.OpenFile(subSubDir+"/subsub.other", flag, mod)
	if err != nil {
		return "", err
	}
	err = os.Symlink(otherDir+"/sym2.toml", subSubDir+"/sym2.toml")
	if err != nil {
		return "", err
	}

	// tmpDir may contain symlinks itself
	actualTmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		return "", err
	}
	return actualTmpDir, nil
}

func TestRecursiveFilesLookup(t *testing.T) {
	log.SetLevel("warn")
	// Setup temporary directories
	rootDir, err := createDirStructure()
	if err != nil {
		t.Errorf("Failed to create temp dirs: %s", err.Error())
	}
	defer os.RemoveAll(rootDir)
	files, err := RecursiveFilesLookup(rootDir+"/root", "*toml")
	if err != nil {
		t.Errorf("Failed to run recursiveFindFiles, got error: %s", err.Error())
	}
	sort.Strings(files)
	expectedFiles := []string{
		rootDir + "/other/" + "sym1.toml",
		rootDir + "/other/" + "sym2.toml",
		rootDir + "/root/" + "root.toml",
		rootDir + "/root/subDir1/" + "sub1.toml",
		rootDir + "/root/subDir1/" + "sub12.toml",
		rootDir + "/root/subDir2/" + "sub2.toml",
		rootDir + "/root/subDir2/" + "sub22.toml",
		rootDir + "/root/subDir2/subSubDir/" + "subsub.toml",
		rootDir + "/root/subDir2/subSubDir/" + "subsub2.toml",
	}
	if len(files) != len(expectedFiles) {
		t.Fatalf("Did not find expected files:\nExpected:\n\t%s\nFound:\n\t%s\n",
			strings.Join(expectedFiles, "\n\t"),
			strings.Join(files, "\n\t"))
	}
	for i, f := range expectedFiles {
		if f != files[i] {
			t.Fatalf("Did not find file %s\n", f)
		}
	}
}

func TestIsConfigChangedTrue(t *testing.T) {
	log.SetLevel("warn")
	src, err := os.CreateTemp("", "src")
	defer os.Remove(src.Name())
	if err != nil {
		t.Error(err)
	}
	_, err = src.WriteString("foo")
	if err != nil {
		t.Error(err)
	}
	dest, err := os.CreateTemp("", "dest")
	defer os.Remove(dest.Name())
	if err != nil {
		t.Error(err)
	}
	_, err = dest.WriteString("foo")
	if err != nil {
		t.Error(err)
	}
	status, err := IsConfigChanged(src.Name(), dest.Name())
	if err != nil {
		t.Error(err)
	}
	if status == true {
		t.Errorf("Expected IsConfigChanged(src, dest) to be %v, got %v", true, status)
	}
}

func TestIsConfigChangedFalse(t *testing.T) {
	log.SetLevel("warn")
	src, err := os.CreateTemp("", "src")
	defer os.Remove(src.Name())
	if err != nil {
		t.Error(err)
	}
	_, err = src.WriteString("src")
	if err != nil {
		t.Error(err)
	}
	dest, err := os.CreateTemp("", "dest")
	defer os.Remove(dest.Name())
	if err != nil {
		t.Error(err)
	}
	_, err = dest.WriteString("dest")
	if err != nil {
		t.Error(err)
	}
	status, err := IsConfigChanged(src.Name(), dest.Name())
	if err != nil {
		t.Error(err)
	}
	if status == false {
		t.Errorf("Expected sameConfig(src, dest) to be %v, got %v", false, status)
	}
}

func TestNodes_String(t *testing.T) {
	tests := []struct {
		name     string
		nodes    Nodes
		expected string
	}{
		{
			name:     "empty nodes",
			nodes:    Nodes{},
			expected: "[]",
		},
		{
			name:     "single node",
			nodes:    Nodes{"http://localhost:2379"},
			expected: "[http://localhost:2379]",
		},
		{
			name:     "multiple nodes",
			nodes:    Nodes{"http://node1:2379", "http://node2:2379", "http://node3:2379"},
			expected: "[http://node1:2379 http://node2:2379 http://node3:2379]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.nodes.String()
			if result != tt.expected {
				t.Errorf("Nodes.String() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestNodes_Set(t *testing.T) {
	var nodes Nodes

	// Set first node
	err := nodes.Set("http://node1:2379")
	if err != nil {
		t.Errorf("Nodes.Set() unexpected error: %v", err)
	}
	if len(nodes) != 1 || nodes[0] != "http://node1:2379" {
		t.Errorf("Nodes.Set() nodes = %v, want [http://node1:2379]", nodes)
	}

	// Set second node
	err = nodes.Set("http://node2:2379")
	if err != nil {
		t.Errorf("Nodes.Set() unexpected error: %v", err)
	}
	if len(nodes) != 2 || nodes[1] != "http://node2:2379" {
		t.Errorf("Nodes.Set() nodes = %v, want [http://node1:2379 http://node2:2379]", nodes)
	}
}

func TestAppendPrefix(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		keys     []string
		expected []string
	}{
		{
			name:     "empty keys",
			prefix:   "/app",
			keys:     []string{},
			expected: []string{},
		},
		{
			name:     "single key",
			prefix:   "/app",
			keys:     []string{"/config"},
			expected: []string{"/app/config"},
		},
		{
			name:     "multiple keys",
			prefix:   "/production",
			keys:     []string{"/db/host", "/db/port", "/api/key"},
			expected: []string{"/production/db/host", "/production/db/port", "/production/api/key"},
		},
		{
			name:     "empty prefix",
			prefix:   "",
			keys:     []string{"/config"},
			expected: []string{"/config"},
		},
		{
			name:     "trailing slash prefix",
			prefix:   "/app/",
			keys:     []string{"config"},
			expected: []string{"/app/config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AppendPrefix(tt.prefix, tt.keys)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("AppendPrefix(%s, %v) = %v, want %v", tt.prefix, tt.keys, result, tt.expected)
			}
		})
	}
}

func TestArrayShift(t *testing.T) {
	tests := []struct {
		name     string
		array    []string
		position int
		value    string
		expected []string
	}{
		{
			name:     "insert at beginning",
			array:    []string{"b", "c"},
			position: 0,
			value:    "a",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "insert in middle",
			array:    []string{"a", "c"},
			position: 1,
			value:    "b",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "insert at end",
			array:    []string{"a", "b"},
			position: 2,
			value:    "c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "insert into empty array",
			array:    []string{},
			position: 0,
			value:    "a",
			expected: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			array := make([]string, len(tt.array))
			copy(array, tt.array)
			ArrayShift(&array, tt.position, tt.value)
			if !reflect.DeepEqual(array, tt.expected) {
				t.Errorf("ArrayShift() array = %v, want %v", array, tt.expected)
			}
		})
	}
}

func TestRecursiveDirsLookup(t *testing.T) {
	log.SetLevel("warn")
	// Setup temporary directories
	rootDir, err := createDirStructure()
	if err != nil {
		t.Fatalf("Failed to create temp dirs: %s", err.Error())
	}
	defer os.RemoveAll(rootDir)

	dirs, err := RecursiveDirsLookup(rootDir+"/root", "sub*")
	if err != nil {
		t.Fatalf("Failed to run RecursiveDirsLookup, got error: %s", err.Error())
	}

	// Should find subDir1, subDir2, and subSubDir
	if len(dirs) < 2 {
		t.Errorf("RecursiveDirsLookup() found %d dirs, expected at least 2", len(dirs))
	}

	// Check that results are directories
	for _, dir := range dirs {
		isDir, err := IsDirectory(dir)
		if err != nil {
			t.Errorf("IsDirectory(%s) error: %v", dir, err)
		}
		if !isDir {
			t.Errorf("RecursiveDirsLookup() returned non-directory: %s", dir)
		}
	}
}

func TestIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Test directory
	isDir, err := IsDirectory(tmpDir)
	if err != nil {
		t.Errorf("IsDirectory() unexpected error: %v", err)
	}
	if !isDir {
		t.Error("IsDirectory() = false for directory, want true")
	}

	// Test file
	tmpFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	isDir, err = IsDirectory(tmpFile)
	if err != nil {
		t.Errorf("IsDirectory() unexpected error: %v", err)
	}
	if isDir {
		t.Error("IsDirectory() = true for file, want false")
	}

	// Test non-existent path
	_, err = IsDirectory("/nonexistent/path/12345")
	if err == nil {
		t.Error("IsDirectory() expected error for non-existent path, got nil")
	}
}

func TestIsFileExist(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Test existing file
	if !IsFileExist(tmpFile.Name()) {
		t.Error("IsFileExist() = false for existing file, want true")
	}

	// Test non-existent file
	if IsFileExist("/nonexistent/file/12345") {
		t.Error("IsFileExist() = true for non-existent file, want false")
	}
}

func TestIsConfigChanged_DestNotExist(t *testing.T) {
	log.SetLevel("warn")
	src, err := os.CreateTemp("", "src")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	src.WriteString("content")
	src.Close()
	defer os.Remove(src.Name())

	// Destination doesn't exist - should return true (changed)
	changed, err := IsConfigChanged(src.Name(), "/nonexistent/dest/file")
	if err != nil {
		t.Errorf("IsConfigChanged() unexpected error: %v", err)
	}
	if !changed {
		t.Error("IsConfigChanged() = false when dest doesn't exist, want true")
	}
}
