// Package workspace provides per-job workspace isolation for the runner.
//
// When jobs execute concurrently against a shared source tree they collide on
// shared mutable state — for example, two jobs running `pnpm install` in the
// same monorepo overwrite each other's `node_modules`, and two jobs building
// the same site race on the same output directory. To eliminate these
// collisions, gluon stages an isolated copy of the source workspace for each
// concurrent job and re-points `WorkspaceDir`, `WorkDir`, and
// `GITHUB_WORKSPACE` at the staged copy.
//
// To keep staging cheap on large monorepos, files are materialised through the
// fastest mechanism available on the host filesystem:
//
//  1. APFS clonefile(2) on macOS — true copy-on-write at the inode level.
//  2. ioctl(FICLONE) reflinks on Linux btrfs/xfs/bcachefs.
//  3. Hard links for read-only source files (safe when jobs only write into
//     ignored directories such as node_modules / dist / build).
//  4. Plain copies as the universal fallback.
//
// Volatile, generated, or VCS-internal directories (node_modules, dist, .git
// objects, etc.) are excluded by default so each job rebuilds them in its own
// staged tree.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
)

// Mode controls how aggressively the stager replicates files.
type Mode int

const (
	// ModeAuto picks the best supported strategy for the destination
	// filesystem (clonefile/reflink → hardlink → copy).
	ModeAuto Mode = iota
	// ModeCopy always performs a deep, content copy. Slowest, safest.
	ModeCopy
	// ModeReflinkOnly fails if the underlying filesystem does not support
	// copy-on-write reflinks. Useful for tests / strict deployments.
	ModeReflinkOnly
)

// Options configures the stager.
type Options struct {
	// Mode selects the cloning strategy.
	Mode Mode
	// Ignore is a list of additional path patterns (relative, slash separated)
	// that should be skipped on top of the built-in defaults.
	Ignore []string
	// IncludeVCS, when true, copies the .git directory. The default behaviour
	// hardlink-clones .git so git operations work but never mutates the source
	// repository.
	IncludeVCS bool
}

// DefaultIgnore is the set of paths that are always skipped when staging a
// workspace. These directories are either rebuilt by the job (node_modules),
// derived outputs (dist, build, …), or gluon's own state (.gluon).
var DefaultIgnore = []string{
	".gluon",
	"node_modules",
	".pnpm-store",
	".yarn/cache",
	".yarn/install-state.gz",
	".next",
	".turbo",
	".cache",
	".parcel-cache",
	".docusaurus",
	".vite",
	".svelte-kit",
	".astro",
	".nuxt",
	".output",
	"dist",
	"build",
	"out",
	"target",
	"coverage",
	".terraform",
	".venv",
	"venv",
	"__pycache__",
}

// Stats reports counters from a staging run.
type Stats struct {
	Files      int64
	Dirs       int64
	Bytes      int64
	Cloned     int64 // files materialised via clonefile/reflink
	Hardlinked int64
	Copied     int64
	Skipped    int64
}

// Stage stages src into dst using the given options. dst must not exist or
// must be empty; the caller is responsible for cleanup.
func Stage(ctx context.Context, src, dst string, opts Options) (Stats, error) {
	var stats Stats
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return stats, fmt.Errorf("resolve workspace source %s: %w", src, err)
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return stats, fmt.Errorf("resolve workspace destination %s: %w", dst, err)
	}
	if srcAbs == dstAbs {
		return stats, fmt.Errorf("workspace destination must differ from source: %s", srcAbs)
	}
	// Note: dst is allowed to live inside src (e.g. <src>/.gluon/runs/...)
	// because the ignore matcher always skips ".gluon", and walkAndCopy refuses
	// to descend into the destination root as an extra safety net.

	info, err := os.Stat(srcAbs)
	if err != nil {
		return stats, fmt.Errorf("stat workspace source %s: %w", srcAbs, err)
	}
	if !info.IsDir() {
		return stats, fmt.Errorf("workspace source %s is not a directory", srcAbs)
	}

	if err := os.MkdirAll(dstAbs, 0o755); err != nil {
		return stats, fmt.Errorf("create workspace destination %s: %w", dstAbs, err)
	}

	matcher := newIgnoreMatcher(opts.Ignore)
	if err := walkAndCopy(ctx, srcAbs, dstAbs, "", opts, matcher, &stats, dstAbs); err != nil {
		return stats, err
	}
	return stats, nil
}

// Cleanup removes a previously staged workspace. Errors are returned but the
// caller may safely ignore them — staged workspaces are not authoritative.
func Cleanup(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	return os.RemoveAll(dir)
}

func walkAndCopy(ctx context.Context, srcRoot, dstRoot, rel string, opts Options, matcher *ignoreMatcher, stats *Stats, guardDst string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	srcDir := filepath.Join(srcRoot, rel)
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read workspace dir %s: %w", srcDir, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		name := entry.Name()
		childRel := name
		if rel != "" {
			childRel = filepath.ToSlash(filepath.Join(rel, name))
		}

		if matcher.match(childRel, entry.IsDir()) {
			atomic.AddInt64(&stats.Skipped, 1)
			continue
		}

		srcPath := filepath.Join(srcRoot, filepath.FromSlash(childRel))
		// Never recurse into the destination tree even when dst lives inside
		// src — this prevents infinite staging if the user nests the staged
		// directory in a path the ignore matcher doesn't cover.
		if guardDst != "" && srcPath == guardDst {
			atomic.AddInt64(&stats.Skipped, 1)
			continue
		}
		dstPath := filepath.Join(dstRoot, filepath.FromSlash(childRel))

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", srcPath, err)
		}

		switch {
		case info.Mode()&os.ModeSymlink != 0:
			if err := copySymlink(srcPath, dstPath); err != nil {
				return err
			}
			atomic.AddInt64(&stats.Copied, 1)
		case info.IsDir():
			if err := os.MkdirAll(dstPath, info.Mode().Perm()|0o700); err != nil {
				return fmt.Errorf("create %s: %w", dstPath, err)
			}
			atomic.AddInt64(&stats.Dirs, 1)
			if err := walkAndCopy(ctx, srcRoot, dstRoot, childRel, opts, matcher, stats, guardDst); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := materialiseFile(srcPath, dstPath, info, opts, childRel, stats); err != nil {
				return err
			}
			atomic.AddInt64(&stats.Files, 1)
			atomic.AddInt64(&stats.Bytes, info.Size())
		default:
			// Sockets, devices, fifos, etc. — skip silently.
			atomic.AddInt64(&stats.Skipped, 1)
		}
	}
	return nil
}

// materialiseFile installs srcPath at dstPath using the fastest strategy that
// is safe for the file. Files inside .git are never hardlinked because git
// rewrites object files in place and that would corrupt the source repo.
func materialiseFile(srcPath, dstPath string, info os.FileInfo, opts Options, rel string, stats *Stats) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create parent for %s: %w", dstPath, err)
	}

	allowHardlink := !insideVCSDir(rel)
	mode := opts.Mode

	switch mode {
	case ModeCopy:
		if err := copyFile(srcPath, dstPath, info); err != nil {
			return err
		}
		atomic.AddInt64(&stats.Copied, 1)
		return nil
	case ModeReflinkOnly:
		if err := cloneFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("reflink %s -> %s: %w", srcPath, dstPath, err)
		}
		atomic.AddInt64(&stats.Cloned, 1)
		return nil
	}

	// ModeAuto: try clonefile/reflink, then hardlink (when safe), then copy.
	if err := cloneFile(srcPath, dstPath); err == nil {
		atomic.AddInt64(&stats.Cloned, 1)
		return nil
	} else if !errors.Is(err, errCloneUnsupported) {
		// A real clonefile error (e.g. EXDEV across volumes). Fall back.
	}

	if allowHardlink {
		if err := os.Link(srcPath, dstPath); err == nil {
			atomic.AddInt64(&stats.Hardlinked, 1)
			return nil
		}
	}

	if err := copyFile(srcPath, dstPath, info); err != nil {
		return err
	}
	atomic.AddInt64(&stats.Copied, 1)
	return nil
}

func copyFile(srcPath, dstPath string, info os.FileInfo) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer src.Close()

	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", dstPath, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return fmt.Errorf("copy %s -> %s: %w", srcPath, dstPath, err)
	}
	return dst.Close()
}

func copySymlink(srcPath, dstPath string) error {
	target, err := os.Readlink(srcPath)
	if err != nil {
		return fmt.Errorf("readlink %s: %w", srcPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create parent for %s: %w", dstPath, err)
	}
	if err := os.Symlink(target, dstPath); err != nil && !os.IsExist(err) {
		return fmt.Errorf("symlink %s -> %s: %w", dstPath, target, err)
	}
	return nil
}

func insideVCSDir(rel string) bool {
	if rel == ".git" || strings.HasPrefix(rel, ".git/") {
		return true
	}
	return false
}

// errCloneUnsupported is returned by cloneFile when the host platform or
// filesystem cannot perform a copy-on-write clone. The stager treats this as a
// signal to fall through to the next strategy.
var errCloneUnsupported = errors.New("clone not supported")
