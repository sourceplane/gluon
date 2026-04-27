//go:build darwin

package workspace

import (
	"errors"
	"syscall"

	"golang.org/x/sys/unix"
)

// cloneFile materialises src at dst via APFS clonefile(2). On non-APFS
// volumes (or across volumes) the syscall returns ENOTSUP/EXDEV and we fall
// through to the next strategy.
func cloneFile(src, dst string) error {
	err := unix.Clonefile(src, dst, 0)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EXDEV) ||
		errors.Is(err, syscall.EPERM) ||
		errors.Is(err, syscall.ENOSYS) {
		return errCloneUnsupported
	}
	return err
}
