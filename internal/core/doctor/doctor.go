package doctor

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/domain/operationalhealth"
	"agent-harness/internal/core/repopath"
)

type HarnessDoctorRequest struct {
	RepoRoot            string                       `json:"repo_root,omitempty"`
	HarnessRoot         string                       `json:"harness_root,omitempty"`
	Home                string                       `json:"home,omitempty"`
	Version             string                       `json:"version,omitempty"`
	DaemonAdmission     HarnessDoctorDaemonAdmission `json:"daemon_admission,omitempty"`
	OperationalSnapshot *operationalhealth.Snapshot  `json:"-"`
	OperationalOptions  operationalhealth.Options    `json:"-"`
}

type HarnessDoctorDaemonAdmission struct {
	ActiveConnections int  `json:"active_connections"`
	MaxConnections    int  `json:"max_connections"`
	Accepting         bool `json:"accepting"`
	Draining          bool `json:"draining"`
}

type HarnessDoctorResult struct {
	OK                bool                      `json:"ok"`
	Healthy           bool                      `json:"healthy"`
	Kind              string                    `json:"kind"`
	Version           string                    `json:"version,omitempty"`
	HarnessRoot       string                    `json:"harness_root,omitempty"`
	RepoRoot          string                    `json:"repo_root"`
	StateDir          string                    `json:"state_dir"`
	LifecycleState    ProjectLifecycleStatePlan `json:"lifecycle_state"`
	PipeCapacityBytes int                       `json:"pipe_capacity_bytes"`
	ActiveConnections int                       `json:"active_connections"`
	MaxConnections    int                       `json:"max_connections"`
	Accepting         bool                      `json:"accepting"`
	Draining          bool                      `json:"draining"`
	Checks            []HarnessDoctorCheck      `json:"checks"`
	Issues            []HarnessDoctorIssue      `json:"issues"`
	GeneratedAt       string                    `json:"generated_at"`
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
	root, err := repopath.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return HarnessDoctorResult{OK: false, Kind: "harness_doctor", StateDir: StateDir()}, err
	}
	result := HarnessDoctorResult{
		OK:                true,
		Healthy:           true,
		Kind:              "harness_doctor",
		Version:           req.Version,
		HarnessRoot:       req.HarnessRoot,
		RepoRoot:          root,
		StateDir:          StateDir(),
		ActiveConnections: req.DaemonAdmission.ActiveConnections,
		MaxConnections:    req.DaemonAdmission.MaxConnections,
		Accepting:         req.DaemonAdmission.Accepting,
		Draining:          req.DaemonAdmission.Draining,
		Checks:            []HarnessDoctorCheck{},
		Issues:            []HarnessDoctorIssue{},
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	result.addCheck("binary", true, "agent-harness command is running")

	stateDoctor, stateDoctorErr := StateDoctor()
	if stateDoctorErr != nil {
		result.addIssue("state_doctor_error", "error", "state doctor could not inspect the user-state store", StateDir(), &HarnessDoctorFix{Description: "Check user-state directory permissions or set HARNESS_STATE_DIR to a writable location."})
	} else {
		result.addCheck("state_store", stateDoctor.Healthy, stateDoctor.StateDir)
		for _, issue := range stateDoctor.Issues {
			if req.OperationalSnapshot != nil && isUnexpectedStateArtifact(issue.Code) {
				continue
			}
			result.addIssue("state_"+issue.Code, issue.Severity, issue.Message, issue.Path, &HarnessDoctorFix{Command: "agent-harness state doctor --json", Description: "Inspect state-store integrity details with the narrow state doctor."})
		}
	}
	if req.OperationalSnapshot != nil {
		snapshot := cloneOperationalSnapshot(*req.OperationalSnapshot)
		if stateDoctorErr == nil {
			for _, issue := range stateDoctor.Issues {
				if isUnexpectedStateArtifact(issue.Code) {
					snapshot.StateArtifacts = append(snapshot.StateArtifacts, operationalhealth.StateArtifact{Path: issue.Path, Code: issue.Code})
				}
			}
		}
		op := operationalhealth.Classify(snapshot, req.OperationalOptions)
		result.addCheck("operational_state", op.Healthy, fmt.Sprintf("findings=%d", len(op.Findings)))
		for _, finding := range op.Findings {
			severity := "warning"
			if finding.Code == operationalhealth.FindingInventoryUnknown {
				severity = "error"
			}
			summary := finding.Summary
			if resourceID := strings.TrimSpace(finding.ResourceID); resourceID != "" {
				resourceKind := strings.TrimSpace(finding.ResourceKind)
				if resourceKind == "" {
					resourceKind = "resource"
				}
				summary = fmt.Sprintf("%s %s: %s", resourceKind, resourceID, summary)
			}
			result.addIssue(finding.Code, severity, summary, finding.Path, &HarnessDoctorFix{Description: "Inspect exact operational identities and reconcile this finding before continuing."})
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
	result.checkLoopContracts(root)
	result.checkPipeCapacity()
	result.checkMCPGateways(req.Home)
	result.checkNativeIntegrations(req.Home)
	result.checkBinaryDrift(req.HarnessRoot)

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

func isUnexpectedStateArtifact(code string) bool {
	return code == "unexpected_file" || code == "unexpected_directory"
}

func cloneOperationalSnapshot(snapshot operationalhealth.Snapshot) operationalhealth.Snapshot {
	snapshot.Cycles = append([]operationalhealth.Cycle(nil), snapshot.Cycles...)
	snapshot.GitWorktrees = append([]operationalhealth.GitWorktree(nil), snapshot.GitWorktrees...)
	snapshot.LocalRefs = append([]operationalhealth.GitRef(nil), snapshot.LocalRefs...)
	snapshot.RemoteRefs = append([]operationalhealth.GitRef(nil), snapshot.RemoteRefs...)
	snapshot.OrcaWorktrees = append([]operationalhealth.OrcaWorktree(nil), snapshot.OrcaWorktrees...)
	snapshot.Terminals = append([]operationalhealth.OrcaTerminal(nil), snapshot.Terminals...)
	snapshot.Tasks = append([]operationalhealth.OrcaTask(nil), snapshot.Tasks...)
	snapshot.Dispatches = append([]operationalhealth.OrcaDispatch(nil), snapshot.Dispatches...)
	snapshot.Gates = append([]operationalhealth.OrcaGate(nil), snapshot.Gates...)
	snapshot.StateArtifacts = append([]operationalhealth.StateArtifact(nil), snapshot.StateArtifacts...)
	snapshot.InventoryProblems = append([]operationalhealth.InventoryProblem(nil), snapshot.InventoryProblems...)
	return snapshot
}
