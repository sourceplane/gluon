package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type ProviderManifest struct {
	Distribution struct {
		ArtifactType string `yaml:"artifactType"`
	} `yaml:"distribution"`
	Goreleaser struct {
		Config string `yaml:"config"`
	} `yaml:"goreleaser"`
	Entrypoint struct {
		Executable string `yaml:"executable"`
	} `yaml:"entrypoint"`
	Assets struct {
		Root string `yaml:"root"`
	} `yaml:"assets"`
	Platforms []struct {
		OS     string `yaml:"os"`
		Arch   string `yaml:"arch"`
		Binary string `yaml:"binary"`
	} `yaml:"platforms"`
	Layers struct {
		Core struct {
			MediaType       string `yaml:"mediaType"`
			AssetsMediaType string `yaml:"assetsMediaType"`
		} `yaml:"core"`
		Binaries map[string]struct {
			MediaType string `yaml:"mediaType"`
			Platform  string `yaml:"platform"`
		} `yaml:"binaries"`
		Examples struct {
			Includes  []string `yaml:"includes"`
			MediaType string   `yaml:"mediaType"`
		} `yaml:"examples"`
	} `yaml:"layers"`
}

func LoadProviderManifest(path string) (ProviderManifest, error) {
	var manifest ProviderManifest

	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}

	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}

	if len(manifest.Platforms) == 0 {
		return manifest, errors.New("no platforms declared")
	}

	return manifest, nil
}
