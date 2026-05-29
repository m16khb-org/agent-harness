package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHarnessDoctorHealthyBaseline(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repo := t.TempDir()
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: repo, Write: true}); err != nil {
		t.Fatal(err)
	}
	result, err := HarnessDoctor(HarnessDoctorRequest{RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Kind != "harness_doctor" {
		t.Fatalf("unexpected doctor result: %+v", result)
	}
	if hasHarnessDoctorIssue(result.Issues, "repo_local_state_present") || hasHarnessDoctorIssue(result.Issues, "lifecycle_namespace_mismatch") {
		t.Fatalf("unexpected serious project issue: %+v", result.Issues)
	}
}

func TestHarnessDoctorReportsRepoLocalRuntimeState(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, ".agent-harness", "state", "live.json"), `{"x":1}`)
	result, err := HarnessDoctor(HarnessDoctorRequest{RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test"})
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
	plan, err := InitProjectLifecycleState(repo, true)
	if err != nil {
		t.Fatal(err)
	}
	var profile ProjectLifecycleProfile
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
	result, err := HarnessDoctor(HarnessDoctorRequest{RepoRoot: repo, HarnessRoot: repo, Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasHarnessDoctorIssue(result.Issues, "lifecycle_namespace_mismatch") {
		t.Fatalf("expected namespace mismatch issue: %+v", result.Issues)
	}
}

func hasHarnessDoctorIssue(issues []HarnessDoctorIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
