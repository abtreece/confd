package util

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/abtreece/confd/pkg/log"
)

// Nodes is a custom flag Var representing a list of etcd nodes.
type Nodes []string

// String returns the string representation of a node var.
func (n *Nodes) String() string {
	return fmt.Sprintf("%s", *n)
}

// Set appends the node to the etcd node list.
func (n *Nodes) Set(node string) error {
	*n = append(*n, node)
	return nil
}

// fileInfo describes a configuration file and is returned by fileStat.
type FileInfo struct {
	Uid  uint32
	Gid  uint32
	Mode os.FileMode
	Md5  string
}

// AppendPrefix prepends the given prefix to each key in the slice.
func AppendPrefix(prefix string, keys []string) []string {
	s := make([]string, len(keys))
	for i, k := range keys {
		s[i] = path.Join(prefix, k)
	}
	return s
}

// ArrayShift inserts a value at the specified position in the array.
func ArrayShift(array *[]string, position int, value string) {
	*array = append(*array, "")
	copy((*array)[position+1:], (*array)[position:])
	(*array)[position] = value
}

// isFileExist reports whether path exits.
func IsFileExist(fpath string) bool {
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		return false
	}
	return true
}

// IsConfigChanged reports whether src and dest config files are equal.
// Two config files are equal when they have the same file contents and
// Unix permissions. The owner, group, and mode must match.
// It returns false in other cases.
//
// This function is optimized to minimize syscalls by short-circuiting on
// metadata differences (size, mode, uid/gid) before computing MD5 hashes.
func IsConfigChanged(src, dest string) (bool, error) {
	// Single stat for dest, handle not-exist case
	destStat, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// Single stat for src
	srcStat, err := os.Stat(src)
	if err != nil {
		return false, err
	}

	// Short-circuit: size differs = content differs
	if destStat.Size() != srcStat.Size() {
		log.Info("%s has size %d should be %d", dest, destStat.Size(), srcStat.Size())
		return true, nil
	}

	// Short-circuit: mode differs
	if destStat.Mode() != srcStat.Mode() {
		log.Info("%s has mode %s should be %s", dest, destStat.Mode(), srcStat.Mode())
		return true, nil
	}

	// Platform-specific: check uid/gid (POSIX only, no-op on Windows)
	if changed, reason := checkOwnership(srcStat, destStat, dest); changed {
		log.Info("%s", reason)
		return true, nil
	}

	// Only compute MD5 if all metadata matches
	srcMD5, err := computeMD5(src)
	if err != nil {
		return false, err
	}
	destMD5, err := computeMD5(dest)
	if err != nil {
		return false, err
	}

	if srcMD5 != destMD5 {
		log.Info("%s has md5sum %s should be %s", dest, destMD5, srcMD5)
		return true, nil
	}

	return false, nil
}

// computeMD5 computes the MD5 hash of a file and returns it as a hex string.
// Note: MD5 is used here for fast change detection in non-adversarial scenarios,
// not for cryptographic security. This is appropriate for detecting accidental
// file modifications where collision resistance is not required.
func computeMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func IsDirectory(path string) (bool, error) {
	f, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	switch mode := f.Mode(); {
	case mode.IsDir():
		return true, nil
	case mode.IsRegular():
		return false, nil
	}
	return false, nil
}

func RecursiveFilesLookup(root string, pattern string) ([]string, error) {
	return recursiveLookup(root, pattern, false)
}

func RecursiveDirsLookup(root string, pattern string) ([]string, error) {
	return recursiveLookup(root, pattern, true)
}

func recursiveLookup(root string, pattern string, dirsLookup bool) ([]string, error) {
	var result []string

	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	isDir, err := IsDirectory(root)
	if err != nil {
		return nil, err
	}
	if isDir {
		err := filepath.Walk(root, func(root string, f os.FileInfo, err error) error {
			match, err := filepath.Match(pattern, f.Name())
			if err != nil {
				return err
			}
			if match {
				root, err := filepath.EvalSymlinks(root)
				if err != nil {
					return err
				}
				isDir, err := IsDirectory(root)
				if err != nil {
					return err
				}
				if isDir && dirsLookup {
					result = append(result, root)
				} else if !isDir && !dirsLookup {
					result = append(result, root)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		if !dirsLookup {
			result = append(result, root)
		}
	}
	return result, nil
}
