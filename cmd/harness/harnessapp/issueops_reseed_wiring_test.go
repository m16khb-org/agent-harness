package harnessapp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"agent-harness/cmd/harness/issueopscli/executioncmd"
	"agent-harness/internal/adapter/issueops"
	basesyncoutbound "agent-harness/internal/adapter/outbound/issueopsbasesync"
	model "agent-harness/internal/contract/issueops"
	"agent-harness/internal/domain/commandparse"
	"agent-harness/internal/port"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

type reseedProvenanceObserver struct{}

func (reseedProvenanceObserver) Observe(context.Context) (provenanceport.Receipt, error) {
	return provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: strings.Repeat("a", 64),
	}, nil
}

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
				StateRoot:  func() string { return stateRoot },
				Provenance: reseedProvenanceObserver{},
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

func TestExecutionReseedCompletedStatusExposesReopenContract(t *testing.T) {
	stateRoot, record, _, _, _ := seedOrcaClaimSnapshot(t)
	claimWiringGit(t, record.Execution.Workspace.SourceRoot, "remote", "add", "origin", record.Execution.Workspace.SourceRoot)
	oldHead := "d6d8c6a5a98fcca6bca33edf9e7965636429ce28"
	record.Phase = model.IssueOpsPhaseDone
	record.Execution.Lease.Status = model.LeaseStatusReleased
	record.Execution.Lease.ClaimTokenSHA256 = ""
	record.Execution.Completion = &model.ExecutionCompletion{Generation: 1, FinalHead: oldHead, TuringReportPath: ".agent-harness/turing/old.json", Verification: []string{"old verification"}, RemoteArtifactURL: "https://gitlab.example.com/acme/repo/-/merge_requests/1", CompletedAt: "2026-08-03T00:00:00Z"}
	record.AISlopCleanAt = "old"
	record.AISlopCleanHead = oldHead
	record.AISlopCleanFingerprint = "old-fingerprint"
	record.AISlopCleanCategories = []string{"duplication"}
	record.AISlopCleanVerification = []string{"old verification"}
	record.ImplementationReview = &model.IssueOpsImplementationReview{Verdict: "pass"}
	record.RemoteCompletion = &model.IssueOpsRemoteCompletion{ReflectedAt: "old"}
	record.PhaseLedger = model.IssueOpsPhaseLedger{
		model.IssueOpsPhaseProblem:     {Phase: model.IssueOpsPhaseProblem, CompletedAt: "old-upstream"},
		model.IssueOpsPhaseImplement:   {Phase: model.IssueOpsPhaseImplement, CompletedAt: "old"},
		model.IssueOpsPhaseAISlopClean: {Phase: model.IssueOpsPhaseAISlopClean, CompletedAt: "old"},
		model.IssueOpsPhaseFeedback:    {Phase: model.IssueOpsPhaseFeedback, CompletedAt: "old"},
		model.IssueOpsPhasePR:          {Phase: model.IssueOpsPhasePR, CompletedAt: "old"},
		model.IssueOpsPhaseDone:        {Phase: model.IssueOpsPhaseDone, CompletedAt: "old"},
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	actor := claimWiringActor(t)
	owner := reseedWiringOwner{}
	preview, err := issueops.ReplaceExecutionWithDependencies(context.Background(), stateRoot, issueops.ExecutionReplaceRequest{ID: record.ID, Action: issueops.ExecutionReplacePreview, ExpectedGeneration: 1, CompletionGeneration: 1, Actor: actor, CWD: record.Execution.Workspace.Root}, issueops.ExecutionReplaceDependencies{OrcaOwner: owner, BaseSync: basesyncoutbound.NewInspector(basesyncoutbound.RunGit)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.NextCommand, "--completion-generation 1") {
		t.Fatalf("preview lost typed completion provenance: %q", preview.NextCommand)
	}
	result, err := issueOpsReseedHandlerWithOwner(context.Background(), stateRoot, issueops.ExecutionReseedRequest{ID: record.ID, ExpectedGeneration: 1, CompletionGeneration: 1, Actor: actor, CWD: record.Execution.Workspace.Root, InventoryFingerprint: preview.InventoryFingerprint, Reason: "functional HEAD changed", Confirm: true, ReadIssue: func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		return port.ExecutionIssueSnapshot{URL: request.URL, Body: claimWiringIssueBody(), State: "opened"}, nil
	}}, owner)
	if err != nil {
		t.Fatalf("completed reseed: %v", err)
	}
	if result.Execution.Completion != nil || len(result.Execution.CompletionHistory) != 1 || result.Execution.CompletionHistory[0].Completion.FinalHead != oldHead || result.Execution.CompletionHistory[0].Completion.Verification[0] != "old verification" {
		t.Fatalf("reseed projection=%+v", result.Execution)
	}
	status, err := issueops.StatusExecution(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := issueOpsStatusHandler()(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Execution.Completion != nil || len(status.Execution.CompletionHistory) != 1 || persisted.Phase != model.IssueOpsPhaseImplement {
		t.Fatalf("status projection=%+v record_phase=%s", status.Execution, persisted.Phase)
	}
	if !strings.Contains(status.NextCommand, "issueops execution resume") || strings.Contains(status.NextCommand, "execution complete") {
		t.Fatalf("status next command=%q", status.NextCommand)
	}
	if persisted.AISlopCleanAt != "" || persisted.ImplementationReview != nil || persisted.RemoteCompletion != nil {
		t.Fatalf("status retained current proof: %+v", persisted)
	}
	if persisted.PhaseLedger[model.IssueOpsPhaseProblem].CompletedAt != "old-upstream" {
		t.Fatalf("upstream ledger changed: %+v", persisted.PhaseLedger)
	}
	for _, phase := range []model.IssueOpsPhase{model.IssueOpsPhaseImplement, model.IssueOpsPhaseAISlopClean, model.IssueOpsPhaseFeedback, model.IssueOpsPhasePR, model.IssueOpsPhaseDone} {
		entry := persisted.PhaseLedger[phase]
		if entry.CompletedAt != "" || len(entry.Notes) == 0 || entry.Notes[len(entry.Notes)-1] != "stale: completed execution reseed (1 -> 2)" {
			t.Fatalf("phase %s ledger=%+v", phase, entry)
		}
	}
}

func TestCompletedReplacementPreviewRequiresSyncBaseBeforeReseedWhenParentDrifted(t *testing.T) {
	stateRoot, record, actor, owner := completedReplacementPreviewFixture(t, true)
	preview, err := issueops.ReplaceExecutionWithDependencies(context.Background(), stateRoot, issueops.ExecutionReplaceRequest{
		ID: record.ID, Action: issueops.ExecutionReplacePreview, ExpectedGeneration: 1, CompletionGeneration: 1,
		Actor: actor, CWD: record.Execution.Workspace.Root,
	}, issueops.ExecutionReplaceDependencies{OrcaOwner: owner, BaseSync: basesyncoutbound.NewInspector(basesyncoutbound.RunGit)})
	if err == nil {
		t.Fatalf("drifted completed preview emitted reseed instead of sync-base: %+v", preview)
	}
	structured, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok {
		t.Fatalf("drift rejection is not structured: %T %v", err, err)
	}
	fields := structured.IssueOpsErrorFields()
	if fields["code"] != "post_completion_sync_base_required" || fields["completion_generation"] != uint64(1) {
		t.Fatalf("drift rejection fields=%v", fields)
	}
	next, _ := fields["next_command"].(string)
	if next != "agent-harness issueops execution sync-base --id '"+record.ID+"' --preview --completion-generation 1 --json" || strings.Contains(next, "--reseed") {
		t.Fatalf("drift rejection next_command=%q", next)
	}
}

func TestCompletedReplacementPreviewKeepsNoDriftReseed(t *testing.T) {
	stateRoot, record, actor, owner := completedReplacementPreviewFixture(t, false)
	preview, err := issueops.ReplaceExecutionWithDependencies(context.Background(), stateRoot, issueops.ExecutionReplaceRequest{
		ID: record.ID, Action: issueops.ExecutionReplacePreview, ExpectedGeneration: 1, CompletionGeneration: 1,
		Actor: actor, CWD: record.Execution.Workspace.Root,
	}, issueops.ExecutionReplaceDependencies{OrcaOwner: owner, BaseSync: basesyncoutbound.NewInspector(basesyncoutbound.RunGit)})
	if err != nil {
		t.Fatalf("no-drift completed preview: %v", err)
	}
	if !strings.Contains(preview.NextCommand, "execution replace") || !strings.Contains(preview.NextCommand, "--reseed") || !strings.Contains(preview.NextCommand, "--completion-generation 1") {
		t.Fatalf("no-drift completed preview lost reseed command: %q", preview.NextCommand)
	}
}

func TestCompletedReplacementPreviewRejectsMissingStampedCompletionGeneration(t *testing.T) {
	stateRoot, record, actor, owner := completedReplacementPreviewFixture(t, false)
	record.Execution.Completion.Generation = 0
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	_, err := issueops.ReplaceExecutionWithDependencies(context.Background(), stateRoot, issueops.ExecutionReplaceRequest{
		ID: record.ID, Action: issueops.ExecutionReplacePreview, ExpectedGeneration: 1, CompletionGeneration: 1,
		Actor: actor, CWD: record.Execution.Workspace.Root,
	}, issueops.ExecutionReplaceDependencies{OrcaOwner: owner, BaseSync: basesyncoutbound.NewInspector(basesyncoutbound.RunGit)})
	if err == nil || err.Error() != "invalid or missing stamped completion generation" {
		t.Fatalf("zero-generation preview error=%v", err)
	}
}

func completedReplacementPreviewFixture(t *testing.T, drift bool) (string, model.IssueOpsRecord, model.NativeActor, reseedWiringOwner) {
	t.Helper()
	stateRoot, record, _, _, _ := seedOrcaClaimSnapshot(t)
	source := record.Execution.Workspace.SourceRoot
	worktree := record.Execution.Workspace.Root
	claimWiringGit(t, source, "remote", "add", "origin", source)
	if drift {
		if err := os.WriteFile(source+"/parent-drift.txt", []byte("parent advanced\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		claimWiringGit(t, source, "add", "parent-drift.txt")
		claimWiringGit(t, source, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "-q", "-m", "test: advance parent")
	}
	finalHead := strings.TrimSpace(claimWiringGit(t, worktree, "rev-parse", "HEAD"))
	record.Phase = model.IssueOpsPhaseDone
	record.Execution.Lease.Status = model.LeaseStatusReleased
	record.Execution.Lease.Holder = nil
	record.Execution.Lease.ClaimTokenSHA256 = ""
	record.Execution.Lease.ReleasedAt = "2026-08-03T00:00:00Z"
	record.Execution.Completion = &model.ExecutionCompletion{
		Generation: 1, FinalHead: finalHead, TuringReportPath: ".agent-harness/turing/old.json",
		Verification: []string{"old verification"}, RemoteArtifactURL: "https://gitlab.example.com/acme/repo/-/merge_requests/1", CompletedAt: "2026-08-03T00:00:00Z",
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, claimWiringActor(t), reseedWiringOwner{}
}

func TestExecutionReseedPreviewNextCommandRunsWithoutCallerRepair(t *testing.T) {
	stateRoot, record, _, _, _ := seedOrcaClaimSnapshot(t)
	actor := claimWiringActor(t)
	actor.SessionID = "claim wiring's session"
	owner := reseedWiringOwner{}
	preview, err := issueops.ReplaceExecutionWithDependencies(context.Background(), stateRoot, issueops.ExecutionReplaceRequest{
		ID: record.ID, Action: issueops.ExecutionReplacePreview, ExpectedGeneration: 1, Actor: actor, CWD: record.Execution.Workspace.SourceRoot,
	}, issueops.ExecutionReplaceDependencies{OrcaOwner: owner})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	for _, flag := range []string{"--host", "--session-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"} {
		if !strings.Contains(preview.NextCommand, flag) {
			t.Fatalf("preview next command %q does not contain %s", preview.NextCommand, flag)
		}
	}
	tokens := commandparse.SplitCommandTokens(preview.NextCommand)
	if len(tokens) < 5 || strings.Join(tokens[:4], " ") != "agent-harness issueops execution replace" {
		t.Fatalf("unexpected preview next command: %q", preview.NextCommand)
	}
	if !containsWiringTokenPair(tokens, "--session-id", actor.SessionID) {
		t.Fatalf("quoted session identity did not round-trip: %q", preview.NextCommand)
	}
	if !containsWiringTokenPair(tokens, "--cwd", record.Execution.Workspace.Root) {
		t.Fatalf("preview did not emit canonical worktree cwd: %q", preview.NextCommand)
	}
	if err := executioncmd.Run(tokens[3:], executioncmd.Deps{
		StateRoot:  func() string { return stateRoot },
		Provenance: reseedProvenanceObserver{},
		ReadIssue: func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			return port.ExecutionIssueSnapshot{URL: request.URL, Body: claimWiringIssueBody(), State: "opened"}, nil
		},
		Reseed: func(ctx context.Context, root string, request issueops.ExecutionReseedRequest) (issueops.ExecutionReplaceResult, error) {
			return issueOpsReseedHandlerWithOwner(ctx, root, request, owner)
		},
	}); err != nil {
		t.Fatalf("execute emitted preview next command without repair: %v", err)
	}
	persisted, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Lease.Generation != 2 || persisted.Execution.Lease.Status != model.LeaseStatusClaimable {
		t.Fatalf("persisted reseed=%+v", persisted.Execution.Lease)
	}
}

func containsWiringTokenPair(tokens []string, name, value string) bool {
	for index := 0; index+1 < len(tokens); index++ {
		if tokens[index] == name && tokens[index+1] == value {
			return true
		}
	}
	return false
}

type reseedWiringOwner struct{}

func (reseedWiringOwner) InspectOwner(context.Context, port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	return port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime", TaskStatus: "completed", DispatchStatus: "failed"}, nil
}
