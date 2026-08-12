package issueops

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestSwitchExecutionModeApplyReturnsNonCommandNextActionAfterExecutionRemoval(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "303-switch-provenance"})
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: repo, Root: filepath.Join(repo+".worktrees", record.Branch), Branch: record.Branch,
			BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-08-04T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{Generation: 6, Status: issueopscontract.LeaseStatusReleased},
	}
	record.WorktreePath = record.Execution.Workspace.Root
	record.PlanPath = filepath.Join(record.WorktreePath, filepath.FromSlash(IssueOpsArtifactDir), "plan.md")
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	git := func(string, ...string) (int, string) { return 1, "" }
	preview, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca",
	}, ExecutionSwitchModeDependencies{Git: git})
	if err != nil {
		t.Fatal(err)
	}
	if preview.NextCommand == "" || preview.LeaseGeneration != 6 {
		t.Fatalf("switch preview = %#v", preview)
	}
	result, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca", Apply: true, Confirm: true, Fingerprint: preview.Fingerprint,
	}, ExecutionSwitchModeDependencies{Git: git})
	if err != nil {
		t.Fatal(err)
	}
	if result.NextCommand != "" || result.NextAction == "" || result.LeaseGeneration != 6 {
		t.Fatalf("switch apply must return only non-command guidance: %#v", result)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution != nil {
		t.Fatalf("switch apply retained execution authority: %#v", persisted.Execution)
	}
	if persisted.WorktreePath != "" || persisted.PlanPath != "" {
		t.Fatalf("switch apply retained deleted workspace paths: worktree=%q plan=%q", persisted.WorktreePath, persisted.PlanPath)
	}
}
