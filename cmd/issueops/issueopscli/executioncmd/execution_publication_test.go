package executioncmd

import (
	"context"
	"fmt"
	"io"
	"issueops/cmd/issueops/issueopscli/remotecmd"
	"os"
	"reflect"
	"strconv"
	"testing"

	"issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestActionDepsPropagatePublicationReconcileWithoutInvocation(t *testing.T) {
	invoked := 0
	handler := issueops.RemotePullRequestReconcileHandler(func(context.Context, string, issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
		invoked++
		return issueops.ExecutionReconcileResult{}, nil
	})

	deps := (Deps{Publication: remotecmd.PublicationHandlers{Reconcile: handler}}).actionDeps()
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
			Publication: remotecmd.PublicationHandlers{Reconcile: func(_ context.Context, _ string, request issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
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

func publicationReconcileCLIRecord(t *testing.T) (string, issueopscontract.IssueOpsRecord, issueopscontract.NativeProcessReceipt) {
	t.Helper()
	ancestry, err := issueops.ObserveNativeProcessAncestry(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	var receipt issueopscontract.NativeProcessReceipt
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
	actor := issueopscontract.NativeActor{Host: "codex", SessionID: "publication-cli-session", SessionProcess: &receipt}
	record := issueopscontract.IssueOpsRecord{
		OK: true, SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID: issueops.NewIssueOpsID(repo, "195-publication-cli"), Repo: repo, Branch: "195-publication-cli",
		Phase: issueops.IssueOpsPhasePR, WorktreePath: worktree,
		Execution: &issueopscontract.Execution{
			Mode:      issueopscontract.ExecutionModeDirect,
			Workspace: issueopscontract.Workspace{SourceRoot: repo, Root: worktree, Branch: "195-publication-cli", BaseHead: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Driver: "git", LinkedAt: "2026-08-01T00:00:00Z"},
			Lease:     issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusActive, Holder: &actor, ClaimedAt: "2026-08-01T00:00:00Z"},
			Pending:   &issueopscontract.ExternalIntent{OperationID: "0123456789abcdef0123456789abcdef", Kind: "remote_pr_create", Marker: "<!-- issueops:issueops-v1 operation=0123456789abcdef0123456789abcdef -->", StartedAt: "2026-08-01T00:00:00Z"},
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
