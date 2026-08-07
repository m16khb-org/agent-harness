package issueopscompletion

import (
	"context"
	"reflect"
	"testing"

	"agent-harness/internal/adapter/issueops"
	completionapp "agent-harness/internal/application/issueopscompletion"
	model "agent-harness/internal/contract/issueops"
	leasecontract "agent-harness/internal/contract/issueopslease"
)

func TestHandlerMapsCoreRequestAndResult(t *testing.T) {
	receipt := model.NativeProcessReceipt{PID: 198, StartedAt: "2026-08-02T00:00:00Z", Executable: "/bin/codex"}
	request := issueops.ExecutionCompleteRequest{ID: "io-198", Generation: 2, Actor: model.NativeActor{Host: "codex", SessionID: "session", SessionProcess: &receipt, ProcessAncestry: []model.NativeProcessReceipt{receipt}}, CWD: "/worktree", FinalHead: "head", TuringReportPath: "/worktree/report", Verification: []string{"test"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/198", Confirm: true}
	service := &serviceFake{result: completionapp.Result{OK: true, ID: request.ID, Execution: leasecontract.Execution{Mode: "direct", Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "198", BaseHead: "base", Driver: "git", LinkedAt: "now"}, Lease: leasecontract.Lease{Generation: 2, Status: "released"}, CompletionHistory: []leasecontract.CompletionHistoryEntry{{Generation: 1, Completion: leasecontract.Completion{Verification: []string{"old verification"}}, Reason: "reseed", ReopenedAt: "now"}}}, OrcaTaskSettled: true}}

	result, err := NewHandler(service)(context.Background(), t.TempDir(), request)
	if err != nil {
		t.Fatal(err)
	}
	if service.request.ID != request.ID || service.request.Actor.Process == nil || len(service.request.Ancestry) != 1 || service.request.CWD != request.CWD {
		t.Fatalf("mapped request = %+v", service.request)
	}
	if !reflect.DeepEqual(service.request.Verification, request.Verification) {
		t.Fatalf("verification = %#v", service.request.Verification)
	}
	if !result.OK || result.ID != request.ID || !result.OrcaTaskSettled || result.Execution.Lease.Status != model.LeaseStatusReleased {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Execution.CompletionHistory) != 1 || result.Execution.CompletionHistory[0].Completion.Verification[0] != "old verification" {
		t.Fatalf("completion history=%+v", result.Execution.CompletionHistory)
	}
}

type serviceFake struct {
	request completionapp.Request
	result  completionapp.Result
	err     error
}

func (f *serviceFake) Complete(_ context.Context, request completionapp.Request) (completionapp.Result, error) {
	f.request = request
	return f.result, f.err
}
