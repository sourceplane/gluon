package gha

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type referenceKind string

const (
	referenceKindRemote referenceKind = "remote"
	referenceKindLocal  referenceKind = "local"
	referenceKindDocker referenceKind = "docker"
)

var remoteReferencePattern = regexp.MustCompile(`^([^/@]+)/([^/@]+)(?:/(.+))?@(.+)$`)

type ActionReference struct {
	Kind     referenceKind
	Original string
	Owner    string
	Repo     string
	Path     string
	Ref      string
	Image    string
	Local    string
}

func ParseActionReference(raw string) (ActionReference, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ActionReference{}, fmt.Errorf("action reference cannot be empty")
	}

	if strings.HasPrefix(value, "docker://") {
		image := strings.TrimSpace(strings.TrimPrefix(value, "docker://"))
		if image == "" {
			return ActionReference{}, fmt.Errorf("docker action reference %q is missing an image", raw)
		}
		return ActionReference{Kind: referenceKindDocker, Original: value, Image: image}, nil
	}

	if strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") {
		return ActionReference{Kind: referenceKindLocal, Original: value, Local: filepath.Clean(value)}, nil
	}

	matches := remoteReferencePattern.FindStringSubmatch(value)
	if len(matches) != 5 {
		return ActionReference{}, fmt.Errorf("unsupported action reference %q; expected ./path, docker://image, or owner/repo[/path]@ref", raw)
	}

	return ActionReference{
		Kind:     referenceKindRemote,
		Original: value,
		Owner:    matches[1],
		Repo:     matches[2],
		Path:     strings.TrimSpace(matches[3]),
		Ref:      strings.TrimSpace(matches[4]),
	}, nil
}

func (ref ActionReference) Repository() string {
	if ref.Owner == "" || ref.Repo == "" {
		return ""
	}
	return ref.Owner + "/" + ref.Repo
}

func (ref ActionReference) CachePath() string {
	if ref.Owner == "" || ref.Repo == "" {
		return ""
	}
	return filepath.Join(ref.Owner, ref.Repo)
}
