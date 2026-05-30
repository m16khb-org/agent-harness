package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type HarnessDoctorRequest struct {
	RepoRoot    string `json:"repo_root,omitempty"`
	HarnessRoot string `json:"harness_root,omitempty"`
	Home        string `json:"home,omitempty"`
	Version     string `json:"version,omitempty"`
}

type HarnessDoctorResult struct {
	OK             bool                      `json:"ok"`
	Healthy        bool                      `json:"healthy"`
	Kind           string                    `json:"kind"`
	Version        string                    `json:"version,omitempty"`
	HarnessRoot    string                    `json:"harness_root,omitempty"`
	RepoRoot       string                    `json:"repo_root"`
	StateDir       string                    `json:"state_dir"`
	LifecycleState ProjectLifecycleStatePlan `json:"lifecycle_state"`
	Checks         []HarnessDoctorCheck      `json:"checks"`
	Issues         []HarnessDoctorIssue      `json:"issues"`
	GeneratedAt    string                    `json:"generated_at"`
}

type HarnessDoctorCheck struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Summary string `json:"summary"`
}

type HarnessDoctorIssue struct {
	Code     string            `json:"code"`
	Severity string            `json:"severity"`
	Summary  string            `json:"summary"`
	Path     string            `json:"path,omitempty"`
	Fix      *HarnessDoctorFix `json:"fix,omitempty"`
}

type HarnessDoctorFix struct {
	Command     string `json:"command,omitempty"`
	Destructive bool   `json:"destructive"`
	Description string `json:"description"`
}

func HarnessDoctor(req HarnessDoctorRequest) (HarnessDoctorResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return HarnessDoctorResult{OK: false, Kind: "harness_doctor", StateDir: StateDir()}, err
	}
	result := HarnessDoctorResult{
		OK:          true,
		Healthy:     true,
		Kind:        "harness_doctor",
		Version:     req.Version,
		HarnessRoot: req.HarnessRoot,
		RepoRoot:    root,
		StateDir:    StateDir(),
		Checks:      []HarnessDoctorCheck{},
		Issues:      []HarnessDoctorIssue{},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	result.addCheck("binary", true, "agent-harness command is running")

	stateDoctor, err := StateDoctor()
	if err != nil {
		result.addIssue("state_doctor_error", "error", "state doctor could not inspect the user-state store", StateDir(), &HarnessDoctorFix{Description: "Check user-state directory permissions or set HARNESS_STATE_DIR to a writable location."})
	} else {
		result.addCheck("state_store", stateDoctor.Healthy, stateDoctor.StateDir)
		for _, issue := range stateDoctor.Issues {
			result.addIssue("state_"+issue.Code, issue.Severity, issue.Message, issue.Path, &HarnessDoctorFix{Command: "agent-harness state doctor --json", Description: "Inspect state-store integrity details with the narrow state doctor."})
		}
	}

	lifecycle, err := ValidateProjectLifecycleState(root)
	if err != nil {
		result.addIssue("lifecycle_state_error", "error", "project lifecycle state could not be resolved", root, &HarnessDoctorFix{Command: "agent-harness project bootstrap --repo " + shellQuote(root), Description: "Initialize project lifecycle state and repo metadata through project bootstrap."})
	} else {
		result.LifecycleState = lifecycle
		result.addCheck("project_lifecycle_state", lifecycle.Exists && lifecycle.NamespaceValid, lifecycle.ProjectStateDir)
		if !lifecycle.Exists {
			result.addIssue("lifecycle_state_missing", "warning", "project lifecycle namespace has not been initialized", lifecycle.ProjectStateDir, &HarnessDoctorFix{Command: "agent-harness project bootstrap --repo " + shellQuote(root), Description: "Create the repo-scoped lifecycle namespace and profile metadata in user-state."})
		} else if !lifecycle.NamespaceValid {
			result.addIssue("lifecycle_namespace_mismatch", "error", "project lifecycle state fingerprint does not match this repo", lifecycle.ProjectJSONPath, &HarnessDoctorFix{Command: "agent-harness doctor --repo " + shellQuote(root) + " --json", Description: "Review the namespace mismatch before migrating or deleting stale state."})
		}
	}

	result.checkProjectDocs(root)
	result.checkRepoLocalRuntimeState(root)
	result.checkNativeIntegrations(req.Home)

	sort.Slice(result.Issues, func(i, j int) bool {
		if result.Issues[i].Severity != result.Issues[j].Severity {
			return severityRank(result.Issues[i].Severity) < severityRank(result.Issues[j].Severity)
		}
		if result.Issues[i].Code != result.Issues[j].Code {
			return result.Issues[i].Code < result.Issues[j].Code
		}
		return result.Issues[i].Path < result.Issues[j].Path
	})
	result.Healthy = true
	for _, issue := range result.Issues {
		if issue.Severity == "error" || issue.Severity == "warning" {
			result.Healthy = false
			break
		}
	}
	return result, nil
}

func (r *HarnessDoctorResult) addCheck(name string, healthy bool, summary string) {
	r.Checks = append(r.Checks, HarnessDoctorCheck{Name: name, Healthy: healthy, Summary: summary})
}

func (r *HarnessDoctorResult) addIssue(code, severity, summary, path string, fix *HarnessDoctorFix) {
	r.Issues = append(r.Issues, HarnessDoctorIssue{Code: code, Severity: severity, Summary: summary, Path: path, Fix: fix})
}

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

func severityRank(severity string) int {
	switch severity {
	case "error":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
