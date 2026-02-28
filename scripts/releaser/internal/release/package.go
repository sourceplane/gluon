package release

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sourceplane/releaser/internal/config"
)

func packageOCI(opts Options, manifest config.ProviderManifest) error {
	if !dirExists(opts.DistDir) {
		return fmt.Errorf("dist directory not found: %s", opts.DistDir)
	}

	providerDir := filepath.Dir(opts.ProviderPath)

	if err := os.RemoveAll(opts.OutputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return err
	}

	providerSource := opts.ProviderPath
	if !filepath.IsAbs(providerSource) {
		providerSource = filepath.Clean(providerSource)
	}
	providerBase := filepath.Base(providerSource)
	if err := copyFile(providerSource, filepath.Join(opts.OutputDir, providerBase), 0o644); err != nil {
		return err
	}

	assetsRoot := manifest.Assets.Root
	if assetsRoot == "" {
		assetsRoot = "assets"
	}
	assetsRel := strings.TrimPrefix(filepath.ToSlash(assetsRoot), "./")

	assetsSource := assetsRoot
	if !filepath.IsAbs(assetsSource) {
		assetsSource = filepath.Join(providerDir, assetsSource)
	}
	if !dirExists(assetsSource) {
		return fmt.Errorf("assets directory declared in provider not found: %s", assetsSource)
	}

	if err := copyDir(assetsSource, filepath.Join(opts.OutputDir, filepath.FromSlash(assetsRel))); err != nil {
		return err
	}

	execName := manifest.Entrypoint.Executable
	if execName == "" {
		execName = "entrypoint"
	}

	for _, platform := range manifest.Platforms {
		if platform.OS == "" || platform.Arch == "" || platform.Binary == "" {
			return errors.New("platform entries must include os, arch, and binary")
		}

		targetFile := filepath.Join(opts.OutputDir, filepath.FromSlash(platform.Binary))
		if err := os.MkdirAll(filepath.Dir(targetFile), 0o755); err != nil {
			return err
		}

		srcBinary, err := findBinaryInDist(opts.DistDir, platform.OS, platform.Arch, execName)
		if err != nil {
			return err
		}

		if err := copyFile(srcBinary, targetFile, 0o755); err != nil {
			return err
		}
	}

	fmt.Printf("OCI layout built at: %s\n", opts.OutputDir)
	return nil
}

func findBinaryInDist(distDir, osName, arch, execName string) (string, error) {
	archivePath := firstArchiveForPlatform(distDir, osName, arch)
	if archivePath != "" {
		path, err := extractBinaryFromArchive(archivePath, execName)
		if err != nil {
			return "", err
		}
		return path, nil
	}

	matches := []string{}
	_ = filepath.WalkDir(distDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(path) != execName {
			return nil
		}
		n := strings.ToLower(filepath.ToSlash(path))
		if strings.Contains(n, osName) && strings.Contains(n, arch) {
			matches = append(matches, path)
		}
		return nil
	})

	sort.Strings(matches)
	if len(matches) == 0 {
		return "", fmt.Errorf("no archive or built binary found for %s/%s in %s", osName, arch, distDir)
	}
	return matches[0], nil
}

func firstArchiveForPlatform(distDir, osName, arch string) string {
	entries, err := os.ReadDir(distDir)
	if err != nil {
		return ""
	}

	suffix := "_" + osName + "_" + arch + ".tar.gz"
	matches := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, suffix) {
			matches = append(matches, filepath.Join(distDir, name))
		}
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

func extractBinaryFromArchive(archivePath, execName string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != execName {
			continue
		}

		tmpFile, err := os.CreateTemp("", execName+"-*")
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(tmpFile, tr); err != nil {
			tmpFile.Close()
			return "", err
		}
		if err := tmpFile.Close(); err != nil {
			return "", err
		}
		if err := os.Chmod(tmpFile.Name(), 0o755); err != nil {
			return "", err
		}
		return tmpFile.Name(), nil
	}

	return "", fmt.Errorf("could not find executable '%s' inside %s", execName, archivePath)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		return copyFile(path, target, mode)
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}
