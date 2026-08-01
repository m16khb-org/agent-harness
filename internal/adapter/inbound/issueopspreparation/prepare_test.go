package issueopspreparation

import (
	"context"
	"errors"
	"reflect"
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
)

func TestHandlerMapsEveryRequestAndResultField(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 199, StartedAt: "2026-08-02T01:02:03.123456789Z", Executable: "/usr/local/bin/codex"}
	request := issueops.ExecutionPrepareRequest{
		ID: "io-199", Mode: "orca",
		Actor: model.NativeActor{Host: "codex", SessionID: "session-199", AgentID: "agent-199", SessionProcess: &process},
		CWD:   "/repo", OwnerHost: "claude", OwnerModel: "claude-sonnet-5", OwnerEffort: "high", Confirm: true,
	}
	execution := fullContractExecution()
	service := &serviceFake{result: preparationcontract.Result{
		OK: true, ID: request.ID, Preview: true, RequestedMode: "auto", ResolvedMode: "orca", FallbackCode: "fallback",
		Workspace: execution.Workspace, Execution: &execution,
		ClaimTokenPath: "/tokens/199", IssueBodySHA256: "issue-sha", ContextPacketPath: "/packets/199",
		ContextPacketSHA256: "packet-sha", OwnerPromptPath: "/prompts/199", OwnerPromptSHA256: "prompt-sha",
		IssueSnapshotSource: "provider", NextCommand: "agent-harness next",
	}}

	got, err := NewHandler(service)(context.Background(), "/state", request, issueops.ExecutionPrepareInvocation{})
	if err != nil {
		t.Fatal(err)
	}
	wantCommand := preparationcontract.Command{
		ID: request.ID, Mode: request.Mode,
		Actor: preparationcontract.Actor{
			Host: request.Actor.Host, SessionID: request.Actor.SessionID, AgentID: request.Actor.AgentID,
			SessionProcess: &leasecontract.ProcessReceipt{PID: process.PID, StartedAt: process.StartedAt, Executable: process.Executable},
		},
		CWD: request.CWD, OwnerHost: request.OwnerHost, OwnerModel: request.OwnerModel, OwnerEffort: request.OwnerEffort, Confirm: true,
	}
	if !reflect.DeepEqual(service.command, wantCommand) {
		t.Fatalf("command=%#v want=%#v", service.command, wantCommand)
	}
	want := issueops.ExecutionPrepareResult{
		OK: true, ID: request.ID, Preview: true, RequestedMode: "auto", ResolvedMode: "orca", FallbackCode: "fallback",
		Workspace: coreWorkspace(execution.Workspace), Execution: coreExecution(&execution),
		ClaimTokenPath: "/tokens/199", IssueBodySHA256: "issue-sha", ContextPacketPath: "/packets/199",
		ContextPacketSHA256: "packet-sha", OwnerPromptPath: "/prompts/199", OwnerPromptSHA256: "prompt-sha",
		IssueSnapshotSource: "provider", NextCommand: "agent-harness next",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result=%#v want=%#v", got, want)
	}

	request.Actor.SessionProcess.PID = 999
	if !reflect.DeepEqual(service.command, wantCommand) {
		t.Fatalf("mapped command aliases public request: %#v", service.command)
	}
}

func TestHandlerPreservesResultWithServiceError(t *testing.T) {
	cause := errors.New("pending external intent")
	service := &serviceFake{result: preparationcontract.Result{ID: "io-199", RequestedMode: "orca", NextCommand: "reconcile"}, err: cause}

	got, err := NewHandler(service)(context.Background(), "/state", issueops.ExecutionPrepareRequest{ID: "io-199"}, issueops.ExecutionPrepareInvocation{})
	if err != cause || got.ID != "io-199" || got.RequestedMode != "orca" || got.NextCommand != "reconcile" {
		t.Fatalf("result=%#v err=%v", got, err)
	}
}

func TestHandlerFailsClosedWithoutService(t *testing.T) {
	got, err := NewHandler(nil)(context.Background(), "/state", issueops.ExecutionPrepareRequest{ID: "io-199"}, issueops.ExecutionPrepareInvocation{})
	if !errors.Is(err, issueops.ErrPrepareHandlerUnavailable) || got != (issueops.ExecutionPrepareResult{ID: "io-199"}) {
		t.Fatalf("result=%#v err=%v", got, err)
	}
}

type serviceFake struct {
	command preparationcontract.Command
	result  preparationcontract.Result
	err     error
}

func (fake *serviceFake) Prepare(_ context.Context, command preparationcontract.Command) (preparationcontract.Result, error) {
	fake.command = command.Clone()
	return fake.result.Clone(), fake.err
}

func fullContractExecution() leasecontract.Execution {
	return leasecontract.Execution{
		Mode:      "orca",
		Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "199-vertical", BaseHead: "base", ParentWorktree: "/parent", Driver: "orca", LinkedAt: "2026-08-02T02:03:04Z"},
		Lease: leasecontract.Lease{
			Generation: 3, Status: "active",
			Holder:           &leasecontract.Actor{Host: "claude", SessionID: "owner", AgentID: "agent", SessionProcess: &leasecontract.ProcessReceipt{PID: 200, StartedAt: "2026-08-02T02:03:05Z", Executable: "/bin/claude"}},
			ClaimTokenSHA256: "claim-sha", ClaimedAt: "claimed", ReleasedAt: "released", ReplacedAt: "replaced", ReplacementReason: "reason",
		},
		Orca:           &leasecontract.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", RunID: "run", WorktreeInstanceID: "instance", LeaseGeneration: 3, OwnerHost: "claude", OwnerModel: "model", OwnerEffort: "high", TaskID: "task", DispatchID: "dispatch", TerminalPTYID: "pty"},
		Pending:        &leasecontract.ExternalIntent{OperationID: "operation", Kind: "orca_prepare", Marker: "marker", StartedAt: "started"},
		Completion:     &leasecontract.Completion{FinalHead: "head", TuringReportPath: "/report", Verification: []string{"go test ./..."}, RemoteArtifactURL: "https://example.test/pr/199", CompletedAt: "completed"},
		Failure:        &leasecontract.FailureDetail{OperationID: "operation", Code: "failed", Message: "redacted", At: "failed-at"},
		SyncBaseEvents: []leasecontract.SyncBaseEvent{{Mode: "merge", BaseBranch: "main", BaseOID: "base", MergeCommit: "merge", ConflictFiles: 2, Actor: "codex", At: "synced"}},
	}
}
