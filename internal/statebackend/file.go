package statebackend

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/sourceplane/orun/internal/model"
	"github.com/sourceplane/orun/internal/state"
)

// FileStateBackend implements Backend using the local .orun/ filesystem store.
// It is a thin adapter so that status and logs commands can use the same
// interface regardless of whether remote state is active.
type FileStateBackend struct {
	Store *state.Store
}

// NewFileStateBackend wraps an existing state.Store.
func NewFileStateBackend(store *state.Store) *FileStateBackend {
	return &FileStateBackend{Store: store}
}

func (f *FileStateBackend) InitRun(_ context.Context, plan *model.Plan, opts InitRunOptions) (*RunHandle, error) {
	if err := f.Store.EnsureDirs(); err != nil {
		return nil, err
	}
	if _, err := f.Store.CreateExecution(opts.RunID, plan); err != nil {
		return nil, err
	}
	return &RunHandle{RunID: opts.RunID}, nil
}

// ClaimJob always succeeds locally — there is no concurrent runner to compete with.
func (f *FileStateBackend) ClaimJob(_ context.Context, _ string, _ model.PlanJob, _ string) (*ClaimResult, error) {
	return &ClaimResult{Claimed: true}, nil
}

// Heartbeat is a no-op locally.
func (f *FileStateBackend) Heartbeat(_ context.Context, _, _, _ string) (*HeartbeatResult, error) {
	return &HeartbeatResult{OK: true}, nil
}

// UpdateJob is a no-op locally — the runner updates local state directly.
func (f *FileStateBackend) UpdateJob(_ context.Context, _, _, _ string, _ JobStatus, _ string) error {
	return nil
}

// AppendStepLog is a no-op locally — the runner writes logs via writeStepLog.
func (f *FileStateBackend) AppendStepLog(_ context.Context, _, _, _ string) error {
	return nil
}

func (f *FileStateBackend) LoadRunState(_ context.Context, runID string) (*state.ExecState, *state.ExecMetadata, error) {
	st, err := f.Store.LoadState(runID)
	if err != nil {
		return nil, nil, err
	}
	meta, err := f.Store.LoadMetadata(runID)
	if err != nil {
		return nil, nil, err
	}
	return st, meta, nil
}

// ReadJobLog concatenates all per-step log files for the given job.
func (f *FileStateBackend) ReadJobLog(_ context.Context, runID string, jobID string) (string, error) {
	logDir := f.Store.LogDir(runID, jobID)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var sb strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(logDir, entry.Name()))
		if readErr == nil && len(data) > 0 {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.Write(data)
		}
	}
	return sb.String(), nil
}

func (f *FileStateBackend) Close(_ context.Context) error { return nil }
