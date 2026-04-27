//go:build linux

package workspace

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// cloneFile materialises src at dst via ioctl(FICLONE) — supported on btrfs,
// xfs (with reflink=1), and bcachefs. On unsupported filesystems the kernel
// returns EOPNOTSUPP/EINVAL/EXDEV and we surface errCloneUnsupported so the
// stager falls through to hardlink/copy.
func cloneFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if err := unix.IoctlFileClone(int(dstFile.Fd()), int(srcFile.Fd())); err != nil {
		_ = os.Remove(dst)
		if errors.Is(err, syscall.EOPNOTSUPP) ||
			errors.Is(err, syscall.ENOTSUP) ||
			errors.Is(err, syscall.EXDEV) ||
			errors.Is(err, syscall.EINVAL) ||
			errors.Is(err, syscall.ENOSYS) {
			return errCloneUnsupported
		}
		return err
	}
	return nil
}
