//go:build windows

package template

import (
	"os"

	"github.com/abtreece/confd/pkg/log"
)

// resolveOwnership on Windows ignores owner/group fields — Windows does not use
// Unix-style numeric UIDs/GIDs, and file ownership is managed differently.
// chownFile is a no-op on Windows, so these values are never applied.
func resolveOwnership(owner, group string, uid, gid int) (int, int, error) {
	if owner != "" || group != "" {
		log.Warning("owner/group settings are not supported on Windows and will be ignored")
	}
	if uid == -1 {
		uid = os.Getuid()
	}
	if gid == -1 {
		gid = os.Getegid()
	}
	return uid, gid, nil
}
