package gha

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ActionMetadata struct {
	Name    string                  `yaml:"name"`
	Inputs  map[string]ActionInput  `yaml:"inputs"`
	Outputs map[string]ActionOutput `yaml:"outputs"`
	Runs    ActionRuns              `yaml:"runs"`
}

type ActionInput struct {
	Description string      `yaml:"description"`
	Required    bool        `yaml:"required"`
	Default     interface{} `yaml:"default"`
}

type ActionOutput struct {
	Description string `yaml:"description"`
	Value       string `yaml:"value"`
}

type ActionRuns struct {
	Using          string                `yaml:"using"`
	Main           string                `yaml:"main"`
	Pre            string                `yaml:"pre"`
	Post           string                `yaml:"post"`
	PreIf          string                `yaml:"pre-if"`
	PostIf         string                `yaml:"post-if"`
	Image          string                `yaml:"image"`
	Env            map[string]string     `yaml:"env"`
	Entrypoint     string                `yaml:"entrypoint"`
	PreEntrypoint  string                `yaml:"pre-entrypoint"`
	PostEntrypoint string                `yaml:"post-entrypoint"`
	Args           []string              `yaml:"args"`
	Steps          []CompositeActionStep `yaml:"steps"`
}

type CompositeActionStep struct {
	ID               string                 `yaml:"id"`
	Name             string                 `yaml:"name"`
	Run              string                 `yaml:"run"`
	Uses             string                 `yaml:"uses"`
	With             map[string]interface{} `yaml:"with"`
	Env              map[string]interface{} `yaml:"env"`
	Shell            string                 `yaml:"shell"`
	WorkingDirectory string                 `yaml:"working-directory"`
	If               string                 `yaml:"if"`
	ContinueOnError  bool                   `yaml:"continue-on-error"`
}

func LoadActionMetadata(actionDir string) (*ActionMetadata, string, error) {
	paths := []string{
		filepath.Join(actionDir, "action.yml"),
		filepath.Join(actionDir, "action.yaml"),
	}

	var metadataPath string
	for _, candidate := range paths {
		if _, err := os.Stat(candidate); err == nil {
			metadataPath = candidate
			break
		}
	}
	if metadataPath == "" {
		return nil, "", fmt.Errorf("action metadata not found in %s", actionDir)
	}

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, "", fmt.Errorf("read action metadata %s: %w", metadataPath, err)
	}

	var metadata ActionMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return nil, "", fmt.Errorf("parse action metadata %s: %w", metadataPath, err)
	}

	metadata.Runs.Using = strings.ToLower(strings.TrimSpace(metadata.Runs.Using))
	if metadata.Inputs == nil {
		metadata.Inputs = map[string]ActionInput{}
	}
	if metadata.Outputs == nil {
		metadata.Outputs = map[string]ActionOutput{}
	}
	if metadata.Runs.Env == nil {
		metadata.Runs.Env = map[string]string{}
	}

	return &metadata, metadataPath, nil
}
