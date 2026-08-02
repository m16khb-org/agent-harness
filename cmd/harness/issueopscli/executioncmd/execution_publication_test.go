package executioncmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"testing"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
)

func TestActionDepsPropagatePublicationReconcileWithoutInvocation(t *testing.T) {
	invoked := 0
	handler := issueops.RemotePullRequestReconcileHandler(func(context.Context, string, issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
		invoked++
		return issueops.ExecutionReconcileResult{}, nil
	})

	deps := (Deps{Publication: issueops.RemotePublicationHandlers{Reconcile: handler}}).actionDeps()
	if deps.RemoteReconcile == nil {
		t.Fatal("publication reconcile handler was not propagated")
	}
	if reflect.ValueOf(deps.RemoteReconcile).Pointer() != reflect.ValueOf(handler).Pointer() {
		t.Fatal("publication reconcile handler changed during execution dependency mapping")
	}
	if invoked != 0 {
		t.Fatalf("publication reconcile handler invoked during propagation: %d", invoked)
	}
}

func TestRunPublicationReconcilePreservesCLITextProjection(t *testing.T) {
	stateRoot, record, receipt := publicationReconcileCLIRecord(t)
	calls := 0
	output := capturePublicationCLIStdout(t, func() {
		err := Run([]string{
			"reconcile", "--id", record.ID, "--confirm",
			"--host", "codex", "--session-id", "publication-cli-session",
			"--session-pid", strconv.Itoa(receipt.PID), "--session-started-at", receipt.StartedAt,
			"--session-executable", receipt.Executable, "--cwd", record.Execution.Workspace.Root,
		}, Deps{
			StateRoot: func() string { return stateRoot },
			Publication: issueops.RemotePublicationHandlers{Reconcile: func(_ context.Context, _ string, request issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
				calls++
				if request.Snapshot == nil || request.Snapshot.ID != record.ID {
					t.Fatalf("publication reconcile snapshot=%#v", request.Snapshot)
				}
				return issueops.ExecutionReconcileResult{OK: true, ID: record.ID, Reconciled: true, Code: "remote_reconcile_adopted"}, nil
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	if calls != 1 || output != fmt.Sprintf("%s remote_reconcile_adopted pending=false\n", record.ID) {
		t.Fatalf("calls=%d output=%q", calls, output)
	}
}

func publicationReconcileCLIRecord(t *testing.T) (string, issueops.IssueOpsRecord, model.NativeProcessReceipt) {
	t.Helper()
	ancestry, err := issueops.ObserveNativeProcessAncestry(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	var receipt model.NativeProcessReceipt
	for _, candidate := range ancestry {
		if candidate.PID == os.Getpid() {
			receipt = candidate
			break
		}
	}
	if receipt.PID == 0 {
		t.Fatalf("current process receipt missing from ancestry: %#v", ancestry)
	}
	stateRoot, repo, worktree := t.TempDir(), t.TempDir(), t.TempDir()
	actor := model.NativeActor{Host: "codex", SessionID: "publication-cli-session", SessionProcess: &receipt}
	record := issueops.IssueOpsRecord{
		OK: true, SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID: issueops.NewIssueOpsID(repo, "195-publication-cli"), Repo: repo, Branch: "195-publication-cli",
		Phase: issueops.IssueOpsPhasePR, WorktreePath: worktree,
		Execution: &model.Execution{
			Mode:      model.ExecutionModeDirect,
			Workspace: model.Workspace{SourceRoot: repo, Root: worktree, Branch: "195-publication-cli", BaseHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Driver: "git", LinkedAt: "2026-08-01T00:00:00Z"},
			Lease:     model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &actor, ClaimedAt: "2026-08-01T00:00:00Z"},
			Pending:   &model.ExternalIntent{OperationID: "0123456789abcdef0123456789abcdef", Kind: "remote_pr_create", Marker: "<!-- agent-harness:issueops-v1 operation=0123456789abcdef0123456789abcdef -->", StartedAt: "2026-08-01T00:00:00Z"},
		},
		CreatedAt: "2026-08-01T00:00:00Z",
		UpdatedAt: "2026-08-01T00:00:00Z",
	}
	written, err := issueops.WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, written, receipt
}

func capturePublicationCLIStdout(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	run()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}
