//go:build !windows

package template

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

// resolveOwnership resolves the UID and GID for the template resource.
// If Owner/Group are specified, it looks them up. Otherwise uses effective UID/GID.
func resolveOwnership(owner, group string, uid, gid int) (int, int, error) {
	// Resolve UID
	if uid == -1 {
		if owner != "" {
			u, err := user.Lookup(owner)
			if err != nil {
				return 0, 0, fmt.Errorf("cannot find owner's UID: %w", err)
			}
			uid, err = strconv.Atoi(u.Uid)
			if err != nil {
				return 0, 0, fmt.Errorf("cannot convert string to int: %w", err)
			}
		} else {
			uid = os.Geteuid()
		}
	}

	// Resolve GID
	if gid == -1 {
		if group != "" {
			g, err := user.LookupGroup(group)
			if err != nil {
				return 0, 0, fmt.Errorf("cannot find group's GID for group %q: %w", group, err)
			}
			gid, err = strconv.Atoi(g.Gid)
			if err != nil {
				return 0, 0, fmt.Errorf("cannot convert group GID %q to int for group %q: %w", g.Gid, group, err)
			}
		} else {
			gid = os.Getegid()
		}
	}

	return uid, gid, nil
}
