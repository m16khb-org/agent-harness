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

	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "user", "tighten acceptance criteria")
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
