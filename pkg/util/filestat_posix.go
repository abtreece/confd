//go:build !windows

package util

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

// checkOwnership compares uid/gid between source and dest stats.
// Returns (changed bool, reason string).
func checkOwnership(srcStat, destStat os.FileInfo, destPath string) (bool, string) {
	srcSys := srcStat.Sys().(*syscall.Stat_t)
	destSys := destStat.Sys().(*syscall.Stat_t)

	if destSys.Uid != srcSys.Uid {
		return true, fmt.Sprintf("%s has UID %d should be %d", destPath, destSys.Uid, srcSys.Uid)
	}
	if destSys.Gid != srcSys.Gid {
		return true, fmt.Sprintf("%s has GID %d should be %d", destPath, destSys.Gid, srcSys.Gid)
	}
	return false, ""
}

// filestat return a FileInfo describing the named file.
func FileStat(name string) (fi FileInfo, err error) {
	if IsFileExist(name) {
		f, err := os.Open(name)
		if err != nil {
			return fi, err
		}
		defer f.Close()
		stats, _ := f.Stat()
		fi.Uid = stats.Sys().(*syscall.Stat_t).Uid
		fi.Gid = stats.Sys().(*syscall.Stat_t).Gid
		fi.Mode = stats.Mode()
		h := md5.New()
		io.Copy(h, f)
		fi.Md5 = fmt.Sprintf("%x", h.Sum(nil))
		return fi, nil
	}
	return fi, errors.New("File not found")
}
