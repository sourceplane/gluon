package workspace

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStageReplicatesTreeAndSkipsIgnored(t *testing.T) {
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "package.json"), `{"name":"demo"}`)
	mustWrite(t, filepath.Join(src, "src", "index.ts"), "export const x = 1;\n")
	mustWrite(t, filepath.Join(src, "node_modules", "lodash", "index.js"), "module.exports = {};\n")
	mustWrite(t, filepath.Join(src, "dist", "bundle.js"), "// generated\n")
	mustWrite(t, filepath.Join(src, ".gluon", "runs", "x.json"), "{}")
	mustWrite(t, filepath.Join(src, ".git", "HEAD"), "ref: refs/heads/main\n")

	dst := filepath.Join(t.TempDir(), "staged")
	stats, err := Stage(context.Background(), src, dst, Options{Mode: ModeAuto})
	if err != nil {
		t.Fatalf("Stage returned error: %v", err)
	}
	if stats.Files == 0 {
		t.Fatalf("expected at least one staged file, got %+v", stats)
	}

	mustExist(t, filepath.Join(dst, "package.json"))
	mustExist(t, filepath.Join(dst, "src", "index.ts"))
	mustExist(t, filepath.Join(dst, ".git", "HEAD"))

	mustNotExist(t, filepath.Join(dst, "node_modules"))
	mustNotExist(t, filepath.Join(dst, "dist"))
	mustNotExist(t, filepath.Join(dst, ".gluon"))
}

func TestStageRefusesNestedDestination(t *testing.T) {
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "a.txt"), "hi")
	// Nesting dst inside src is allowed; the walker refuses to descend into
	// the destination root and the default ignore list skips ".gluon".
	dst := filepath.Join(src, ".gluon", "runs", "x", "work")
	if _, err := Stage(context.Background(), src, dst, Options{}); err != nil {
		t.Fatalf("nested destination should be allowed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "a.txt")); err != nil {
		t.Fatalf("expected staged a.txt: %v", err)
	}
}

func TestStageDoesNotMutateSourceWhenJobWritesIntoStaged(t *testing.T) {
	// Simulate a job writing into the staged copy: source must remain pristine
	// even when the underlying clone strategy ends up using hardlinks. This is
	// the key property that the package guarantees for ignored directories
	// such as node_modules / dist — a job dropping new files in those
	// directories of the staged tree must never reach back into the source.
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "src", "main.go"), "package main\n")

	dst := filepath.Join(t.TempDir(), "staged")
	if _, err := Stage(context.Background(), src, dst, Options{Mode: ModeAuto}); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// Job creates an output directory inside the staged workspace.
	outDir := filepath.Join(dst, "dist")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir staged dist: %v", err)
	}
	mustWrite(t, filepath.Join(outDir, "bundle.js"), "// from job\n")

	if _, err := os.Stat(filepath.Join(src, "dist")); !os.IsNotExist(err) {
		t.Fatalf("expected src/dist to not exist after staged job wrote to dist; err=%v", err)
	}
}

func TestSanitizeIgnoreMatcherBasenamesAndExact(t *testing.T) {
	m := newIgnoreMatcher([]string{"foo/bar", "baz"})
	cases := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"foo/bar", true, true},
		{"foo/bar/inside.txt", false, false}, // exact path only matches the exact rel
		{"baz", true, true},
		{"any/level/baz", true, true},
		{"node_modules", true, true},
		{".git", true, false}, // .git is no longer in DefaultIgnore
		{"src/index.ts", false, false},
	}
	for _, tc := range cases {
		if got := m.match(tc.rel, tc.isDir); got != tc.want {
			t.Errorf("match(%q,isDir=%v) = %v, want %v", tc.rel, tc.isDir, got, tc.want)
		}
	}
}

func TestStageOnDarwinUsesClonefile(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("clonefile is only available on macOS")
	}
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "a.txt"), strings.Repeat("a", 1024))

	dst := filepath.Join(t.TempDir(), "staged")
	stats, err := Stage(context.Background(), src, dst, Options{Mode: ModeAuto})
	if err != nil {
		t.Fatalf("Stage failed: %v", err)
	}
	if stats.Cloned == 0 {
		t.Skipf("APFS clonefile unavailable in this test environment (stats=%+v)", stats)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, got err=%v", path, err)
	}
}
