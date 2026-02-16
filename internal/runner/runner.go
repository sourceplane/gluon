package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sourceplane/liteci/internal/model"
)

// Runner executes a compiled plan in dependency order.
type Runner struct {
	WorkDir string
	Stdout  io.Writer
	Stderr  io.Writer
	DryRun  bool
	JobID   string
	Retry   bool
}

type State struct {
	PlanChecksum string               `json:"planChecksum"`
	Jobs         map[string]*JobState `json:"jobs"`
}

type JobState struct {
	Status     string            `json:"status"`
	StartedAt  string            `json:"startedAt,omitempty"`
	FinishedAt string            `json:"finishedAt,omitempty"`
	Steps      map[string]string `json:"steps"`
	LastError  string            `json:"lastError,omitempty"`
}

type runSummary struct {
	completed int
	skipped   int
	failed    int
	waiting   int
}

func NewRunner(workDir string, stdout, stderr io.Writer, dryRun bool, jobID string, retry bool) *Runner {
	return &Runner{
		WorkDir: workDir,
		Stdout:  stdout,
		Stderr:  stderr,
		DryRun:  dryRun,
		JobID:   jobID,
		Retry:   retry,
	}
}

func (r *Runner) Run(plan *model.Plan) error {
	if plan == nil {
		return fmt.Errorf("plan cannot be nil")
	}
	if len(plan.Jobs) == 0 {
		return fmt.Errorf("plan has no jobs")
	}

	statePath := r.resolveStateFile(plan)
	state, err := r.loadState(statePath)
	if err != nil {
		return err
	}

	if state.PlanChecksum == "" {
		state.PlanChecksum = plan.Metadata.Checksum
	}
	if plan.Metadata.Checksum != "" && state.PlanChecksum != "" && state.PlanChecksum != plan.Metadata.Checksum {
		return fmt.Errorf("state file checksum mismatch: expected %s, got %s", plan.Metadata.Checksum, state.PlanChecksum)
	}

	if r.JobID != "" && r.Retry {
		state.Jobs[r.JobID] = nil
	}

	persistState := !r.DryRun

	r.printRunHeader(plan, statePath)

	orderedJobs, err := topologicalOrder(plan.Jobs)
	if err != nil {
		return err
	}

	r.printReadinessSnapshot(orderedJobs, state)

	failFast := plan.Execution.FailFast
	if !plan.Execution.FailFast {
		// keep explicit false as is
	} else {
		failFast = true
	}

	summary := &runSummary{}

	executedTarget := false

	for _, job := range orderedJobs {
		if r.JobID != "" && job.ID != r.JobID {
			continue
		}

		unmet := unresolvedDependencies(job, state)
		if len(unmet) > 0 {
			summary.waiting++
			r.printWaiting(job, unmet, state)
			if r.JobID != "" {
				return fmt.Errorf("cannot run %s: dependencies not completed (%s)", job.ID, strings.Join(unmet, ", "))
			}
			continue
		}

		executedTarget = true

		jobState := ensureJobState(state, job)
		if jobState.Status == "completed" {
			summary.skipped++
			fmt.Fprintf(r.Stdout, "↷ Skip job %s (already completed)\n", job.ID)
			continue
		}

		jobState.Status = "running"
		jobState.FinishedAt = ""
		jobState.LastError = ""
		if jobState.StartedAt == "" {
			jobState.StartedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if persistState {
			if err := r.saveState(statePath, state); err != nil {
				return err
			}
		}

		fmt.Fprintf(r.Stdout, "\n▶ Job %s (%s/%s)\n", job.ID, job.Component, job.Environment)
		fmt.Fprintf(r.Stdout, "  status: ready (all dependency conditions met)\n")

		jobFailed := false
		for _, step := range job.Steps {
			stepID := stepIdentifier(step)
			if jobState.Steps[stepID] == "completed" {
				fmt.Fprintf(r.Stdout, "  ↷ Step %s (already completed)\n", stepID)
				continue
			}

			jobState.Steps[stepID] = "running"
			if persistState {
				if err := r.saveState(statePath, state); err != nil {
					return err
				}
			}

			fmt.Fprintf(r.Stdout, "  • Step %s\n", stepID)
			fmt.Fprintf(r.Stdout, "    phase=%s order=%d\n", normalizeStepPhase(step.Phase), step.Order)
			if r.DryRun {
				fmt.Fprintf(r.Stdout, "    run: %s\n", step.Run)
				jobState.Steps[stepID] = "completed"
				if persistState {
					if err := r.saveState(statePath, state); err != nil {
						return err
					}
				}
				fmt.Fprintf(r.Stdout, "    ✓ completed (dry-run)\n")
				continue
			}

			cmd := exec.Command("sh", "-c", step.Run)
			cmd.Dir = r.resolveWorkingDir(job.Path)
			cmd.Stdout = r.Stdout
			cmd.Stderr = r.Stderr

			if err := cmd.Run(); err != nil {
				jobState.Steps[stepID] = "failed"
				jobState.Status = "failed"
				jobState.LastError = fmt.Sprintf("step %s: %v", stepID, err)
				jobState.FinishedAt = time.Now().UTC().Format(time.RFC3339)
				if persistState {
					if err := r.saveState(statePath, state); err != nil {
						return err
					}
				}

				fmt.Fprintf(r.Stdout, "    ✗ failed: %v\n", err)
				if strings.EqualFold(step.OnFailure, "continue") {
					fmt.Fprintf(r.Stdout, "    ⚠ onFailure=continue, moving to next step\n")
					continue
				}

				jobFailed = true
				summary.failed++
				if failFast {
					return fmt.Errorf("job %s step %s failed: %w", job.ID, stepID, err)
				}
				break
			}

			jobState.Steps[stepID] = "completed"
			if persistState {
				if err := r.saveState(statePath, state); err != nil {
					return err
				}
			}
			fmt.Fprintf(r.Stdout, "    ✓ completed\n")
		}

		if jobState.Status != "failed" {
			jobState.Status = "completed"
			jobState.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			jobState.LastError = ""
			if persistState {
				if err := r.saveState(statePath, state); err != nil {
					return err
				}
			}
			summary.completed++
			fmt.Fprintf(r.Stdout, "  ✓ Job %s completed\n", job.ID)
		} else if !jobFailed {
			summary.failed++
		}

		if r.JobID != "" {
			break
		}
	}

	if r.JobID != "" {
		if _, exists := state.Jobs[r.JobID]; !exists {
			return fmt.Errorf("job not found: %s", r.JobID)
		}
		if !executedTarget {
			return fmt.Errorf("job not found in runnable set: %s", r.JobID)
		}
	}

	r.printRunSummary(summary)

	return nil
}

func (r *Runner) printRunHeader(plan *model.Plan, statePath string) {
	fmt.Fprintln(r.Stdout, "═══════════════════════════════════════════════════════════")
	fmt.Fprintln(r.Stdout, "liteci run")
	fmt.Fprintln(r.Stdout, "═══════════════════════════════════════════════════════════")
	fmt.Fprintf(r.Stdout, "plan: %s (%s)\n", plan.Metadata.Name, plan.Metadata.Checksum)
	fmt.Fprintf(r.Stdout, "jobs: %d\n", len(plan.Jobs))
	fmt.Fprintf(r.Stdout, "state: %s\n", statePath)
	mode := "execute"
	if r.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(r.Stdout, "mode: %s\n", mode)
	if r.JobID != "" {
		fmt.Fprintf(r.Stdout, "target-job: %s\n", r.JobID)
	}
	fmt.Fprintln(r.Stdout, "═══════════════════════════════════════════════════════════")
}

func (r *Runner) printReadinessSnapshot(jobs []model.PlanJob, state *State) {
	fmt.Fprintln(r.Stdout, "Job readiness snapshot:")
	for _, job := range jobs {
		if r.JobID != "" && job.ID != r.JobID {
			continue
		}

		jobState := state.Jobs[job.ID]
		if jobState != nil && jobState.Status == "completed" {
			fmt.Fprintf(r.Stdout, "  ✓ %s (completed from state)\n", job.ID)
			continue
		}

		unmet := unresolvedDependencies(job, state)
		if len(unmet) > 0 {
			fmt.Fprintf(r.Stdout, "  ⏳ %s waiting for: %s\n", job.ID, strings.Join(unmet, ", "))
			continue
		}

		fmt.Fprintf(r.Stdout, "  ▶ %s ready\n", job.ID)
	}
}

func (r *Runner) printWaiting(job model.PlanJob, unmet []string, state *State) {
	fmt.Fprintf(r.Stdout, "⏳ Job %s waiting for dependencies:\n", job.ID)
	for _, dep := range unmet {
		status := "pending"
		if depState, ok := state.Jobs[dep]; ok && depState != nil && depState.Status != "" {
			status = depState.Status
		}
		fmt.Fprintf(r.Stdout, "   - %s (%s)\n", dep, status)
	}
}

func (r *Runner) printRunSummary(summary *runSummary) {
	fmt.Fprintln(r.Stdout, "\n═══════════════════════════════════════════════════════════")
	fmt.Fprintln(r.Stdout, "Run summary")
	fmt.Fprintln(r.Stdout, "═══════════════════════════════════════════════════════════")
	fmt.Fprintf(r.Stdout, "completed: %d\n", summary.completed)
	fmt.Fprintf(r.Stdout, "skipped:   %d\n", summary.skipped)
	fmt.Fprintf(r.Stdout, "waiting:   %d\n", summary.waiting)
	fmt.Fprintf(r.Stdout, "failed:    %d\n", summary.failed)
}

func normalizeStepPhase(phase string) string {
	p := strings.TrimSpace(strings.ToLower(phase))
	if p == "" {
		return "main"
	}
	return p
}

func (r *Runner) resolveStateFile(plan *model.Plan) string {
	stateFile := ".liteci-state.json"
	if plan != nil && strings.TrimSpace(plan.Execution.StateFile) != "" {
		stateFile = strings.TrimSpace(plan.Execution.StateFile)
	}
	if filepath.IsAbs(stateFile) {
		return stateFile
	}
	return filepath.Join(r.WorkDir, stateFile)
}

func (r *Runner) loadState(statePath string) (*State, error) {
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{Jobs: map[string]*JobState{}}, nil
		}
		return nil, fmt.Errorf("failed to read state file %s: %w", statePath, err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file %s: %w", statePath, err)
	}
	if state.Jobs == nil {
		state.Jobs = map[string]*JobState{}
	}

	return &state, nil
}

func (r *Runner) saveState(statePath string, state *State) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}

	dir := filepath.Dir(statePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create state directory: %w", err)
		}
	}

	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	tmp := statePath + ".tmp"
	if err := os.WriteFile(tmp, payload, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}
	if err := os.Rename(tmp, statePath); err != nil {
		return fmt.Errorf("failed to atomically replace state file: %w", err)
	}

	return nil
}

func ensureJobState(state *State, job model.PlanJob) *JobState {
	jobState, exists := state.Jobs[job.ID]
	if !exists || jobState == nil {
		jobState = &JobState{
			Status: "pending",
			Steps:  map[string]string{},
		}
		state.Jobs[job.ID] = jobState
	}
	if jobState.Steps == nil {
		jobState.Steps = map[string]string{}
	}

	for _, step := range job.Steps {
		stepID := stepIdentifier(step)
		if _, ok := jobState.Steps[stepID]; !ok {
			jobState.Steps[stepID] = "pending"
		}
	}

	return jobState
}

func stepIdentifier(step model.PlanStep) string {
	if strings.TrimSpace(step.ID) != "" {
		return strings.TrimSpace(step.ID)
	}
	if strings.TrimSpace(step.Name) != "" {
		return strings.TrimSpace(step.Name)
	}
	return "unnamed-step"
}

func unresolvedDependencies(job model.PlanJob, state *State) []string {
	missing := make([]string, 0)
	for _, dep := range job.DependsOn {
		depState, exists := state.Jobs[dep]
		if !exists || depState == nil || depState.Status != "completed" {
			missing = append(missing, dep)
		}
	}
	return missing
}

func (r *Runner) resolveWorkingDir(path string) string {
	if path == "" || path == "./" {
		return r.WorkDir
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(r.WorkDir, path)
}

func topologicalOrder(jobs []model.PlanJob) ([]model.PlanJob, error) {
	jobsByID := make(map[string]model.PlanJob, len(jobs))
	inDegree := make(map[string]int, len(jobs))
	dependents := make(map[string][]string, len(jobs))

	for _, job := range jobs {
		jobsByID[job.ID] = job
		inDegree[job.ID] = 0
		dependents[job.ID] = []string{}
	}

	for _, job := range jobs {
		for _, dep := range job.DependsOn {
			if _, exists := jobsByID[dep]; !exists {
				return nil, fmt.Errorf("job %s depends on unknown job %s", job.ID, dep)
			}
			inDegree[job.ID]++
			dependents[dep] = append(dependents[dep], job.ID)
		}
	}

	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	ordered := make([]model.PlanJob, 0, len(jobs))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		ordered = append(ordered, jobsByID[current])

		for _, dep := range dependents[current] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
		sort.Strings(queue)
	}

	if len(ordered) != len(jobs) {
		return nil, fmt.Errorf("cycle detected in plan jobs")
	}

	return ordered, nil
}
