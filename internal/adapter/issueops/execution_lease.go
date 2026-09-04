package issueops

import (
	"context"
	"fmt"
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
	// inspectWorkspace는 quiescence 판정의 워크스페이스 점유 관측이다. 기본
	// 구현은 시스템 전역 lsof라 호스트 상태와 그 3초 상한에 묶이므로, lease
	// 상태 기계를 검증하는 테스트는 결정적 관측자를 주입한다. 비공개 필드라
	// 프로덕션 경로는 항상 기본 구현을 쓴다.
	inspectWorkspace executionWorkspaceProcessInspector
}

// executionWorkspaceProcessInspector는 워크트리를 점유한 프로세스를 관측한다.
type executionWorkspaceProcessInspector func(root string, excluded map[int]bool) ([]workspaceProcess, error)

func (deps ExecutionReplaceDependencies) workspaceInspector() executionWorkspaceProcessInspector {
	if deps.inspectWorkspace != nil {
		return deps.inspectWorkspace
	}
	return inspectWorkspaceProcesses
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

// ExecutionWriterAbsentRecoveryCommand는 writer 없는 lease의 회복 명령을
// 노출한다. 본체는 아래 한 곳뿐이며, 단계 분류가 같은 문자열을 다시 만들지
// 않도록 감싸기만 한다.
func ExecutionWriterAbsentRecoveryCommand(record issueops.IssueOpsRecord) string {
	return executionWriterAbsentRecoveryCommand(record)
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
