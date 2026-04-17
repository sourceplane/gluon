package gha

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const containerFileCommandRoot = "/github/file_commands"

type StepFiles struct {
	HostDir          string
	HostEnv          string
	HostOutput       string
	HostPath         string
	HostState        string
	HostSummary      string
	ContainerEnv     string
	ContainerOutput  string
	ContainerPath    string
	ContainerState   string
	ContainerSummary string
}

type FileCommandResult struct {
	Env     map[string]string
	Outputs map[string]string
	State   map[string]string
	Paths   []string
	Summary string
}

func NewStepFiles(root string) (StepFiles, error) {
	id, err := randomHex(6)
	if err != nil {
		return StepFiles{}, err
	}

	hostDir := filepath.Join(root, id)
	if err := os.MkdirAll(hostDir, 0755); err != nil {
		return StepFiles{}, fmt.Errorf("create runner file-command directory: %w", err)
	}

	hostEnv := filepath.Join(hostDir, "env")
	hostOutput := filepath.Join(hostDir, "output")
	hostPath := filepath.Join(hostDir, "path")
	hostState := filepath.Join(hostDir, "state")
	hostSummary := filepath.Join(hostDir, "summary")

	for _, filePath := range []string{hostEnv, hostOutput, hostPath, hostState, hostSummary} {
		if err := os.WriteFile(filePath, nil, 0644); err != nil {
			return StepFiles{}, fmt.Errorf("create runner command file %s: %w", filePath, err)
		}
	}

	containerDir := path.Join(containerFileCommandRoot, id)
	return StepFiles{
		HostDir:          hostDir,
		HostEnv:          hostEnv,
		HostOutput:       hostOutput,
		HostPath:         hostPath,
		HostState:        hostState,
		HostSummary:      hostSummary,
		ContainerEnv:     path.Join(containerDir, "env"),
		ContainerOutput:  path.Join(containerDir, "output"),
		ContainerPath:    path.Join(containerDir, "path"),
		ContainerState:   path.Join(containerDir, "state"),
		ContainerSummary: path.Join(containerDir, "summary"),
	}, nil
}

func (files StepFiles) Parse() (FileCommandResult, error) {
	env, err := parseKVFile(files.HostEnv)
	if err != nil {
		return FileCommandResult{}, fmt.Errorf("parse GITHUB_ENV: %w", err)
	}

	outputs, err := parseKVFile(files.HostOutput)
	if err != nil {
		return FileCommandResult{}, fmt.Errorf("parse GITHUB_OUTPUT: %w", err)
	}

	state, err := parseKVFile(files.HostState)
	if err != nil {
		return FileCommandResult{}, fmt.Errorf("parse GITHUB_STATE: %w", err)
	}

	paths, err := parsePathFile(files.HostPath)
	if err != nil {
		return FileCommandResult{}, fmt.Errorf("parse GITHUB_PATH: %w", err)
	}

	summaryBytes, err := os.ReadFile(files.HostSummary)
	if err != nil && !os.IsNotExist(err) {
		return FileCommandResult{}, fmt.Errorf("read GITHUB_STEP_SUMMARY: %w", err)
	}

	return FileCommandResult{
		Env:     env,
		Outputs: outputs,
		State:   state,
		Paths:   paths,
		Summary: string(summaryBytes),
	}, nil
}

func parseKVFile(filePath string) (map[string]string, error) {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}

	content := string(contentBytes)
	content = strings.TrimPrefix(content, "\ufeff")
	values := map[string]string{}

	for index := 0; index < len(content); {
		line, next, hadNewline := readLine(content, index)
		index = next
		if line == "" && hadNewline {
			continue
		}

		heredocIndex := strings.Index(line, "<<")
		equalsIndex := strings.Index(line, "=")
		if heredocIndex >= 0 && (equalsIndex == -1 || heredocIndex < equalsIndex) {
			key := strings.TrimSpace(line[:heredocIndex])
			delimiter := line[heredocIndex+2:]
			if key == "" || delimiter == "" {
				return nil, fmt.Errorf("invalid heredoc assignment %q", line)
			}

			var builder strings.Builder
			matched := false
			for index < len(content) {
				nextLine, nextIndex, nextHadNewline := readLine(content, index)
				index = nextIndex
				if nextLine == delimiter {
					matched = true
					break
				}
				builder.WriteString(nextLine)
				if nextHadNewline {
					builder.WriteByte('\n')
				}
			}
			if !matched {
				return nil, fmt.Errorf("matching delimiter %q not found", delimiter)
			}
			values[key] = builder.String()
			continue
		}

		if equalsIndex <= 0 {
			if strings.TrimSpace(line) == "" {
				continue
			}
			return nil, fmt.Errorf("invalid file-command assignment %q", line)
		}

		values[line[:equalsIndex]] = line[equalsIndex+1:]
	}

	return values, nil
}

func parsePathFile(filePath string) ([]string, error) {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	content := strings.TrimPrefix(string(contentBytes), "\ufeff")
	values := make([]string, 0)
	for index := 0; index < len(content); {
		line, next, _ := readLine(content, index)
		index = next
		if strings.TrimSpace(line) == "" {
			continue
		}
		values = append(values, line)
	}
	return values, nil
}

func readLine(content string, start int) (string, int, bool) {
	for index := start; index < len(content); index++ {
		if content[index] == '\n' {
			line := strings.TrimSuffix(content[start:index], "\r")
			return line, index + 1, true
		}
	}
	line := strings.TrimSuffix(content[start:], "\r")
	return line, len(content), false
}

func randomHex(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
