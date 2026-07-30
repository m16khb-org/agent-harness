package harnessapp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"agent-harness/cmd/harness/issueopscli/executioncmd"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestHarnessAppReseedWiringUsesVerticalRepository(t *testing.T) {
	receipt, err := issueops.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatalf("observe native process: %v", err)
	}
	_, err = issueOpsReseedHandler(context.Background(), t.TempDir(), issueops.ExecutionReseedRequest{ID: "io-reseed-wiring", Confirm: true, Actor: model.NativeActor{Host: "codex", SessionID: "reseed-wiring", SessionProcess: &receipt, ProcessAncestry: []model.NativeProcessReceipt{receipt}}})
	if err == nil || !strings.Contains(err.Error(), "issueops record io-reseed-wiring not found") {
		t.Fatalf("reseed wiring error=%v", err)
	}
}

func TestIssueOpsReseedHandlerUsesResolvedSnapshotReader(t *testing.T) {
	stateRoot, record, _, _, _ := seedOrcaClaimSnapshot(t)
	actor := claimWiringActor(t)
	owner := reseedWiringOwner{}
	preview, err := issueops.ReplaceExecutionWithDependencies(context.Background(), stateRoot, issueops.ExecutionReplaceRequest{
		ID: record.ID, Action: issueops.ExecutionReplacePreview, ExpectedGeneration: 1, Actor: actor, CWD: record.Execution.Workspace.Root,
	}, issueops.ExecutionReplaceDependencies{OrcaOwner: owner})
	if err != nil {
		t.Fatalf("preview reseed inventory: %v", err)
	}
	evidence := &port.ExecutionIssueSnapshotEvidence{
		Provider: "gitlab", Source: "glab_mcp", WebURL: "https://gitlab.example.com/acme/repo/-/issues/16",
		Body: claimWiringIssueBody(), State: "opened",
	}
	fallbackCalls := 0
	raw, err := issueops.ExecuteExecution(context.Background(), stateRoot, issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionReplace, ReplaceAction: issueops.ExecutionReplaceReseed, ID: record.ID,
		ExpectedGeneration: 1, InventoryFingerprint: preview.InventoryFingerprint, Reason: "resolved snapshot reseed",
		Actor: actor, CWD: record.Execution.Workspace.Root, Confirm: true, IssueSnapshot: evidence,
	}, issueops.ExecutionActionDependencies{
		ReadIssue: func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			fallbackCalls++
			return port.ExecutionIssueSnapshot{}, context.DeadlineExceeded
		},
		Reseed: func(ctx context.Context, root string, request issueops.ExecutionReseedRequest) (issueops.ExecutionReplaceResult, error) {
			return issueOpsReseedHandlerWithOwner(ctx, root, request, owner)
		},
	})
	if err != nil {
		t.Fatalf("reseed with resolved snapshot reader: %v", err)
	}
	result, ok := raw.(issueops.ExecutionReplaceResult)
	if !ok || !result.OK || result.Execution.Lease.Generation != 2 || result.IssueSnapshotSource != "glab_mcp" || fallbackCalls != 0 {
		t.Fatalf("reseed result=%#v fallback_calls=%d", raw, fallbackCalls)
	}
}

func TestExecutionReseedCLIDogfoodDirectAndOrca(t *testing.T) {
	for _, mode := range []model.ExecutionMode{model.ExecutionModeDirect, model.ExecutionModeOrca} {
		t.Run(string(mode), func(t *testing.T) {
			stateRoot, record, oldToken, _, _ := seedOrcaClaimSnapshot(t)
			if mode == model.ExecutionModeDirect {
				record.Execution.Mode = model.ExecutionModeDirect
				record.Execution.Workspace.Driver = "git"
				record.Execution.Orca = nil
				if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
					t.Fatal(err)
				}
			}
			actor := claimWiringActor(t)
			owner := reseedWiringOwner{}
			preview, err := issueops.ReplaceExecutionWithDependencies(context.Background(), stateRoot, issueops.ExecutionReplaceRequest{
				ID: record.ID, Action: issueops.ExecutionReplacePreview, ExpectedGeneration: 1, Actor: actor, CWD: record.Execution.Workspace.Root,
			}, issueops.ExecutionReplaceDependencies{OrcaOwner: owner})
			if err != nil {
				t.Fatalf("preview: %v", err)
			}
			var result issueops.ExecutionReplaceResult
			deps := executioncmd.Deps{
				StateRoot: func() string { return stateRoot },
				Reseed: func(ctx context.Context, root string, request issueops.ExecutionReseedRequest) (issueops.ExecutionReplaceResult, error) {
					return issueOpsReseedHandlerWithOwner(ctx, root, request, owner)
				},
				PrintJSON: func(value any) error {
					mapped, ok := value.(issueops.ExecutionReplaceResult)
					if !ok {
						return fmt.Errorf("unexpected CLI result %T", value)
					}
					result = mapped
					return nil
				},
			}
			if mode == model.ExecutionModeOrca {
				deps.ReadIssue = func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
					return port.ExecutionIssueSnapshot{URL: request.URL, Body: claimWiringIssueBody(), State: "opened"}, nil
				}
			}
			process := actor.SessionProcess
			args := []string{
				"replace", "--id", record.ID, "--expected-generation", "1", "--reseed", "--confirm", "--inventory-fingerprint", preview.InventoryFingerprint,
				"--reason", "CLI dogfood reseed", "--host", actor.Host, "--session-id", actor.SessionID,
				"--session-pid", fmt.Sprintf("%d", process.PID), "--session-started-at", process.StartedAt,
				"--session-executable", process.Executable, "--cwd", record.Execution.Workspace.Root, "--json",
			}
			if err := executioncmd.Run(args, deps); err != nil {
				t.Fatalf("CLI reseed: %v", err)
			}
			if !result.OK || result.Action != issueops.ExecutionReplaceReseed || result.Execution.Lease.Generation != 2 || result.Execution.Lease.Status != model.LeaseStatusClaimable || result.ClaimTokenPath == "" {
				t.Fatalf("CLI reseed result=%+v", result)
			}
			if info, err := os.Stat(result.ClaimTokenPath); err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("new token info=%v err=%v", info, err)
			}
			if _, err := os.Stat(oldToken); !os.IsNotExist(err) {
				t.Fatalf("superseded token err=%v", err)
			}
			persisted, err := issueops.ReadIssueOps(stateRoot, record.ID)
			if err != nil || persisted.Execution.Lease.Generation != 2 || persisted.Execution.Lease.Status != model.LeaseStatusClaimable {
				t.Fatalf("persisted reseed=%+v err=%v", persisted.Execution, err)
			}
			if mode == model.ExecutionModeOrca && (result.ContextPacketSHA256 == "" || result.OwnerPromptSHA256 == "" || result.NextCommand == "") {
				t.Fatalf("Orca owner reseal result=%+v", result)
			}
		})
	}
}

type reseedWiringOwner struct{}

func (reseedWiringOwner) InspectOwner(context.Context, port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	return port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime", TaskStatus: "completed", DispatchStatus: "failed"}, nil
}
