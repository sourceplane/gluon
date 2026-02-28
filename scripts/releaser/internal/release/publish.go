package release

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sourceplane/releaser/internal/config"
)

func publishOCI(opts Options, manifest config.ProviderManifest) error {
	if _, err := exec.LookPath("oras"); err != nil {
		return fmt.Errorf("oras is required for publishing but was not found in PATH")
	}

	providerDir := filepath.Dir(opts.ProviderPath)
	providerBase := filepath.Base(opts.ProviderPath)
	assetsRoot := manifest.Assets.Root
	if assetsRoot == "" {
		assetsRoot = "assets"
	}
	assetsRel := strings.TrimPrefix(filepath.ToSlash(assetsRoot), "./")

	artifactType := manifest.Distribution.ArtifactType
	if artifactType == "" {
		artifactType = "application/vnd.thin.provider.v1"
	}
	coreMediaType := manifest.Layers.Core.MediaType
	if coreMediaType == "" {
		coreMediaType = "application/vnd.sourceplane.provider.v1"
	}
	assetsMediaType := manifest.Layers.Core.AssetsMediaType
	if assetsMediaType == "" {
		assetsMediaType = "application/vnd.sourceplane.assets.v1"
	}

	mediaByPlatform := map[string]string{}
	for _, layer := range manifest.Layers.Binaries {
		if layer.Platform != "" && layer.MediaType != "" {
			mediaByPlatform[layer.Platform] = layer.MediaType
		}
	}

	args := []string{
		"push",
		opts.PublishRef,
		"--artifact-type", artifactType,
		fmt.Sprintf("%s:%s", filepath.ToSlash(filepath.Join(opts.OutputDir, providerBase)), coreMediaType),
		fmt.Sprintf("%s/:%s", filepath.ToSlash(filepath.Join(opts.OutputDir, filepath.FromSlash(assetsRel))), assetsMediaType),
	}

	for _, platform := range manifest.Platforms {
		platformKey := platform.OS + "/" + platform.Arch
		mediaType := mediaByPlatform[platformKey]
		if mediaType == "" {
			mediaType = fmt.Sprintf("application/vnd.thin.provider.bin.%s-%s", platform.OS, platform.Arch)
		}
		args = append(args, fmt.Sprintf("%s:%s", filepath.ToSlash(filepath.Join(opts.OutputDir, filepath.FromSlash(platform.Binary))), mediaType))
	}

	examplesDir, examplesMediaType := examplesConfig(manifest)
	examplesPath := examplesDir
	if !filepath.IsAbs(examplesPath) {
		examplesPath = filepath.Join(providerDir, examplesPath)
	}
	if dirExists(examplesPath) {
		entries, err := os.ReadDir(examplesPath)
		if err == nil && len(entries) > 0 {
			args = append(args, fmt.Sprintf("%s/:%s", filepath.ToSlash(examplesPath), examplesMediaType))
		}
	}

	cmd := exec.Command("oras", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Printf("✅ OCI artifact published at %s\n", opts.PublishRef)
	return nil
}

func examplesConfig(manifest config.ProviderManifest) (string, string) {
	examplesDir := "examples"
	if len(manifest.Layers.Examples.Includes) > 0 {
		v := manifest.Layers.Examples.Includes[0]
		v = strings.TrimPrefix(v, "./")
		v = strings.TrimSuffix(v, "/**")
		v = strings.TrimSuffix(v, "/*")
		if v != "" {
			examplesDir = v
		}
	}

	examplesMediaType := manifest.Layers.Examples.MediaType
	if examplesMediaType == "" {
		examplesMediaType = "application/vnd.sourceplane.examples.v1"
	}
	return examplesDir, examplesMediaType
}
