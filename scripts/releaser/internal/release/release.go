package release

import (
	"fmt"

	"github.com/sourceplane/releaser/internal/config"
)

type Options struct {
	ProviderPath string
	BuildWith    string
	DistDir      string
	OutputDir    string
	PublishRef   string
}

func Run(opts Options) error {
	manifest, err := config.LoadProviderManifest(opts.ProviderPath)
	if err != nil {
		return fmt.Errorf("failed to load provider manifest: %w", err)
	}

	if err := runBuild(opts, manifest); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	if err := packageOCI(opts, manifest); err != nil {
		return fmt.Errorf("packaging failed: %w", err)
	}

	if opts.PublishRef != "" {
		if err := publishOCI(opts, manifest); err != nil {
			return fmt.Errorf("publish failed: %w", err)
		}
	}

	return nil
}
