package release

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sourceplane/releaser/internal/config"
)

func runBuild(opts Options, manifest config.ProviderManifest) error {
	if opts.BuildWith == "" {
		return nil
	}

	switch opts.BuildWith {
	case "goreleaser", "gorelaser":
		if _, err := exec.LookPath("goreleaser"); err != nil {
			return fmt.Errorf("goreleaser not found in PATH")
		}

		providerDir := filepath.Dir(opts.ProviderPath)
		configPath := manifest.Goreleaser.Config
		if configPath == "" {
			configPath = ".goreleaser.yaml"
		}
		if !filepath.IsAbs(configPath) {
			configPath = filepath.Join(providerDir, configPath)
		}

		overridePath := filepath.Join(providerDir, ".goreleaser.yaml")
		if fileExists(overridePath) {
			configPath = overridePath
		}
		if !fileExists(configPath) {
			return fmt.Errorf("goreleaser config not found: %s", configPath)
		}

		fmt.Printf("Using GoReleaser config: %s\n", configPath)
		cmd := exec.Command("goreleaser", "build", "--clean", "--config", configPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported --build-with value: %s", opts.BuildWith)
	}
}
