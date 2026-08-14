package doctor_test

import (
	"agent-harness/internal/adapter/doctor"
	lifecycle "agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/looprun"
	projectbootstrap "agent-harness/internal/adapter/projectbootstrap"
	loopruncontract "agent-harness/internal/contract/looprun"
	"agent-harness/internal/domain/operationalhealth"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHarnessDoctorJSONIncludesDaemonAdmissionHealth(t *testing.T) {
	repo := t.TempDir()
	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{
		RepoRoot:    repo,
		HarnessRoot: repo,
		Home:        t.TempDir(),
		Version:     "test",
		DaemonAdmission: doctor.HarnessDoctorDaemonAdmission{
			Observed:          true,
			ActiveConnections: 12,
			MaxConnections:    64,
			Accepting:         true,
			Draining:          false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"active_connections", "max_connections", "accepting", "draining"} {
		if _, ok := payload[field]; !ok {
			t.Fatalf("doctor JSON is missing %q: %s", field, raw)
		}
	}
	if result.ActiveConnections != 12 || result.MaxConnections != 64 || !result.Accepting || result.Draining {
		t.Fatalf("doctor lost supplied daemon admission health: %#v", result)
	}
	for _, check := range result.Checks {
		if check.Name == "daemon_admission" {
			if !check.Healthy {
				t.Fatalf("available daemon admission check must be healthy: %+v", check)
			}
			return
		}
	}
	t.Fatalf("daemon admission check missing: %+v", result.Checks)
}

func TestHarnessDoctorMarksSaturatedDaemonUnhealthy(t *testing.T) {
	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{
		RepoRoot:    t.TempDir(),
		HarnessRoot: t.TempDir(),
		Home:        t.TempDir(),
		Version:     "test",
		StaticOnly:  true,
		DaemonAdmission: doctor.HarnessDoctorDaemonAdmission{
			Observed:          true,
			ActiveConnections: 64,
			MaxConnections:    64,
			Accepting:         false,
			Draining:          false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Healthy {
		t.Fatalf("saturated daemon must keep doctor available but unhealthy: %+v", result)
	}
	if !hasHarnessDoctorIssue(result.Issues, "daemon_connection_limit_reached") {
		t.Fatalf("saturated daemon issue missing: %+v", result.Issues)
	}
	for _, check := range result.Checks {
		if check.Name == "daemon_admission" {
			if check.Healthy {
				t.Fatalf("saturated daemon admission check must be unhealthy: %+v", check)
			}
			return
		}
	}
	t.Fatalf("daemon admission check missing: %+v", result.Checks)
}

func TestHarnessDoctorHealthyBaseline(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repo := t.TempDir()
	if _, err := projectbootstrap.BootstrapProjectDocs(projectbootstrap.ProjectDocsBootstrapRequest{RepoRoot: repo, Write: true}); err != nil {
		t.Fatal(err)
	}
	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Kind != "harness_doctor" {
		t.Fatalf("unexpected doctor result: %+v", result)
	}
	if hasHarnessDoctorIssue(result.Issues, "repo_local_state_present") || hasHarnessDoctorIssue(result.Issues, "lifecycle_namespace_mismatch") {
		t.Fatalf("unexpected serious project issue: %+v", result.Issues)
	}
	if _, err := os.Stat(filepath.Join(repo, ".agent-harness", "VCS.md")); !os.IsNotExist(err) {
		t.Fatalf("bootstrap unexpectedly created optional VCS.md: %v", err)
	}
	if hasHarnessDoctorIssue(result.Issues, "project_docs_missing") {
		t.Fatalf("doctor treated optional VCS.md as required: %+v", result.Issues)
	}
}

func TestHarnessDoctorProjectsOperationalFinding(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	snapshot := healthyDoctorOperationalSnapshot(repo)
	snapshot.Gates = []operationalhealth.OrcaGate{{ID: "gate-1", Status: "pending"}}

	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{
		RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test",
		OperationalSnapshot: &snapshot,
		OperationalOptions:  operationalhealth.Options{Now: doctorOperationalNow()},
	})
	if err != nil {
		t.Fatal(err)
	}

	check, ok := harnessDoctorCheck(result.Checks, "operational_state")
	if !ok || check.Healthy || harnessDoctorCheckCount(result.Checks, "operational_state") != 1 {
		t.Fatalf("operational check projection = %#v", result.Checks)
	}
	issue, ok := harnessDoctorIssue(result.Issues, operationalhealth.FindingGateResidue)
	if !ok || issue.Severity != "warning" || !strings.Contains(issue.Summary, "gate-1") || issue.Fix == nil || issue.Fix.Destructive || issue.Fix.Command != "" {
		t.Fatalf("operational issue projection = %#v", issue)
	}
}

func TestHarnessDoctorOperationalInventoryProblemIsError(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	snapshot := healthyDoctorOperationalSnapshot(repo)
	snapshot.InventoryProblems = []operationalhealth.InventoryProblem{{
		Source: "orca_tasks", Code: "orca_tasks_failed", Detail: "task inventory failed",
	}}

	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{
		RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test",
		OperationalSnapshot: &snapshot,
		OperationalOptions:  operationalhealth.Options{Now: doctorOperationalNow()},
	})
	if err != nil {
		t.Fatal(err)
	}

	issue, ok := harnessDoctorIssue(result.Issues, operationalhealth.FindingInventoryUnknown)
	if result.Healthy || !ok || issue.Severity != "error" {
		t.Fatalf("inventory problem projection = healthy=%v issue=%#v all=%#v", result.Healthy, issue, result.Issues)
	}
}

func TestHarnessDoctorProjectsStateArtifactsWithoutLegacyDuplicates(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	unexpectedFile := filepath.Join(stateRoot, "recovery.patch")
	unexpectedDirectory := filepath.Join(stateRoot, "legacy-recovery")
	mustWrite(t, unexpectedFile, "recovery evidence")
	if err := os.MkdirAll(unexpectedDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	snapshot := healthyDoctorOperationalSnapshot(repo)
	snapshot.StateArtifacts = make([]operationalhealth.StateArtifact, 1, 4)
	snapshot.StateArtifacts[0] = operationalhealth.StateArtifact{Path: unexpectedFile, Code: "unexpected_file"}
	before := append(make([]operationalhealth.StateArtifact, 0, len(snapshot.StateArtifacts)), snapshot.StateArtifacts...)

	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{
		RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test",
		OperationalSnapshot: &snapshot,
		OperationalOptions:  operationalhealth.Options{Now: doctorOperationalNow()},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(snapshot.StateArtifacts, before) {
		t.Fatalf("doctor mutated injected snapshot: before=%#v after=%#v", before, snapshot.StateArtifacts)
	}
	if hasHarnessDoctorIssue(result.Issues, "state_unexpected_file") || hasHarnessDoctorIssue(result.Issues, "state_unexpected_directory") {
		t.Fatalf("legacy state issues duplicated operational residue: %#v", result.Issues)
	}
	if countHarnessDoctorIssues(result.Issues, operationalhealth.FindingStateArtifactResidue) != 2 {
		t.Fatalf("state artifact projection = %#v", result.Issues)
	}
}

func TestHarnessDoctorNilOperationalSnapshotPreservesLegacyStateIssues(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	mustWrite(t, filepath.Join(stateRoot, "recovery.patch"), "recovery evidence")
	repo := t.TempDir()

	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}

	if !hasHarnessDoctorIssue(result.Issues, "state_unexpected_file") || harnessDoctorCheckCount(result.Checks, "operational_state") != 0 {
		t.Fatalf("nil operational snapshot changed legacy behavior: checks=%#v issues=%#v", result.Checks, result.Issues)
	}
}

func TestHarnessDoctorReportsRepoLocalRuntimeState(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, ".agent-harness", "state", "live.json"), `{"x":1}`)
	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasHarnessDoctorIssue(result.Issues, "repo_local_state_present") {
		t.Fatalf("expected repo local state issue: %+v", result.Issues)
	}
}

func TestHarnessDoctorReportsLifecycleNamespaceMismatch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	plan, err := lifecycle.InitProjectLifecycleState(repo, true)
	if err != nil {
		t.Fatal(err)
	}
	var profile lifecycle.ProjectLifecycleProfile
	b, err := os.ReadFile(plan.ProjectJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &profile); err != nil {
		t.Fatal(err)
	}
	profile.Fingerprint.RepoRoot = t.TempDir()
	b, err = json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.ProjectJSONPath, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasHarnessDoctorIssue(result.Issues, "lifecycle_namespace_mismatch") {
		t.Fatalf("expected namespace mismatch issue: %+v", result.Issues)
	}
}

func TestHarnessDoctorReportsLoopContracts(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := projectbootstrap.BootstrapProjectDocs(projectbootstrap.ProjectDocsBootstrapRequest{RepoRoot: repo, Write: true}); err != nil {
		t.Fatal(err)
	}
	loop, err := looprun.Start(loopruncontract.StartLoopRequest{
		Repo:        repo,
		Name:        "doctor-loop",
		Goal:        "verify doctor loop contract reporting",
		MaxAttempts: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	check, ok := harnessDoctorCheck(result.Checks, "loop_contracts")
	if !ok || check.Healthy || check.Summary != "active=1 exhausted=0" {
		t.Fatalf("expected active loop contract check, got check=%+v ok=%v result=%+v", check, ok, result)
	}
	if !hasHarnessDoctorIssue(result.Issues, "loop_contracts_incomplete") {
		t.Fatalf("expected loop contract issue for %s: %+v", loop.ID, result.Issues)
	}
}

func TestHarnessDoctorLoopContractsHealthyWhenNoIncompleteLoops(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := projectbootstrap.BootstrapProjectDocs(projectbootstrap.ProjectDocsBootstrapRequest{RepoRoot: repo, Write: true}); err != nil {
		t.Fatal(err)
	}
	result, err := doctor.HarnessDoctor(doctor.HarnessDoctorRequest{RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	check, ok := harnessDoctorCheck(result.Checks, "loop_contracts")
	if !ok || !check.Healthy || check.Summary != "active=0 exhausted=0" {
		t.Fatalf("expected healthy loop contract check, got check=%+v ok=%v result=%+v", check, ok, result)
	}
}

func harnessDoctorCheck(checks []doctor.HarnessDoctorCheck, name string) (doctor.HarnessDoctorCheck, bool) {
	for _, check := range checks {
		if check.Name == name {
			return check, true
		}
	}
	return doctor.HarnessDoctorCheck{}, false
}

func hasHarnessDoctorIssue(issues []doctor.HarnessDoctorIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func harnessDoctorIssue(issues []doctor.HarnessDoctorIssue, code string) (doctor.HarnessDoctorIssue, bool) {
	for _, issue := range issues {
		if issue.Code == code {
			return issue, true
		}
	}
	return doctor.HarnessDoctorIssue{}, false
}

func countHarnessDoctorIssues(issues []doctor.HarnessDoctorIssue, code string) int {
	count := 0
	for _, issue := range issues {
		if issue.Code == code {
			count++
		}
	}
	return count
}

func harnessDoctorCheckCount(checks []doctor.HarnessDoctorCheck, name string) int {
	count := 0
	for _, check := range checks {
		if check.Name == name {
			count++
		}
	}
	return count
}

func doctorOperationalNow() time.Time {
	return time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
}

func healthyDoctorOperationalSnapshot(repo string) operationalhealth.Snapshot {
	return operationalhealth.Snapshot{
		RepoRoot: repo, CanonicalBranch: "main", SourceHead: "head-main", SourceClean: true,
		GitWorktrees:  []operationalhealth.GitWorktree{{Path: repo, Branch: "main", Head: "head-main", Clean: true, Canonical: true}},
		LocalRefs:     []operationalhealth.GitRef{{Name: "refs/heads/main", Branch: "main", OID: "head-main", Location: "local"}},
		RemoteRefs:    []operationalhealth.GitRef{{Name: "refs/heads/main", Branch: "main", OID: "head-main", Location: "remote"}},
		OrcaWorktrees: []operationalhealth.OrcaWorktree{{ID: "wt-main", InstanceID: "instance-main", Repo: repo, Path: repo, Branch: "main", Head: "head-main"}},
		Messages:      operationalhealth.MessagePresence{Empty: true, CompleteAbsence: true},
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
