package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"agent-harness/internal/core/looprun"
)

const pipeCapacityWarningThreshold = 8192

var measurePipeCapacity = measureSystemPipeCapacity

func (r *HarnessDoctorResult) checkProjectDocs(root string) {
	missing := []string{}
	for _, name := range ProjectDocNames() {
		if _, err := os.Stat(filepath.Join(root, ProjectDocsDir, name)); os.IsNotExist(err) {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		r.addCheck("project_docs", true, "all standard .agent-harness docs exist")
		return
	}
	r.addCheck("project_docs", false, strings.Join(missing, ", "))
	r.addIssue("project_docs_missing", "warning", "standard .agent-harness docs are missing", filepath.Join(root, ProjectDocsDir), &HarnessDoctorFix{Command: "agent-harness project bootstrap --repo " + shellQuote(root), Description: "Create or refresh the standard project guidance docs and profile metadata."})
}

func (r *HarnessDoctorResult) checkRepoLocalRuntimeState(root string) {
	candidates := []string{
		filepath.Join(root, ProjectDocsDir, "state"),
		filepath.Join(root, ProjectDocsDir, "state.schema.json"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			r.addIssue("repo_local_state_present", "warning", "repo-local lifecycle runtime or schema state should not be committed in team repositories", path, &HarnessDoctorFix{Description: "Move runtime state to the user-state project namespace and ensure repo-local state paths are ignored or removed."})
		}
	}
	stateMD := filepath.Join(root, ProjectDocsDir, "STATE.md")
	if b, err := os.ReadFile(stateMD); err == nil {
		lower := strings.ToLower(string(b))
		if strings.Contains(lower, "schema") || strings.Contains(lower, "runtime state") || strings.Contains(lower, "jsonl") {
			r.addIssue("repo_local_state_present", "warning", "STATE.md appears to describe runtime/schema state rather than shared project knowledge", stateMD, &HarnessDoctorFix{Description: "Keep lifecycle schemas in agent-harness core and runtime state in user-state, not target repo docs."})
		}
	}
}

func (r *HarnessDoctorResult) checkLoopContracts(root string) {
	summary, warnings := looprun.RepoGateSummaryFor(root)
	incomplete := summary.Active + summary.Exhausted
	r.addCheck("loop_contracts", incomplete == 0 && len(warnings) == 0, fmt.Sprintf("active=%d exhausted=%d", summary.Active, summary.Exhausted))
	if len(warnings) > 0 {
		r.addIssue("loop_contracts_unreadable", "warning", strings.Join(warnings, "; "), looprun.StateRoot(), &HarnessDoctorFix{Command: "agent-harness loop status --id <loop-id> --json", Description: "Inspect loop state records before PR readiness."})
		return
	}
	if incomplete > 0 {
		r.addIssue("loop_contracts_incomplete", "warning", fmt.Sprintf("repo has incomplete loop contracts: active=%d exhausted=%d", summary.Active, summary.Exhausted), looprun.StateRoot(), &HarnessDoctorFix{Command: "agent-harness loop status --id <loop-id> --json", Description: "Stop or complete same-repo loop runs before PR readiness."})
	}
}

func (r *HarnessDoctorResult) checkPipeCapacity() {
	capacity, err := measurePipeCapacity()
	if err != nil {
		r.addCheck("pipe_capacity", true, "pipe capacity unavailable: "+err.Error())
		r.addIssue("pipe_capacity_unavailable", "warning", "system pipe buffer capacity could not be measured", "", &HarnessDoctorFix{Description: "Retry doctor; if this persists, inspect process file descriptors and OS pipe limits."})
		return
	}
	r.PipeCapacityBytes = capacity
	summary := fmt.Sprintf("capacity=%d bytes", capacity)
	if capacity < pipeCapacityWarningThreshold {
		r.addCheck("pipe_capacity", false, summary)
		r.addIssue("pipe_capacity_degraded", "warning", "system pipe buffer degraded; long-lived host process may be leaking pipes; see CAUTIONS 2026-07-09", "", &HarnessDoctorFix{Description: "Restart the leaking long-lived host process, then rerun lsof pipe counts and agent-harness doctor."})
		return
	}
	r.addCheck("pipe_capacity", true, summary)
}

func measureSystemPipeCapacity() (int, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return 0, err
	}
	defer r.Close()
	defer w.Close()

	progress := make(chan int, 64)
	done := make(chan error, 1)
	total := 0
	go func() {
		chunk := make([]byte, 512)
		for {
			n, err := w.Write(chunk)
			if n > 0 {
				progress <- n
			}
			if err != nil {
				done <- err
				return
			}
		}
	}()

	idle := time.NewTimer(100 * time.Millisecond)
	defer idle.Stop()
	for total < 1<<20 {
		select {
		case n := <-progress:
			total += n
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(100 * time.Millisecond)
		case err := <-done:
			if errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EPIPE) {
				return total, nil
			}
			return total, err
		case <-idle.C:
			_ = r.Close()
			_ = w.Close()
			select {
			case err := <-done:
				if err != nil && !errors.Is(err, os.ErrClosed) && !errors.Is(err, syscall.EPIPE) {
					return total, err
				}
			case <-time.After(time.Second):
			}
			return total, nil
		}
	}
	return total, nil
}

func (r *HarnessDoctorResult) checkNativeIntegrations(home string) {
	if home == "" {
		r.addCheck("native_integrations", true, "home directory unavailable; skipped user-level integration checks")
		return
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "hooks.json")); os.IsNotExist(err) {
		r.addIssue("codex_hooks_missing", "warning", "Codex hooks.json is not present", filepath.Join(home, ".codex", "hooks.json"), &HarnessDoctorFix{Command: "agent-harness install-native", Description: "Install user-level hooks, skills, and MCP configuration."})
	}
	r.addCheck("native_integrations", true, "checked user-level integration paths")
}

func (r *HarnessDoctorResult) checkBinaryDrift(harnessRoot string) {
	if harnessRoot == "" {
		return
	}
	binPath := filepath.Join(harnessRoot, "bin", "agent-harness")
	binInfo, err := os.Stat(binPath)
	if err != nil {
		r.addCheck("binary_drift", true, "no prebuilt bin/agent-harness found; skipping drift check")
		return
	}
	binTime := binInfo.ModTime()
	latestSourceTime := binTime
	sourceDirs := []string{
		filepath.Join(harnessRoot, "cmd"),
		filepath.Join(harnessRoot, "internal"),
	}
	for _, dir := range sourceDirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".go") {
				return nil
			}
			if info.ModTime().After(latestSourceTime) {
				latestSourceTime = info.ModTime()
			}
			return nil
		})
	}
	if latestSourceTime.After(binTime) {
		delta := latestSourceTime.Sub(binTime).Round(time.Second)
		r.addCheck("binary_drift", false, fmt.Sprintf("bin/agent-harness is %s older than latest source change", delta))
		r.addIssue("binary_drift", "warning", fmt.Sprintf("bin/agent-harness may be stale (%s older than source)", delta), binPath, &HarnessDoctorFix{Command: "go build -o bin/agent-harness ./cmd/harness", Description: "Rebuild the agent-harness binary from the current source."})
	} else {
		r.addCheck("binary_drift", true, "bin/agent-harness is current")
	}
}
