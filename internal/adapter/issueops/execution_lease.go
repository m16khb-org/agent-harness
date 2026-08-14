package issueops

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
	basesyncport "agent-harness/internal/port/issueopsbasesync"
)

type ExecutionReplaceRequest struct {
	ID                    string               `json:"id"`
	Action                string               `json:"action"`
	ExpectedGeneration    uint64               `json:"expected_generation"`
	CompletionGeneration  uint64               `json:"completion_generation,omitempty"`
	InventoryFingerprint  string               `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string               `json:"quiescence_fingerprint,omitempty"`
	Reason                string               `json:"reason,omitempty"`
	Actor                 issueops.NativeActor `json:"actor"`
	CWD                   string               `json:"cwd"`
	Confirm               bool                 `json:"confirm,omitempty"`
}

type ExecutionReplaceDependencies struct {
	OrcaOwner port.ExecutionOrcaOwnerInspector
	BaseSync  basesyncport.Inspector
	// ReadIssue는 finalize와 reseed의 재봉인이 현재 이슈 본문을 다시 읽는
	// 경로다. orca 사이클에서 누락되면 재봉인이 fail-closed로 거부된다.
	ReadIssue ExecutionIssueSnapshotReadFunc
}

func StatusExecution(stateRoot, id string) (ExecutionResult, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return ExecutionResult{OK: false, ID: id}, err
	}
	if record.Execution == nil {
		return ExecutionResult{OK: false, ID: id}, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	result := executionResult(record)
	if record.Execution.Completion == nil {
		result.NextCommand = executionWriterAbsentRecoveryCommand(record)
	}
	return result, nil
}

func ReplaceExecution(stateRoot string, req ExecutionReplaceRequest) (ExecutionReplaceResult, error) {
	return ReplaceExecutionWithDependencies(context.Background(), stateRoot, req, ExecutionReplaceDependencies{})
}

func ReplaceExecutionWithDependencies(ctx context.Context, stateRoot string, req ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (ExecutionReplaceResult, error) {
	actor, err := normalizeNativeActor(req.Actor)
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	req.Actor = actor
	switch req.Action {
	case ExecutionReplacePreview:
		return previewExecutionReplacement(ctx, stateRoot, req, deps)
	case ExecutionReplaceFinalizePreview:
		return previewExecutionFinalization(ctx, stateRoot, req, deps)
	case ExecutionReplaceRevoke, ExecutionReplaceFinalize:
		if !req.Confirm {
			return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("%s requires confirm", req.Action)
		}
		return mutateExecutionReplacement(ctx, stateRoot, req, deps)
	default:
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("unsupported execution replace action %q", req.Action)
	}
}

func previewExecutionReplacement(ctx context.Context, stateRoot string, req ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (ExecutionReplaceResult, error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	if record.Execution == nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	// 최초 preview는 현재 세대를 알아내는 읽기 단계이므로 0을 허용한다.
	// 호출자가 세대를 명시했다면 이후 mutation과 같은 CAS 기준으로 검증한다.
	if req.ExpectedGeneration != 0 && record.Execution.Lease.Generation != req.ExpectedGeneration {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf(
			"stale lease generation: current=%d expected=%d",
			record.Execution.Lease.Generation,
			req.ExpectedGeneration,
		)
	}
	if record.Execution.Lease.Status != issueops.LeaseStatusActive && record.Execution.Lease.Status != issueops.LeaseStatusReleased && record.Execution.Lease.Status != issueops.LeaseStatusClaimable {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("replace preview is unavailable from %s", record.Execution.Lease.Status)
	}
	if err := validateExecutionReplacementCWD(record, req.CWD); err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	if err := observeCompletedExecutionBase(ctx, record, req.CompletionGeneration, deps.BaseSync); err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	fingerprint, _, err := executionInventoryFingerprint(ctx, record, req.Actor, deps)
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	result := replaceResult(record, req.Action, fingerprint, "", "")
	if record.Execution.Lease.Status == issueops.LeaseStatusReleased ||
		record.Execution.Lease.Status == issueops.LeaseStatusClaimable {
		result.NextCommand = executionReseedCommand(
			record.ID,
			record.Execution.Lease.Generation,
			req.CompletionGeneration,
			fingerprint,
			req.Actor,
			record.Execution.Workspace.Root,
		)
	}
	return result, nil
}

func observeCompletedExecutionBase(ctx context.Context, record issueops.IssueOpsRecord, selectedGeneration uint64, inspector basesyncport.Inspector) error {
	execution := record.Execution
	if execution == nil || execution.Completion == nil ||
		(execution.Lease.Status != issueops.LeaseStatusReleased && execution.Lease.Status != issueops.LeaseStatusClaimable) {
		return nil
	}
	completionGeneration := execution.Completion.Generation
	if completionGeneration == 0 {
		return fmt.Errorf("invalid or missing stamped completion generation")
	}
	if selectedGeneration != 0 && selectedGeneration != completionGeneration {
		return fmt.Errorf("completion_generation conflicts with stamped completion generation %d", completionGeneration)
	}
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseBranch) == "" {
		return fmt.Errorf("completed replacement preview requires branch_prepare.base_branch")
	}
	if inspector == nil {
		return fmt.Errorf("completed replacement preview requires base sync inspector")
	}
	receipt, err := inspector.Observe(ctx, basesyncport.Request{
		Worktree: execution.Workspace.Root, BaseBranch: record.BranchPrepare.BaseBranch,
	})
	if err != nil {
		return fmt.Errorf("observe completed execution base: %w", err)
	}
	if receipt.SyncRequired {
		return issueops.NewBaseSyncRequiredError(record.ID, completionGeneration)
	}
	return nil
}

func previewExecutionFinalization(ctx context.Context, stateRoot string, req ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (ExecutionReplaceResult, error) {
	record, err := executionRecordAtGeneration(stateRoot, req.ID, req.ExpectedGeneration)
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	if record.Execution.Lease.Status != issueops.LeaseStatusRevoking {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("finalize preview requires a revoking lease")
	}
	if err := validateExecutionReplacementCWD(record, req.CWD); err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	fingerprint, err := executionQuiescenceFingerprint(ctx, record, req.Actor, deps)
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	return replaceResult(record, req.Action, "", fingerprint, ""), nil
}

func mutateExecutionReplacement(ctx context.Context, stateRoot string, req ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (ExecutionReplaceResult, error) {
	var persisted issueops.IssueOpsRecord
	var tokenPath string
	var resealed executionOwnerReseal
	err := withIssueOpsLock(ctx, stateRoot, req.ID, func(context.Context) error {
		record, err := executionRecordAtGeneration(stateRoot, req.ID, req.ExpectedGeneration)
		if err != nil {
			return err
		}
		if record.Execution.Pending != nil {
			return fmt.Errorf("execution replacement is blocked by a pending external intent; run execution reconcile")
		}
		lease := &record.Execution.Lease
		if err := validateExecutionReplacementCWD(record, req.CWD); err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		switch req.Action {
		case ExecutionReplaceRevoke:
			if lease.Status != issueops.LeaseStatusActive || strings.TrimSpace(req.Reason) == "" {
				return fmt.Errorf("revoke requires an active lease and a reason")
			}
			if err := refuseSelfRevoke(record.ID, *lease, req.Actor); err != nil {
				return err
			}
			fingerprint, _, err := executionInventoryFingerprint(ctx, record, req.Actor, deps)
			if err != nil {
				return err
			}
			if fingerprint != req.InventoryFingerprint {
				return fmt.Errorf("stale replacement inventory fingerprint")
			}
			previous := *lease.Holder
			lease.Generation++
			lease.Status = issueops.LeaseStatusRevoking
			lease.ReplacedAt = now
			lease.ReplacementReason = strings.TrimSpace(req.Reason)
			persisted, err = persistExecutionTransition(stateRoot, record, &previous)
			return err
		case ExecutionReplaceFinalize:
			if lease.Status != issueops.LeaseStatusRevoking {
				return fmt.Errorf("finalize requires a revoking lease")
			}
			fingerprint, err := executionQuiescenceFingerprint(ctx, record, req.Actor, deps)
			if err != nil {
				return err
			}
			if fingerprint != req.QuiescenceFingerprint {
				return fmt.Errorf("stale quiescence fingerprint")
			}
			if err := cleanupReplacementGeneration(record); err != nil {
				return err
			}
			// 넘겨줄 workspace가 없으면 claimable은 사실이 아니다 — claim
			// token은 그 worktree에서 읽히도록 만들어지고 owner context
			// 재봉인도 거기에 쓴다. 아무도 claim할 수 없는 세대를 만들면서
			// 쓰기에 실패해 lease가 revoking에 갇히고, abandon은
			// claimable/released만 받으므로 회수가 막힌다(#435).
			//
			// terminal 상태인 released가 정확하다. 이 lifecycle은 폐기하거나
			// 처음부터 다시 prepare해야 하며, 두 경로 모두 released에서 열린다.
			if workspaceRootAbsent(record.Execution.Workspace.Root) {
				lease.Status = issueops.LeaseStatusReleased
				lease.Holder = nil
				lease.ClaimTokenSHA256 = ""
				persisted, err = persistExecutionTransition(stateRoot, record, nil)
				return err
			}
			token, path, err := createClaimToken(record)
			if err != nil {
				return cleanupReplacementFailure(record, err)
			}
			tokenPath = path
			lease.Status = issueops.LeaseStatusClaimable
			lease.Holder = nil
			lease.ClaimTokenSHA256 = tokenSHA256(token)
			// revoking 세대의 durable 상태는 재봉인이 모두 성공한 뒤에만
			// claimable로 바뀐다. 실패한 token은 즉시 지워 재시도 경로만 남긴다.
			reseal, err := resealOwnerContextForReplacement(ctx, stateRoot, record, deps)
			if err != nil {
				return cleanupReplacementFailure(record, err)
			}
			if binding := record.Execution.Orca; binding != nil {
				binding.LeaseGeneration = lease.Generation
				binding.ArtifactIdentityVersion = issueops.OrcaArtifactIdentityVersion
				binding.IssueBodySHA256 = reseal.issueBodySHA256
				binding.ContextPacketSHA256 = reseal.packetSHA256
				binding.OwnerPromptSHA256 = reseal.promptSHA256
			}
			resealed = reseal
			persisted, err = persistExecutionTransition(stateRoot, record, nil)
			if err != nil {
				return cleanupReplacementFailure(record, err)
			}
			return nil
		}
		return fmt.Errorf("unsupported execution replace action %q", req.Action)
	})
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	result := replaceResult(persisted, req.Action, "", "", tokenPath)
	result.IssueBodySHA256 = resealed.issueBodySHA256
	result.ContextPacketPath, result.ContextPacketSHA256 = resealed.packetPath, resealed.packetSHA256
	result.OwnerPromptPath, result.OwnerPromptSHA256 = resealed.promptPath, resealed.promptSHA256
	if persisted.Execution.Lease.Status == issueops.LeaseStatusClaimable {
		switch persisted.Execution.Mode {
		case issueops.ExecutionModeOrca:
			result.NextCommand = ExecutionResumeRecoveryCommand(persisted.ID, persisted.Execution.Lease.Generation)
		case issueops.ExecutionModeDirect:
			result.NextCommand = executionDirectClaimCommand(
				persisted.ID,
				persisted.Execution.Lease.Generation,
				tokenPath,
			)
		}
	}
	return result, nil
}

// direct replacement는 새 owner를 띄우지 않으므로 반환된 token으로 현재
// 세대를 바로 claim해야 한다. 이 명령이 없으면 holderless 복구가 중간에서
// 멈추고 다음 durable mutation이 write-lease 가드에 막힌다.
func executionDirectClaimCommand(id string, generation uint64, _ string) string {
	return "agent-harness issueops execution claim --id " + quoteExecutionOwnerArg(id) +
		" --generation " + strconv.FormatUint(generation, 10) +
		" --claim-current-token"
}

func executionReseedCommand(id string, generation, completionGeneration uint64, fingerprint string, actor issueops.NativeActor, cwd string) string {
	process := actor.SessionProcess
	command := "agent-harness issueops execution replace --id " + quoteExecutionOwnerArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10)
	if completionGeneration != 0 {
		command += " --completion-generation " + strconv.FormatUint(completionGeneration, 10)
	}
	command += " --reseed --inventory-fingerprint " + fingerprint +
		" --host " + quoteExecutionOwnerArg(actor.Host) +
		" --session-id " + quoteExecutionOwnerArg(actor.SessionID)
	if actor.AgentID != "" {
		command += " --agent-id " + quoteExecutionOwnerArg(actor.AgentID)
	}
	command += " --session-pid " + strconv.Itoa(process.PID) +
		" --session-started-at " + quoteExecutionOwnerArg(process.StartedAt) +
		" --session-executable " + quoteExecutionOwnerArg(process.Executable) +
		" --cwd " + quoteExecutionOwnerArg(cwd) + " --confirm"
	return command
}

func executionWriterAbsentRecoveryCommand(record issueops.IssueOpsRecord) string {
	if record.Execution == nil {
		return ""
	}
	lease := record.Execution.Lease
	switch lease.Status {
	case issueops.LeaseStatusClaimable:
		if record.Execution.Mode == issueops.ExecutionModeOrca {
			if !completeOrcaArtifactIdentity(record.Execution.Orca) {
				return executionReplacementPreviewCommand(record.ID, lease.Generation)
			}
			return ExecutionResumeRecoveryCommand(record.ID, lease.Generation)
		}
		if record.Execution.Mode == issueops.ExecutionModeDirect {
			return executionDirectClaimCommand(record.ID, lease.Generation, claimTokenPath(record))
		}
	case issueops.LeaseStatusReleased:
		return executionReplacementPreviewCommand(record.ID, lease.Generation)
	case issueops.LeaseStatusRevoking:
		return "agent-harness issueops execution replace --id " + quoteExecutionOwnerArg(record.ID) +
			" --expected-generation " + strconv.FormatUint(lease.Generation, 10) + " --finalize-preview"
	}
	return ""
}

func executionReplacementPreviewCommand(id string, generation uint64) string {
	return "agent-harness issueops execution replace --id " + quoteExecutionOwnerArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10) + " --preview"
}

// executionOwnerReseal은 replacement가 재봉인한 owner artifact의 정체다.
// direct 모드 사이클에서는 모든 필드가 빈 값으로 남는다(재봉인 대상이 아니다).
type executionOwnerReseal struct {
	issueBodySHA256 string
	packetPath      string
	packetSHA256    string
	promptPath      string
	promptSHA256    string
}

// resealOwnerContextForReplacement는 replacement generation과 새 claim token이
// 반영된 레코드를 기준으로 owner packet과 prompt를 다시 봉인한다.
//
// 봉인은 원래 prepare의 worktree 단계에서 단 1회 일어났고 finalize와 reseed는
// lease만 회전시켰다. 그 결과 새 세대 owner가 lease_generation과
// claim_token_file이 어긋난 낡은 packet을 읽었고, 봉인 이후 이슈 본문이
// 정당하게 개정되면 재봉인 수단이 없어 claim이 영구 거부됐다. 여기서 현재
// 이슈 본문을 다시 읽어 봉인하면 두 문제가 함께 해소된다.
//
// 원격 읽기 실패는 통과가 아니라 거부다: 낡은 packet으로 owner를 띄우는 것보다
// replacement를 멈추는 편이 안전하고 재시도로 해소된다.
func resealOwnerContextForReplacement(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, deps ExecutionReplaceDependencies) (executionOwnerReseal, error) {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return executionOwnerReseal{}, nil
	}
	if strings.TrimSpace(record.PlanPath) == "" {
		return executionOwnerReseal{}, newPlanArtifactRequiredError(record, false)
	}
	stagedPlan, err := RequireStagedExecutionOwnerPlan(stateRoot, record)
	if err != nil {
		return executionOwnerReseal{}, err
	}
	if deps.ReadIssue == nil {
		return executionOwnerReseal{}, fmt.Errorf("replacement cannot reseal the owner context without a remote issue reader")
	}
	snapshot, err := readExecutionOwnerSnapshot(ctx, record, deps.ReadIssue)
	if err != nil {
		return executionOwnerReseal{}, fmt.Errorf("replacement stopped because the remote issue could not be read for resealing: %w", err)
	}
	plan, manifest, err := materializeExecutionOwnerArtifacts(stateRoot, record)
	if err != nil {
		return executionOwnerReseal{}, err
	}
	if plan.Path != record.PlanPath || plan.Digest != stagedPlan.Digest {
		return executionOwnerReseal{}, newPlanArtifactRequiredError(record, false)
	}
	binding := record.Execution.Orca
	artifacts, err := buildExecutionOwnerArtifacts(record, ExecutionPrepareRequest{
		ID: record.ID, Mode: string(issueops.ExecutionModeOrca), OwnerHost: binding.OwnerHost,
		OwnerModel: binding.OwnerModel, OwnerEffort: binding.OwnerEffort,
	}, snapshot, manifest)
	if err != nil {
		return executionOwnerReseal{}, err
	}
	return executionOwnerReseal{
		issueBodySHA256: snapshot.issue.BodySHA256,
		packetPath:      artifacts.packetPath, packetSHA256: artifacts.packetSHA256,
		promptPath: artifacts.promptPath, promptSHA256: artifacts.promptSHA256,
	}, nil
}

func executionRecordAtGeneration(stateRoot, id string, generation uint64) (issueops.IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Execution == nil {
		return record, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	if generation == 0 || record.Execution.Lease.Generation != generation {
		return record, fmt.Errorf("stale lease generation: current=%d expected=%d", record.Execution.Lease.Generation, generation)
	}
	return record, nil
}

func executionResult(record issueops.IssueOpsRecord) ExecutionResult {
	return ExecutionResult{OK: true, ID: record.ID, Execution: *record.Execution}
}

func replaceResult(record issueops.IssueOpsRecord, action, inventory, quiescence, tokenPath string) ExecutionReplaceResult {
	return ExecutionReplaceResult{
		OK: true, ID: record.ID, Action: action, Execution: *record.Execution,
		InventoryFingerprint: inventory, QuiescenceFingerprint: quiescence, ClaimTokenPath: tokenPath,
	}
}

func normalizeNativeActor(actor issueops.NativeActor) (issueops.NativeActor, error) {
	actor.Host = strings.ToLower(strings.TrimSpace(actor.Host))
	actor.SessionID = strings.TrimSpace(actor.SessionID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	if actor.SessionProcess != nil {
		receipt := *actor.SessionProcess
		receipt.StartedAt = strings.TrimSpace(receipt.StartedAt)
		receipt.Executable = strings.TrimSpace(receipt.Executable)
		actor.SessionProcess = &receipt
	}
	actor.ProcessAncestry = append([]issueops.NativeProcessReceipt(nil), actor.ProcessAncestry...)
	if err := issueops.ValidateNativeActor(actor); err != nil {
		return actor, err
	}
	locallyObserved := false
	for _, receipt := range actor.ProcessAncestry {
		if actor.SessionProcess != nil && receipt == *actor.SessionProcess {
			locallyObserved = true
			break
		}
	}
	if !locallyObserved {
		return actor, fmt.Errorf("native session process receipt is not in the local process ancestry")
	}
	if err := requireExactLiveNativeProcessReceipt(*actor.SessionProcess); err != nil {
		return actor, err
	}
	return actor, nil
}

// refuseSelfRevoke는 살아 있는 홀더가 자기 lease를 revoke하는 것을 막는다.
//
// revoke의 존재 이유는 응답 없는 홀더에게서 제3자가 lease를 뺏는 것이다. 그런데
// 홀더 자신이 호출하면 나갈 문이 전부 막힌다: release는 active를, reseed는
// released/claimable을, claim은 claimable을 요구하고, finalize는 이전 홀더가
// dead여야 한다 — 그 홀더가 나 자신이므로 내가 죽어야만 풀린다(이슈 #170).
//
// 홀더가 원한 것은 lease 교체이고 release가 그것을 준다. 그래서 거부만 하지
// 않고 그 명령을 안내한다.
//
// 생존 판정은 finalize가 쓰는 inspectNativeProcessReceipt와 같은 함수다. 두 곳이
// 같은 기준을 봐야 한쪽은 revoke를 막는데 다른 쪽은 finalize를 막는 교착이
// 생기지 않는다. 판정이 실패하거나 live가 아니면 통과시킨다 — 그것이 지금
// 동작이고, 죽은 홀더 뺏기와 제3자 revoke를 막지 않는다.
func refuseSelfRevoke(lifecycleID string, lease issueops.WriteLease, requester issueops.NativeActor) error {
	holder := lease.Holder
	if holder == nil || !sameNativeActorIdentity(holder, &requester) || holder.SessionProcess == nil {
		return nil
	}
	status, _, err := inspectNativeProcessReceipt(*holder.SessionProcess)
	if err != nil || status != NativeProcessStatusLive {
		return nil
	}
	return fmt.Errorf(
		"revoke takes a lease away from an unresponsive holder, but this session is the live holder: "+
			"revoking your own lease leaves no exit because finalize requires the old holder to be dead. "+
			"Run `agent-harness issueops execution release --id %s --generation %d` instead",
		strings.TrimSpace(lifecycleID), lease.Generation)
}

func sameNativeActor(a, b *issueops.NativeActor) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return sameNativeActorIdentity(a, b) && sameNativeProcessReceipt(a.SessionProcess, b.SessionProcess)
}

func sameNativeActorIdentity(a, b *issueops.NativeActor) bool {
	return a != nil && b != nil && strings.EqualFold(a.Host, b.Host) && a.SessionID == b.SessionID && a.AgentID == b.AgentID
}

func sameNativeProcessReceipt(a, b *issueops.NativeProcessReceipt) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func samePath(a, b string) bool {
	left, err := filepath.Abs(strings.TrimSpace(a))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(left); resolveErr == nil {
		left = resolved
	}
	right, err := filepath.Abs(strings.TrimSpace(b))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(right); resolveErr == nil {
		right = resolved
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func executionInventoryFingerprint(ctx context.Context, record issueops.IssueOpsRecord, requester issueops.NativeActor, deps ExecutionReplaceDependencies) (string, port.ExecutionOrcaOwnerInventory, error) {
	snapshot, err := workspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", port.ExecutionOrcaOwnerInventory{}, err
	}
	processSnapshot, _ := observeNativeProcessSnapshot()
	processStatus, orcaInventory, err := executionOwnerInventory(ctx, record, deps, processSnapshot)
	if err != nil {
		return "", port.ExecutionOrcaOwnerInventory{}, err
	}
	if err := validateExecutionRuntimeRolloverWithProcess(record, processStatus, orcaInventory); err != nil {
		return "", port.ExecutionOrcaOwnerInventory{}, err
	}
	payload := struct {
		ID         string                           `json:"id"`
		Generation uint64                           `json:"generation"`
		Status     issueops.LeaseStatus             `json:"status"`
		Holder     *issueops.NativeActor            `json:"holder,omitempty"`
		Requester  issueops.NativeActor             `json:"requester"`
		Process    string                           `json:"process_status"`
		Orca       port.ExecutionOrcaOwnerInventory `json:"orca"`
		Snapshot   string                           `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, record.Execution.Lease.Status, record.Execution.Lease.Holder, requester, processStatus, orcaInventory, snapshot}
	fingerprint, err := hashJSON(payload)
	return fingerprint, orcaInventory, err
}

func executionQuiescenceFingerprint(ctx context.Context, record issueops.IssueOpsRecord, requester issueops.NativeActor, deps ExecutionReplaceDependencies) (string, error) {
	holder := record.Execution.Lease.Holder
	if holder == nil || holder.SessionProcess == nil {
		return "", fmt.Errorf("revoking lease is missing its old process receipt")
	}
	processSnapshot, _ := observeNativeProcessSnapshot()
	processStatus, _, err := inspectNativeProcessReceiptForRollover(*holder.SessionProcess, processSnapshot)
	if err != nil {
		return "", err
	}
	if processStatus == "live" {
		return "", fmt.Errorf("old holder process is still live: pid=%d executable=%s", holder.SessionProcess.PID, holder.SessionProcess.Executable)
	}
	if processStatus != "dead" {
		return "", fmt.Errorf("old holder process identity is unsafe to finalize: pid=%d status=%s", holder.SessionProcess.PID, processStatus)
	}
	orcaInventory, err := executionOrcaOwnerInventory(ctx, record, deps, processStatus)
	if err != nil {
		return "", err
	}
	if err := validateExecutionRuntimeRolloverWithProcess(record, processStatus, orcaInventory); err != nil {
		return "", err
	}
	if !deadOwnerRuntimeRollover(record, processStatus, orcaInventory) && (orcaInventory.TerminalLive || orcaInventory.TaskLive) {
		return "", fmt.Errorf("Orca owner is not quiescent: terminal_live=%t task_live=%t task_status=%s dispatch_status=%s", orcaInventory.TerminalLive, orcaInventory.TaskLive, orcaInventory.TaskStatus, orcaInventory.DispatchStatus)
	}
	inventoryOwners := map[int]bool{os.Getpid(): true}
	requesterOwners := map[int]bool{}
	if requester.SessionProcess != nil {
		inventoryOwners[requester.SessionProcess.PID] = true
		requesterOwners[requester.SessionProcess.PID] = true
	}
	excluded := map[int]bool{}
	for pid := range inventoryOwners {
		for ancestor := range nativeProcessAncestryPIDsFromSnapshot(processSnapshot, pid) {
			excluded[ancestor] = true
		}
	}
	workspaceProcesses, err := inspectWorkspaceProcesses(record.Execution.Workspace.Root, excluded)
	if err != nil {
		return "", err
	}
	workspaceProcesses = dropRequesterOwnedProcessesFromSnapshot(
		workspaceProcesses,
		requesterOwners,
		processSnapshot,
	)
	if len(workspaceProcesses) > 0 {
		process := workspaceProcesses[0]
		return "", fmt.Errorf("workspace process is not quiescent: pid=%d command=%s fd=%s access=%s path=%s", process.PID, process.Command, process.FD, process.Access, process.Path)
	}
	snapshot, err := workspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", err
	}
	payload := struct {
		ID         string                           `json:"id"`
		Generation uint64                           `json:"generation"`
		Holder     issueops.NativeActor             `json:"holder"`
		Requester  issueops.NativeActor             `json:"requester"`
		Process    issueops.NativeProcessReceipt    `json:"process"`
		Orca       port.ExecutionOrcaOwnerInventory `json:"orca"`
		Snapshot   string                           `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, *holder, requester, *holder.SessionProcess, orcaInventory, snapshot}
	return hashJSON(payload)
}

func executionOwnerInventory(
	ctx context.Context,
	record issueops.IssueOpsRecord,
	deps ExecutionReplaceDependencies,
	processSnapshot map[int]nativeProcessSnapshotEntry,
) (string, port.ExecutionOrcaOwnerInventory, error) {
	status := "none"
	if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil {
		var err error
		status, _, err = inspectNativeProcessReceiptForRollover(*holder.SessionProcess, processSnapshot)
		if err != nil {
			return "", port.ExecutionOrcaOwnerInventory{}, err
		}
	}
	inventory, err := executionOrcaOwnerInventory(ctx, record, deps, status)
	return status, inventory, err
}

func inspectNativeProcessReceiptForRollover(
	receipt issueops.NativeProcessReceipt,
	processSnapshot map[int]nativeProcessSnapshotEntry,
) (string, issueops.NativeProcessReceipt, error) {
	if processSnapshot != nil {
		return inspectNativeProcessReceiptFromSnapshot(receipt, processSnapshot)
	}
	return inspectNativeProcessReceipt(receipt)
}

func executionOrcaOwnerInventory(
	ctx context.Context,
	record issueops.IssueOpsRecord,
	deps ExecutionReplaceDependencies,
	status string,
) (port.ExecutionOrcaOwnerInventory, error) {
	if record.Execution.Mode != issueops.ExecutionModeOrca {
		return port.ExecutionOrcaOwnerInventory{}, nil
	}
	if record.Execution.Orca == nil || deps.OrcaOwner == nil {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca execution requires exact owner terminal and task inventory")
	}
	binding := record.Execution.Orca
	inventory, err := deps.OrcaOwner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, RunID: binding.RunID, TaskID: binding.TaskID,
		DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
		AllowRuntimeRollover: allowExecutionRuntimeRollover(record, status),
	})
	return inventory, err
}

func allowExecutionRuntimeRollover(record issueops.IssueOpsRecord, processStatus string) bool {
	lease := record.Execution.Lease
	if lease.Holder == nil {
		return lease.Status == issueops.LeaseStatusReleased || lease.Status == issueops.LeaseStatusClaimable
	}
	// Adapter가 바뀐 runtime을 읽는 권한은 core가 확인한 exact holder process의
	// 종료 영수증에만 묶는다. lease 상태만으로 허용하면 live owner와 경쟁할 수 있다.
	return processStatus == NativeProcessStatusDead &&
		(lease.Status == issueops.LeaseStatusActive || lease.Status == issueops.LeaseStatusRevoking)
}

func deadOwnerRuntimeRollover(record issueops.IssueOpsRecord, processStatus string, inventory port.ExecutionOrcaOwnerInventory) bool {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return false
	}
	lease := record.Execution.Lease
	sealed := strings.TrimSpace(record.Execution.Orca.RuntimeID)
	observed := strings.TrimSpace(inventory.RuntimeID)
	return lease.Holder != nil && lease.Holder.SessionProcess != nil &&
		(lease.Status == issueops.LeaseStatusActive || lease.Status == issueops.LeaseStatusRevoking) &&
		processStatus == NativeProcessStatusDead && observed != "" && observed != sealed &&
		inventory.TerminalID == "" && !inventory.TerminalLive
}

func validateExecutionRuntimeRolloverWithProcess(record issueops.IssueOpsRecord, processStatus string, inventory port.ExecutionOrcaOwnerInventory) error {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return nil
	}
	sealed := strings.TrimSpace(record.Execution.Orca.RuntimeID)
	observed := strings.TrimSpace(inventory.RuntimeID)
	if observed == "" || observed == sealed {
		return nil
	}
	if deadOwnerRuntimeRollover(record, processStatus, inventory) {
		return nil
	}
	lease := record.Execution.Lease
	holderless := lease.Holder == nil && (lease.Status == issueops.LeaseStatusReleased || lease.Status == issueops.LeaseStatusClaimable)
	taskSettled := inventory.TaskStatus == "completed" || inventory.TaskStatus == "failed"
	dispatchSettled := inventory.DispatchStatus == "completed" || inventory.DispatchStatus == "failed" || inventory.DispatchStatus == "circuit_broken"
	if !holderless || inventory.TerminalID != "" || inventory.TerminalLive || inventory.TaskLive || !taskSettled || !dispatchSettled {
		return fmt.Errorf(
			"Orca runtime rollover owner is not quiescent: terminal_id=%s terminal_live=%t task_live=%t task_status=%s dispatch_status=%s",
			inventory.TerminalID, inventory.TerminalLive, inventory.TaskLive, inventory.TaskStatus, inventory.DispatchStatus,
		)
	}
	return nil
}

func validateExecutionReplacementCWD(record issueops.IssueOpsRecord, cwd string) error {
	workspace := record.Execution.Workspace
	if !samePath(cwd, workspace.SourceRoot) && !samePath(cwd, workspace.Root) {
		return fmt.Errorf("execution replace cwd must be source_root or the canonical worktree")
	}
	return nil
}

func workspaceSnapshot(workspace issueops.Workspace) (string, error) {
	info, err := os.Lstat(workspace.Root)
	if errors.Is(err, os.ErrNotExist) {
		// 부재는 quiescence의 약한 증거가 아니라 가장 강한 증거다. 존재하지
		// 않는 디렉터리에는 프로세스가 cwd를 둘 수 없고 쓸 것도 없다.
		//
		// 예전에는 이것을 "관측 불가"로 거부했고, 그래서 worktree가 lease
		// active 상태에서 제거된 lifecycle은 어떤 typed 경로로도 회수할 수
		// 없었다 — replace는 worktree를, abandon은 terminal lease를 요구하고,
		// terminal로 만들려면 replace가 필요하다(#435).
		//
		// 부재라는 사실 자체를 경로에 결속해 봉인한다. finalize 직전에
		// worktree가 되살아나면 fingerprint가 달라져 stale로 멈춘다.
		return workspaceAbsenceSnapshot(workspace), nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		// symlink나 파일이 그 경로를 차지한 것은 부재가 아니라 정체 불명이다.
		return "", fmt.Errorf("canonical worktree must be a real directory")
	}
	top, err := gitOutput(workspace.Root, "rev-parse", "--show-toplevel")
	if err != nil || !samePath(top, workspace.Root) {
		return "", fmt.Errorf("canonical worktree root does not match Git top-level")
	}
	branch, err := gitOutput(workspace.Root, "branch", "--show-current")
	if err != nil || branch != workspace.Branch {
		return "", fmt.Errorf("canonical worktree branch mismatch")
	}
	head, err := gitOutput(workspace.Root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	commonDir, err := gitOutput(workspace.Root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	indexPath, err := gitOutput(workspace.Root, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(workspace.Root, indexPath)
	}
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	_, tracked, stderr := GitCmdRaw(workspace.Root, "diff", "--binary", "--no-ext-diff", "--")
	if stderr != "" {
		return "", fmt.Errorf("read tracked diff: %s", strings.TrimSpace(stderr))
	}
	_, staged, stderr := GitCmdRaw(workspace.Root, "diff", "--cached", "--binary", "--no-ext-diff", "--")
	if stderr != "" {
		return "", fmt.Errorf("read staged diff: %s", strings.TrimSpace(stderr))
	}
	code, untrackedRaw, stderr := GitCmdRaw(workspace.Root, "ls-files", "--others", "--exclude-standard", "-z")
	if code != 0 {
		return "", fmt.Errorf("list untracked files: %s", strings.TrimSpace(stderr))
	}
	untracked := strings.Split(strings.TrimSuffix(untrackedRaw, "\x00"), "\x00")
	if len(untracked) == 1 && untracked[0] == "" {
		untracked = nil
	}
	sort.Strings(untracked)
	hash := sha256.New()
	writeFingerprintPart(hash, workspace.Root)
	writeFingerprintPart(hash, commonDir)
	writeFingerprintPart(hash, branch)
	writeFingerprintPart(hash, head)
	writeFingerprintBytes(hash, indexBytes)
	writeFingerprintPart(hash, tracked)
	writeFingerprintPart(hash, staged)
	for _, relative := range untracked {
		if filepath.IsAbs(relative) || strings.HasPrefix(filepath.Clean(relative), ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("unsafe untracked path %q", relative)
		}
		path := filepath.Join(workspace.Root, filepath.FromSlash(relative))
		entry, err := os.Lstat(path)
		if err != nil || entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
			return "", fmt.Errorf("untracked path must be a regular file: %s", relative)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		writeFingerprintPart(hash, relative)
		writeFingerprintPart(hash, entry.Mode().String())
		writeFingerprintBytes(hash, content)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func gitOutput(root string, args ...string) (string, error) {
	code, stdout, stderr := GitCmd(root, args...)
	if code != 0 {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), stderr)
	}
	return strings.TrimSpace(stdout), nil
}

type fingerprintWriter interface {
	Write([]byte) (int, error)
}

func writeFingerprintPart(hash fingerprintWriter, value string) {
	writeFingerprintBytes(hash, []byte(value))
}

func writeFingerprintBytes(hash fingerprintWriter, value []byte) {
	_, _ = hash.Write([]byte(strconv.Itoa(len(value))))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(value)
	_, _ = hash.Write([]byte{0})
}

func hashJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func claimTokenPath(record issueops.IssueOpsRecord) string {
	key := tokenSHA256(record.ID)[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "state", "issueops-v1", key, fmt.Sprintf("lease-%d.token", record.Execution.Lease.Generation))
}

func createClaimToken(record issueops.IssueOpsRecord) (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(raw)
	path := claimTokenPath(record)
	if err := secureMkdirAll(record.Execution.Workspace.Root, filepath.Dir(path)); err != nil {
		return "", "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", err
	}
	_, writeErr := file.WriteString(token + "\n")
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return "", "", writeErr
		}
		return "", "", closeErr
	}
	return token, path, nil
}

func readExecutionLeaseToken(record issueops.IssueOpsRecord, path string) (string, error) {
	expected := claimTokenPath(record)
	if !samePath(path, expected) {
		return "", fmt.Errorf("claim_token_file must be the deterministic current-generation path")
	}
	info, err := os.Lstat(expected)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return "", fmt.Errorf("claim token file must be a 0600 regular file")
	}
	if info.Size() > 256 {
		return "", fmt.Errorf("claim token file is oversized")
	}
	data, err := os.ReadFile(expected)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("claim token file is empty")
	}
	return token, nil
}

func tokenSHA256(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// cleanupReplacementGeneration은 durable lease가 아직 권한을 부여하지 않은
// target generation의 harness-owned 파일만 지운다. finalize는 revoking 세대,
// reseed는 아직 persist되지 않은 다음 세대이므로 재시도 전에 회수해도 된다.
func cleanupReplacementGeneration(record issueops.IssueOpsRecord) error {
	if record.Execution == nil {
		return fmt.Errorf("cannot clean replacement residue without an execution")
	}
	paths := []string{claimTokenPath(record)}
	if record.Execution.Mode == issueops.ExecutionModeOrca && record.Execution.Orca != nil {
		packetPath, promptPath := executionOwnerArtifactPaths(record)
		paths = append(paths, packetPath, promptPath)
	}
	for _, path := range paths {
		if err := removeReplacementRuntimeFile(record.Execution.Workspace.Root, path); err != nil {
			return fmt.Errorf("clean uncommitted replacement residue %s: %w", path, err)
		}
	}
	return nil
}

func cleanupReplacementFailure(record issueops.IssueOpsRecord, cause error) error {
	if cleanupErr := cleanupReplacementGeneration(record); cleanupErr != nil {
		return fmt.Errorf("%w; replacement residue cleanup failed: %v", cause, cleanupErr)
	}
	return cause
}

func removeReplacementRuntimeFile(root, path string) error {
	// worktree가 통째로 없으면 지울 잔여물도 없다. 부모 디렉터리를 만들려
	// 시도하면 없는 worktree를 되살리려다 실패해 finalize가 멈춘다 — 그러면
	// lease가 revoking에 갇히고 abandon은 claimable/released만 받으므로 다시
	// 막다른 길이 된다(#435).
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := secureMkdirAll(root, filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("replacement residue path is a directory")
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return nil
}

func secureMkdirAll(root, target string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("runtime token directory escapes the canonical worktree")
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(current, 0o700); err != nil {
				return err
			}
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("runtime token directory must contain only real directories")
		}
	}
	return nil
}

// workspaceAbsenceSnapshot은 canonical worktree가 없다는 관측을 정확한 경로에
// 결속해 봉인한다. 서로 다른 부재가 같은 값을 내면 다른 lifecycle의 증거를
// 재사용할 수 있으므로 경로와 branch를 함께 넣는다.
func workspaceAbsenceSnapshot(workspace issueops.Workspace) string {
	hash := sha256.New()
	writeFingerprintPart(hash, "workspace-absent")
	writeFingerprintPart(hash, workspace.Root)
	writeFingerprintPart(hash, workspace.Branch)
	return hex.EncodeToString(hash.Sum(nil))
}

// workspaceRootAbsent는 canonical worktree가 사라졌는지 보고한다. symlink나
// 파일이 그 경로를 차지한 경우는 부재가 아니라 정체 불명이므로 false다 —
// 그 상태는 workspaceSnapshot이 별도로 거부한다.
func workspaceRootAbsent(root string) bool {
	_, err := os.Lstat(root)
	return errors.Is(err, os.ErrNotExist)
}
