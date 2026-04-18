package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sourceplane/arx/internal/model"
	"github.com/sourceplane/arx/internal/ui"
)

var (
	restoreFromCachePattern = regexp.MustCompile(`(?i)^Restoring ['"]?([^'"]+)['"]? from cache$`)
	toolCachedPattern      = regexp.MustCompile(`(?i)^([A-Za-z0-9._ -]+) tool version '([^']+)' has been cached at .+$`)
	toolDownloadPattern    = regexp.MustCompile(`(?i)^Downloading ['"]?([^'"]+)['"]? from .+$`)
)

func (r *Runner) shouldPrintPreflight(jobs []model.PlanJob) bool {
	if r.DryRun {
		return true
	}
	if r.JobID != "" {
		return false
	}
	return len(jobs) > 1
}

func (r *Runner) shouldPrintRunSummary(summary *runSummary) bool {
	if summary == nil {
		return false
	}
	if r.DryRun {
		return true
	}
	if summary.failed > 0 || summary.waiting > 0 || summary.skipped > 0 {
		return true
	}
	return summary.completed > 1
}

func (r *Runner) printStepStart(stepID string, index, total int) {
	fmt.Fprintf(r.Stdout, "\n  %s %s (%d/%d)\n", ui.Cyan(r.Color, "●"), ui.Bold(r.Color, stepID), index, total)
}

func (r *Runner) printStepContext(step model.PlanStep, workingDir, timeoutValue string, retryCount int) {
	meta := []string{
		fmt.Sprintf("runner: %s", displayRunnerName(r.Executor.Name())),
		fmt.Sprintf("cwd: %s", shortenDisplayLine(workingDir)),
	}
	fmt.Fprintf(r.Stdout, "  │ %s\n", strings.Join(meta, "   "))

	if strings.TrimSpace(step.Use) != "" {
		fmt.Fprintf(r.Stdout, "  │ use: %s\n", strings.TrimSpace(step.Use))
	}
	if strings.TrimSpace(step.Run) != "" {
		fmt.Fprintln(r.Stdout, "  │ run:")
		for _, line := range formatCommandPreview(step.Run) {
			fmt.Fprintf(r.Stdout, "  │   %s\n", line)
		}
	}

	settings := make([]string, 0, 2)
	if retryCount > 0 {
		settings = append(settings, fmt.Sprintf("retries: %d", retryCount))
	}
	if timeoutValue != "" {
		settings = append(settings, fmt.Sprintf("timeout: %s", timeoutValue))
	}
	if len(settings) > 0 {
		fmt.Fprintf(r.Stdout, "  │ %s\n", strings.Join(settings, "   "))
	}
}

func (r *Runner) printStepRetry(attempt, attempts int) {
	fmt.Fprintf(r.Stdout, "  │ %s attempt %d/%d\n", ui.Yellow(r.Color, "↻"), attempt, attempts)
}

func (r *Runner) printStepDryRun() {
	fmt.Fprintf(r.Stdout, "  %s dry-run\n", ui.Cyan(r.Color, "◌"))
}

func (r *Runner) printStepSkipped(stepID string, index, total int) {
	fmt.Fprintf(r.Stdout, "  %s %s (%d/%d) already completed\n", ui.Yellow(r.Color, "↷"), stepID, index, total)
}

func (r *Runner) printStepSuccess(step model.PlanStep, output string, duration time.Duration) {
	summary, result, logs := renderStepOutput(step, output)
	printed := false
	if len(summary) > 0 {
		r.printBlock("summary", summary)
		printed = true
	}
	if len(result) > 0 {
		r.printBlock("result", result)
		printed = true
	}
	if len(logs) > 0 {
		if r.Verbose {
			r.printBlock("logs", logs)
		} else {
			fmt.Fprintln(r.Stdout, "  │")
			fmt.Fprintf(r.Stdout, "  │ logs: %s\n", ui.Dim(r.Color, "(collapsed; use --verbose to expand)"))
		}
		printed = true
	}
	if printed {
		fmt.Fprintln(r.Stdout, "  │")
	}
	fmt.Fprintf(r.Stdout, "  %s completed in %s\n", ui.Green(r.Color, "✓"), formatStepDuration(duration))
}

func (r *Runner) printStepFailure(step model.PlanStep, output string, duration time.Duration, err error, workingDir string) {
	summary, result, logs := renderStepOutput(step, output)
	printed := false
	if len(summary) > 0 {
		r.printBlock("summary", summary)
		printed = true
	}
	if len(result) > 0 {
		r.printBlock("result", result)
		printed = true
	}
	if len(logs) > 0 {
		r.printBlock("logs", logs)
		printed = true
	}
	if printed {
		fmt.Fprintln(r.Stdout, "  │")
	}
	fmt.Fprintf(r.Stdout, "  %s failed in %s: %s\n", ui.Red(r.Color, "✗"), formatStepDuration(duration), ui.Red(r.Color, summarizeExecError(err)))
	if hint := stepFailureHint(err, output, workingDir); hint != "" {
		fmt.Fprintf(r.Stdout, "  %s %s\n", ui.Yellow(r.Color, "hint:"), hint)
	}
}

func (r *Runner) printStepContinuation() {
	fmt.Fprintf(r.Stdout, "  %s continuing (onFailure=continue)\n", ui.Yellow(r.Color, "↷"))
}

func (r *Runner) printJobFooter(success bool) {
	status := ui.Green(r.Color, "✓")
	message := "Job completed"
	if !success {
		status = ui.Red(r.Color, "✗")
		message = "Job failed"
	}
	fmt.Fprintf(r.Stdout, "%s %s %s\n", ui.Cyan(r.Color, "╰─"), status, message)
}

func (r *Runner) printBlock(title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintln(r.Stdout, "  │")
	fmt.Fprintf(r.Stdout, "  │ %s:\n", title)
	for _, line := range lines {
		fmt.Fprintf(r.Stdout, "  │   %s\n", line)
	}
}

func renderStepOutput(step model.PlanStep, output string) ([]string, []string, []string) {
	lines := splitDisplayLines(output)
	if len(lines) == 0 {
		return nil, nil, nil
	}
	if strings.TrimSpace(step.Use) != "" {
		return summarizeUseOutput(lines), nil, lines
	}
	return nil, lines, nil
}

func summarizeUseOutput(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}

	installed := ""
	cached := ""
	downloaded := ""

	for _, line := range lines {
		if match := toolCachedPattern.FindStringSubmatch(line); match != nil {
			installed = fmt.Sprintf("Installed %s %s", strings.ToLower(strings.TrimSpace(match[1])), match[2])
			continue
		}
		if restoreFromCachePattern.MatchString(line) {
			cached = "Cached locally"
			continue
		}
		if match := toolDownloadPattern.FindStringSubmatch(line); match != nil {
			downloaded = fmt.Sprintf("Downloaded %s", match[1])
		}
	}

	summary := make([]string, 0, 3)
	for _, line := range []string{installed, cached, downloaded} {
		if strings.TrimSpace(line) != "" {
			summary = append(summary, line)
		}
	}
	if len(summary) > 0 {
		return summary
	}

	limit := len(lines)
	if limit > 3 {
		limit = 3
	}
	return append([]string{}, lines[:limit]...)
}

func splitDisplayLines(output string) []string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil
	}

	lines := make([]string, 0)
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, shortenDisplayLine(strings.TrimSpace(line)))
	}
	return lines
}

func formatCommandPreview(command string) []string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return nil
	}

	lines := make([]string, 0)
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func formatStepDuration(duration time.Duration) string {
	rounded := duration.Round(100 * time.Millisecond)
	if rounded < time.Minute {
		return fmt.Sprintf("%.1fs", rounded.Seconds())
	}
	minutes := int(rounded / time.Minute)
	seconds := rounded % time.Minute
	if seconds%time.Second == 0 {
		return fmt.Sprintf("%dm%ds", minutes, int(seconds/time.Second))
	}
	return fmt.Sprintf("%dm%.1fs", minutes, seconds.Seconds())
}

func displayRunnerName(name string) string {
	switch strings.TrimSpace(name) {
	case "github-actions":
		return "gha"
	default:
		return strings.TrimSpace(name)
	}
}

func shortenDisplayLine(line string) string {
	homeDir, err := os.UserHomeDir()
	if err == nil && homeDir != "" {
		line = strings.ReplaceAll(line, homeDir, "~")
	}
	if prefix, suffix, ok := strings.Cut(line, " at "); ok && isLikelyPath(suffix) {
		return prefix + " at " + shortenPathDisplay(suffix)
	}
	if isLikelyPath(line) {
		return shortenPathDisplay(line)
	}
	return line
}

func shortenPathDisplay(value string) string {
	if value == "" {
		return value
	}

	cleaned := filepath.Clean(value)
	separator := string(filepath.Separator)
	parts := strings.Split(cleaned, separator)
	if len(cleaned) <= 64 && len(parts) <= 7 {
		return cleaned
	}

	prefixCount := 5
	if parts[0] == "" {
		prefixCount = 4
	}
	if len(parts) <= prefixCount+1 {
		return cleaned
	}

	prefix := parts[:prefixCount]
	suffix := parts[len(parts)-1:]
	if parts[0] == "" {
		return separator + strings.Join(append(prefix[1:], append([]string{"..."}, suffix...)...), separator)
	}
	return strings.Join(append(prefix, append([]string{"..."}, suffix...)...), separator)
}

func isLikelyPath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t") {
		return false
	}
	if strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "../") || strings.HasPrefix(trimmed, "/") {
		return true
	}
	return strings.Contains(trimmed, string(filepath.Separator))
}