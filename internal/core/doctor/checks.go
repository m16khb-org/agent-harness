package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
