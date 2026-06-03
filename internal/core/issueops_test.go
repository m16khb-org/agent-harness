package core

import (
	"strings"
	"testing"
)

func TestIssueOpsLifecycle(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "feature/demo"})
	if err != nil {
		t.Fatal(err)
	}
	if record.ID == "" || record.Phase != IssueOpsPhaseProblem || record.Repo != "/repo/example" || record.Branch != "feature/demo" {
		t.Fatalf("unexpected start record: %+v", record)
	}
	if ready := IssueOpsPRReadiness(record); ready.Ready {
		t.Fatalf("new cycle should not be PR-ready: %+v", ready)
	}

	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://gitlab.example/group/project/-/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhasePlan || record.IssueURL == "" {
		t.Fatalf("issue link should move to plan phase: %+v", record)
	}

	record, err = LinkIssueOpsPlan(stateRoot, record.ID, "docs/superpowers/plans/demo.md")
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhaseImplement || record.PlanPath == "" {
		t.Fatalf("plan link should move to implement phase: %+v", record)
	}

	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "user", "tighten acceptance criteria", "")
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhaseFeedback || len(record.Feedback) != 1 || record.Feedback[0].Source != "user" {
		t.Fatalf("feedback should be persisted and move to feedback phase: %+v", record)
	}

	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.ID != record.ID || reloaded.IssueURL != record.IssueURL || reloaded.PlanPath != record.PlanPath || len(reloaded.Feedback) != 1 {
		t.Fatalf("reloaded record mismatch: %+v vs %+v", reloaded, record)
	}
	if ready := IssueOpsPRReadiness(reloaded); !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("cycle with issue and plan should be PR-ready for drafting: %+v", ready)
	}
}

func TestIssueOpsAdvancePhaseCoversFullLifecycle(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "feature/demo"})
	if err != nil {
		t.Fatal(err)
	}
	// problem -> grill is a valid forward step.
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseGrill))
	if err != nil || record.Phase != IssueOpsPhaseGrill {
		t.Fatalf("expected grill phase, got %+v err=%v", record, err)
	}
	// An unknown phase must be rejected.
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, "nonsense"); err == nil {
		t.Fatalf("expected unknown phase rejection")
	}
	// pr phase requires issue + plan evidence (readiness gate).
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR)); err == nil {
		t.Fatalf("pr phase without readiness should be rejected")
	}
	if _, err := LinkIssueOpsIssue(stateRoot, record.ID, "https://example.com/acme/repo/issues/1"); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, "plans/demo.md"); err != nil {
		t.Fatal(err)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR))
	if err != nil || record.Phase != IssueOpsPhasePR {
		t.Fatalf("pr phase with readiness should succeed, got %+v err=%v", record, err)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseDone))
	if err != nil || record.Phase != IssueOpsPhaseDone {
		t.Fatalf("done phase should succeed, got %+v err=%v", record, err)
	}
}

func TestIssueOpsFeedbackRecordsClassification(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "review", "scope change請求", "contract_change")
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Feedback) != 1 || record.Feedback[0].Classification != "contract_change" {
		t.Fatalf("expected classification persisted, got %+v", record.Feedback)
	}
	// classification is optional and defaults to empty.
	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "ci", "flaky test", "")
	if err != nil {
		t.Fatal(err)
	}
	if record.Feedback[1].Classification != "" {
		t.Fatalf("expected empty classification default, got %+v", record.Feedback[1])
	}
}

func TestIssueOpsRejectsUnsafeInputs(t *testing.T) {
	stateRoot := t.TempDir()
	if _, err := StartIssueOps(stateRoot, IssueOpsStartRequest{}); err == nil || !strings.Contains(err.Error(), "repo") {
		t.Fatalf("expected repo validation error, got %v", err)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsIssue(stateRoot, record.ID, "TOKEN=secret-value"); err == nil || !strings.Contains(err.Error(), "issue_url") {
		t.Fatalf("expected issue URL validation error, got %v", err)
	}
}
