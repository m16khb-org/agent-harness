package doctor

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/domain/operationalhealth"
)

func HarnessDoctor(req HarnessDoctorRequest) (HarnessDoctorResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
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
	addCheck(&result, "binary", true, "agent-harness command is running")

	stateDoctor, stateDoctorErr := StateDoctor()
	if stateDoctorErr != nil {
		addIssue(&result, "state_doctor_error", "error", "state doctor could not inspect the user-state store", StateDir(), &HarnessDoctorFix{Description: "Check user-state directory permissions or set HARNESS_STATE_DIR to a writable location."})
	} else {
		addCheck(&result, "state_store", stateDoctor.Healthy, stateDoctor.StateDir)
		for _, issue := range stateDoctor.Issues {
			if req.OperationalSnapshot != nil && isUnexpectedStateArtifact(issue.Code) {
				continue
			}
			addIssue(&result, "state_"+issue.Code, issue.Severity, issue.Message, issue.Path, &HarnessDoctorFix{Command: "agent-harness state doctor --json", Description: "Inspect state-store integrity details with the narrow state doctor."})
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
		addCheck(&result, "operational_state", op.Healthy, fmt.Sprintf("findings=%d", len(op.Findings)))
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
			addIssue(&result, finding.Code, severity, summary, finding.Path, &HarnessDoctorFix{Description: "Inspect exact operational identities and reconcile this finding before continuing."})
		}
	}

	lifecycle, err := ValidateProjectLifecycleState(root)
	if err != nil {
		addIssue(&result, "lifecycle_state_error", "error", "project lifecycle state could not be resolved", root, &HarnessDoctorFix{Command: "agent-harness project bootstrap --repo " + shellQuote(root), Description: "Initialize project lifecycle state and repo metadata through project bootstrap."})
	} else {
		result.LifecycleState = lifecycle
		addCheck(&result, "project_lifecycle_state", lifecycle.Exists && lifecycle.NamespaceValid, lifecycle.ProjectStateDir)
		if !lifecycle.Exists {
			addIssue(&result, "lifecycle_state_missing", "warning", "project lifecycle namespace has not been initialized", lifecycle.ProjectStateDir, &HarnessDoctorFix{Command: "agent-harness project bootstrap --repo " + shellQuote(root), Description: "Create the repo-scoped lifecycle namespace and profile metadata in user-state."})
		} else if !lifecycle.NamespaceValid {
			addIssue(&result, "lifecycle_namespace_mismatch", "error", "project lifecycle state fingerprint does not match this repo", lifecycle.ProjectJSONPath, &HarnessDoctorFix{Command: "agent-harness doctor --repo " + shellQuote(root) + " --json", Description: "Review the namespace mismatch before migrating or deleting stale state."})
		}
	}

	checkProjectDocs(&result, root)
	checkRepoLocalRuntimeState(&result, root)
	checkLoopContracts(&result, root)
	if !req.StaticOnly {
		checkPipeCapacity(&result)
		checkMCPGateways(&result, req.Home)
	}
	checkNativeIntegrations(&result, req.Home)
	checkBinaryDrift(&result, req.HarnessRoot)

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

func addCheck(r *HarnessDoctorResult, name string, healthy bool, summary string) {
	r.Checks = append(r.Checks, HarnessDoctorCheck{Name: name, Healthy: healthy, Summary: summary})
}

func addIssue(r *HarnessDoctorResult, code, severity, summary, path string, fix *HarnessDoctorFix) {
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
