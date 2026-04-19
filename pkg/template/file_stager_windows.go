//go:build windows

package template

import "os"

// chownFile is a no-op on Windows (no uid/gid ownership concept).
func chownFile(_ string, _, _ int) error {
	return nil
}

// destOwnershipMatches is a no-op on Windows (no uid/gid ownership concept).
func destOwnershipMatches(_ os.FileInfo, _, _ int) bool {
	return true
}
