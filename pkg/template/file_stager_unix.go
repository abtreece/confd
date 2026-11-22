//go:build !windows

package template

import (
	"fmt"
	"os"
	"syscall"
)

// chownFile sets the owner and group of the named file.
func chownFile(path string, uid, gid int) error {
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("chown %q: %w", path, err)
	}
	return nil
}

// destOwnershipMatches reports whether the dest file's uid and gid match
// the expected values. Returns false if the syscall info is unavailable.
func destOwnershipMatches(stat os.FileInfo, uid, gid int) bool {
	sys, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return int(sys.Uid) == uid && int(sys.Gid) == gid
}
