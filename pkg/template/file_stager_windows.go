//go:build windows

package template

// chownFile is a no-op on Windows (no uid/gid ownership concept).
func chownFile(_ string, _, _ int) error {
	return nil
}
