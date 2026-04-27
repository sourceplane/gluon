//go:build !darwin && !linux

package workspace

// cloneFile is a no-op on platforms without a known copy-on-write primitive.
// The stager falls through to hardlinking and finally to copying.
func cloneFile(src, dst string) error {
	return errCloneUnsupported
}
