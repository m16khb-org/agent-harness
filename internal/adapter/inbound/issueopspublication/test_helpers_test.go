package issueopspublication

import (
	"context"
	"encoding/json"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	publicationcontract "agent-harness/internal/contract/issueopspublication"
	"agent-harness/internal/core/issueops"
)

type fakeCreateService struct {
	t        *testing.T
	expected bool
	called   bool
	command  publicationcontract.CreateCommand
	result   publicationcontract.ProviderCreateResult
	err      error
}

func (f *fakeCreateService) Create(_ context.Context, command publicationcontract.CreateCommand) (publicationcontract.ProviderCreateResult, error) {
	f.t.Helper()
	if !f.expected || f.called {
		f.t.Fatalf("unexpected publication Create call")
	}
	f.called = true
	f.command = command.Clone()
	return f.result, f.err
}

type fakeReconcileService struct {
	t        *testing.T
	expected bool
	called   bool
	id       string
	result   publicationcontract.ReconcileResult
	err      error
}

func (f *fakeReconcileService) Reconcile(_ context.Context, id string) (publicationcontract.ReconcileResult, error) {
	f.t.Helper()
	if !f.expected || f.called {
		f.t.Fatalf("unexpected publication Reconcile call")
	}
	f.called = true
	f.id = id
	return f.result.Clone(), f.err
}

var _ createService = (*fakeCreateService)(nil)
var _ reconcileService = (*fakeReconcileService)(nil)

func fullCoreCreateRequest() issueops.RemotePullRequestRequest {
	process := issueopscontract.NativeProcessReceipt{PID: 123, StartedAt: "2026-08-01T00:00:00.123456789Z", Executable: "/usr/local/bin/codex"}
	return issueops.RemotePullRequestRequest{
		ID: "io-195", Provider: "github", Title: "Publication vertical", Body: "Compatibility body.",
		Head: "195-publication", Base: "117-hexagonal-architecture-migration",
		Labels: []string{"enhancement", "issueops"}, Assignees: []string{"maintainer"},
		ExpectedGeneration: 9,
		Actor: issueopscontract.NativeActor{
			Host: "codex", SessionID: "session-195", AgentID: "agent-195", SessionProcess: &process,
			ProcessAncestry: []issueopscontract.NativeProcessReceipt{
				{PID: 122, StartedAt: "2026-08-01T00:00:00.023456789Z", Executable: "/usr/bin/zsh"},
				process,
			},
		},
		CWD: "/repo.worktrees/195-publication", Confirm: true,
	}
}

func fullCoreReconcileRequest() issueops.ExecutionReconcileRequest {
	process := issueopscontract.NativeProcessReceipt{PID: 223, StartedAt: "2026-08-01T01:00:00.123456789Z", Executable: "/usr/local/bin/codex"}
	return issueops.ExecutionReconcileRequest{
		ID: "io-195", Preview: false, Confirm: true,
		Actor: issueopscontract.NativeActor{
			Host: "codex", SessionID: "reconcile-session", AgentID: "reconcile-agent", SessionProcess: &process,
			ProcessAncestry: []issueopscontract.NativeProcessReceipt{
				{PID: 222, StartedAt: "2026-08-01T01:00:00.023456789Z", Executable: "/usr/bin/zsh"},
				process,
			},
		},
		CWD:      "/repo.worktrees/195-publication",
		Snapshot: &issueopscontract.IssueOpsRecord{ID: "io-195", Phase: issueops.IssueOpsPhasePR},
	}
}

func publicationRecordRaw(t *testing.T) []byte {
	t.Helper()
	record := issueopscontract.IssueOpsRecord{
		OK: true, SchemaVersion: issueops.IssueOpsCurrentSchemaVersion, ID: "io-195", Repo: "/repo",
		Branch: "195-publication", Phase: issueops.IssueOpsPhasePR,
		Execution: &issueopscontract.Execution{
			Mode: issueopscontract.ExecutionModeDirect,
			Workspace: issueopscontract.Workspace{
				SourceRoot: "/repo", Root: "/repo.worktrees/195-publication", Branch: "195-publication",
				BaseHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Driver: "git", LinkedAt: "2026-08-01T00:00:00Z",
			},
			Lease: issueopscontract.WriteLease{Generation: 9, Status: issueopscontract.LeaseStatusActive},
			Pending: &issueopscontract.ExternalIntent{
				OperationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "remote_pr_create",
				Marker:    "<!-- agent-harness:issueops-v1 operation=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -->",
				StartedAt: "2026-08-01T01:00:00Z",
			},
		},
		CreatedAt: "2026-08-01T00:00:00Z", UpdatedAt: "2026-08-01T01:00:00Z",
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
