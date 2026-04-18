//go:build !windows

package template

import "os"

// chownFile sets the owner and group of the named file.
func chownFile(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}
