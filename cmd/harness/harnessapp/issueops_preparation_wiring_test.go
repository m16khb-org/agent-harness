package harnessapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/cmd/harness/issueopscli/executioncmd"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestIssueOpsPrepareWiringRunsRealDirectPreviewWithoutPersistence(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "config", "user.name", "IssueOps Test")
	claimWiringGit(t, repo, "config", "user.email", "issueops@example.invalid")
	claimWiringGit(t, repo, "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	record, err := issueops.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "199-preparation-wiring"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/199"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	direct := &preparationWiringDirectFake{}
	handler := newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
		Direct: direct, Now: func() time.Time { return time.Date(2026, 8, 2, 4, 5, 6, 7, time.UTC) },
		NewOperationID: func() (string, error) { return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil },
	})
	process := &issueopscontract.NativeProcessReceipt{PID: 199, StartedAt: "2026-08-02T00:00:00Z", Executable: "/usr/local/bin/codex"}

	result, err := handler(context.Background(), stateRoot, issueops.ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", Actor: issueopscontract.NativeActor{Host: "codex", SessionID: "session", SessionProcess: process},
		CWD: repo, DirectReason: "wiring preview test", Confirm: false,
	}, issueops.ExecutionPrepareInvocation{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Preview || result.ResolvedMode != "direct" || direct.calls != 1 {
		t.Fatalf("result=%#v direct calls=%d", result, direct.calls)
	}
	persisted, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution != nil || persisted.WorktreePath != "" {
		t.Fatalf("preview persisted execution: %#v", persisted)
	}
}

func TestIssueOpsPrepareWiringUsesRequestScopedIssueSnapshot(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	record, err := issueops.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "199-preparation-snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/199"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "gitlab", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	fallbackCalls := 0
	fallback := func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		fallbackCalls++
		return port.ExecutionIssueSnapshot{}, context.DeadlineExceeded
	}
	handler := newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
		Orca: &reconcileProvisionerFake{}, ReadIssue: fallback,
	})

	issueSnapshot := &port.ExecutionIssueSnapshotEvidence{
		Provider: "gitlab", Source: "glab_mcp",
		WebURL: "https://gitlab.example.com/acme/repo/-/issues/199",
		Body:   claimWiringIssueBody(), State: "opened",
	}
	snapshotData, err := json.Marshal(issueSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(t.TempDir(), "issue-snapshot.json")
	if err := os.WriteFile(snapshotPath, snapshotData, 0o600); err != nil {
		t.Fatal(err)
	}
	request := issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionPrepare, ID: record.ID, Mode: "orca",
		Actor: claimWiringActor(t), CWD: repo, OwnerHost: "codex",
		IssueSnapshotFile: snapshotPath, IssueSnapshot: issueSnapshot,
	}
	previewRaw, err := issueops.ExecuteExecution(context.Background(), stateRoot, request, issueops.ExecutionActionDependencies{Prepare: handler, ReadIssue: fallback})
	if err != nil {
		t.Fatal(err)
	}
	preview := previewRaw.(issueops.ExecutionPrepareResult)
	if !strings.Contains(preview.NextCommand, "--issue-snapshot-file '") {
		t.Fatalf("snapshot-backed preview lost exact confirm source: %s", preview.NextCommand)
	}
	tokens := commandparse.SplitCommandTokens(preview.NextCommand)
	if len(tokens) < 4 || strings.Join(tokens[:4], " ") != "agent-harness issueops execution prepare" {
		t.Fatalf("invalid next command: %q tokens=%v", preview.NextCommand, tokens)
	}
	var raw any
	err = executioncmd.Run(tokens[3:], executioncmd.Deps{
		StateRoot: func() string { return stateRoot }, Prepare: handler, ReadIssue: fallback,
		PrintJSON: func(value any) error { raw = value; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := raw.(issueops.ExecutionPrepareResult)
	if !ok || !result.OK || result.ResolvedMode != "orca" || result.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("result=%#v", raw)
	}
	if fallbackCalls != 0 {
		t.Fatalf("validated request snapshot called provider fallback %d times", fallbackCalls)
	}
}

type preparationWiringDirectFake struct{ calls int }

func (fake *preparationWiringDirectFake) Prepare(_ context.Context, request port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	fake.calls++
	return port.ExecutionWorkspaceReceipt{
		SourceRoot: request.SourceRoot, Root: request.Root, Branch: request.Branch,
		BaseHead: request.BaseHead, ParentWorktree: request.ParentWorktree, Driver: "git",
	}, nil
}
