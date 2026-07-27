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

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

const (
	ExecutionReplacePreview         = "preview"
	ExecutionReplaceRevoke          = "revoke"
	ExecutionReplaceFinalizePreview = "finalize-preview"
	ExecutionReplaceFinalize        = "finalize"
	ExecutionReplaceReseed          = "reseed"
)

type ExecutionResult struct {
	OK        bool            `json:"ok"`
	ID        string          `json:"id"`
	Execution model.Execution `json:"execution"`
	// OrcaTaskSettled와 OrcaTaskError는 완료가 orca task를 terminal 상태로
	// 옮겼는지를 보고한다. 종결은 best-effort이므로 실패해도 완료 자체는
	// 성공이며, 침묵하면 진단이 불가능하므로 사유를 남긴다(#130).
	OrcaTaskSettled bool   `json:"orca_task_settled,omitempty"`
	OrcaTaskError   string `json:"orca_task_error,omitempty"`
	NextCommand     string `json:"next_command,omitempty"`
}

type ExecutionClaimRequest struct {
	ID                  string            `json:"id"`
	Generation          uint64            `json:"generation"`
	Actor               model.NativeActor `json:"actor"`
	CWD                 string            `json:"cwd"`
	TokenFile           string            `json:"claim_token_file"`
	IssueBodySHA256     string            `json:"issue_body_sha256,omitempty"`
	ContextPacketSHA256 string            `json:"context_packet_sha256,omitempty"`
}

type ExecutionClaimDependencies struct {
	ReadIssue ExecutionIssueSnapshotReadFunc
}

type ExecutionReleaseRequest struct {
	ID         string            `json:"id"`
	Generation uint64            `json:"generation"`
	Actor      model.NativeActor `json:"actor"`
	CWD        string            `json:"cwd"`
}

type ExecutionReplaceRequest struct {
	ID                    string            `json:"id"`
	Action                string            `json:"action"`
	ExpectedGeneration    uint64            `json:"expected_generation"`
	InventoryFingerprint  string            `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string            `json:"quiescence_fingerprint,omitempty"`
	Reason                string            `json:"reason,omitempty"`
	Actor                 model.NativeActor `json:"actor"`
	CWD                   string            `json:"cwd"`
	Confirm               bool              `json:"confirm,omitempty"`
}

type ExecutionReplaceResult struct {
	OK                    bool            `json:"ok"`
	ID                    string          `json:"id"`
	Action                string          `json:"action"`
	Execution             model.Execution `json:"execution"`
	InventoryFingerprint  string          `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string          `json:"quiescence_fingerprint,omitempty"`
	ClaimTokenPath        string          `json:"claim_token_path,omitempty"`
	// 아래 네 값은 reseed가 새 generation으로 재봉인한 owner artifact의 정체다.
	// owner는 이 digest들을 claim 명령에 그대로 넣어야 하므로 결과에 노출한다.
	IssueBodySHA256     string `json:"issue_body_sha256,omitempty"`
	ContextPacketPath   string `json:"context_packet_path,omitempty"`
	ContextPacketSHA256 string `json:"context_packet_sha256,omitempty"`
	OwnerPromptPath     string `json:"owner_prompt_path,omitempty"`
	OwnerPromptSHA256   string `json:"owner_prompt_sha256,omitempty"`
	NextCommand         string `json:"next_command,omitempty"`
}

type ExecutionReplaceDependencies struct {
	OrcaOwner port.ExecutionOrcaOwnerInspector
	// ReadIssue는 reseed의 재봉인이 현재 이슈 본문을 다시 읽는 경로다. orca
	// 사이클에서 누락되면 재봉인이 fail-closed로 거부된다.
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
	return executionResult(record), nil
}

func ClaimExecutionWithDependencies(ctx context.Context, stateRoot string, req ExecutionClaimRequest, deps ExecutionClaimDependencies) (ExecutionResult, error) {
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	validatePacket, err := validateExecutionClaimContext(ctx, stateRoot, req, deps)
	if err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	return claimExecution(stateRoot, req, validatePacket)
}

func claimExecution(stateRoot string, req ExecutionClaimRequest, validatePacket ...func(IssueOpsRecord) error) (ExecutionResult, error) {
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	actor, err := normalizeNativeActor(req.Actor)
	if err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, req.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if record.Execution == nil {
			return fmt.Errorf("IssueOps execution v1 is not prepared")
		}
		lease := &record.Execution.Lease
		if lease.Status == model.LeaseStatusActive && lease.Generation == req.Generation && sameNativeActor(lease.Holder, &actor) {
			persisted = record
			return nil
		}
		if lease.Status != model.LeaseStatusClaimable || lease.Generation != req.Generation {
			return fmt.Errorf("lease is not claimable at generation %d", req.Generation)
		}
		if !samePath(req.CWD, record.Execution.Workspace.Root) {
			return fmt.Errorf("claim cwd must be the canonical worktree")
		}
		for _, validate := range validatePacket {
			if validate != nil {
				if err := validate(record); err != nil {
					return err
				}
			}
		}
		token, err := readClaimToken(record, req.TokenFile)
		if err != nil {
			return err
		}
		if tokenSHA256(token) != lease.ClaimTokenSHA256 {
			return fmt.Errorf("claim token does not match the current generation")
		}
		lease.Status = model.LeaseStatusActive
		lease.Holder = &actor
		lease.ClaimTokenSHA256 = ""
		lease.ClaimedAt = time.Now().UTC().Format(time.RFC3339Nano)
		lease.ReleasedAt = ""
		persisted, err = persistExecutionTransition(stateRoot, record, nil)
		if err != nil {
			return err
		}
		// 상태가 active가 된 뒤 남은 파일은 hash가 비어 재사용할 수 없다.
		_ = os.Remove(req.TokenFile)
		return nil
	})
	if err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	return executionResult(persisted), nil
}

func ReleaseExecution(stateRoot string, req ExecutionReleaseRequest) (ExecutionResult, error) {
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	actor, err := normalizeNativeActor(req.Actor)
	if err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, req.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if record.Execution == nil {
			return fmt.Errorf("IssueOps execution v1 is not prepared")
		}
		lease := &record.Execution.Lease
		if lease.Status != model.LeaseStatusActive || lease.Generation != req.Generation || !sameNativeActor(lease.Holder, &actor) {
			return fmt.Errorf("only the current holder may release generation %d", req.Generation)
		}
		if !samePath(req.CWD, record.Execution.Workspace.Root) {
			return fmt.Errorf("release cwd must be the canonical worktree")
		}
		previous := *lease.Holder
		lease.Status = model.LeaseStatusReleased
		lease.Holder = nil
		lease.ClaimTokenSHA256 = ""
		lease.ReleasedAt = time.Now().UTC().Format(time.RFC3339Nano)
		persisted, err = persistExecutionTransition(stateRoot, record, &previous)
		return err
	})
	if err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	return executionResult(persisted), nil
}

func ReplaceExecution(stateRoot string, req ExecutionReplaceRequest) (ExecutionReplaceResult, error) {
	return ReplaceExecutionWithDependencies(context.Background(), stateRoot, req, ExecutionReplaceDependencies{})
}

func ReplaceExecutionWithDependencies(ctx context.Context, stateRoot string, req ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (ExecutionReplaceResult, error) {
	if req.Action == ExecutionReplaceRevoke || req.Action == ExecutionReplaceFinalize || req.Action == ExecutionReplaceReseed {
		if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
			return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
		}
	}
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
	case ExecutionReplaceRevoke, ExecutionReplaceFinalize, ExecutionReplaceReseed:
		if !req.Confirm {
			return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("%s requires confirm", req.Action)
		}
		return mutateExecutionReplacement(ctx, stateRoot, req, deps)
	default:
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("unsupported execution replace action %q", req.Action)
	}
}

func previewExecutionReplacement(ctx context.Context, stateRoot string, req ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (ExecutionReplaceResult, error) {
	record, err := executionRecordAtGeneration(stateRoot, req.ID, req.ExpectedGeneration)
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	if record.Execution.Lease.Status != model.LeaseStatusActive && record.Execution.Lease.Status != model.LeaseStatusReleased && record.Execution.Lease.Status != model.LeaseStatusClaimable {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, fmt.Errorf("replace preview is unavailable from %s", record.Execution.Lease.Status)
	}
	if err := validateExecutionReplacementCWD(record, req.CWD); err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	fingerprint, err := executionInventoryFingerprint(ctx, record, req.Actor, deps)
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	return replaceResult(record, req.Action, fingerprint, "", ""), nil
}

func previewExecutionFinalization(ctx context.Context, stateRoot string, req ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (ExecutionReplaceResult, error) {
	record, err := executionRecordAtGeneration(stateRoot, req.ID, req.ExpectedGeneration)
	if err != nil {
		return ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.Action}, err
	}
	if record.Execution.Lease.Status != model.LeaseStatusRevoking {
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
	var persisted IssueOpsRecord
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
			if lease.Status != model.LeaseStatusActive || strings.TrimSpace(req.Reason) == "" {
				return fmt.Errorf("revoke requires an active lease and a reason")
			}
			if err := refuseSelfRevoke(record.ID, *lease, req.Actor); err != nil {
				return err
			}
			fingerprint, err := executionInventoryFingerprint(ctx, record, req.Actor, deps)
			if err != nil {
				return err
			}
			if fingerprint != req.InventoryFingerprint {
				return fmt.Errorf("stale replacement inventory fingerprint")
			}
			previous := *lease.Holder
			lease.Generation++
			lease.Status = model.LeaseStatusRevoking
			lease.ReplacedAt = now
			lease.ReplacementReason = strings.TrimSpace(req.Reason)
			persisted, err = persistExecutionTransition(stateRoot, record, &previous)
			return err
		case ExecutionReplaceFinalize:
			if lease.Status != model.LeaseStatusRevoking {
				return fmt.Errorf("finalize requires a revoking lease")
			}
			fingerprint, err := executionQuiescenceFingerprint(ctx, record, req.Actor, deps)
			if err != nil {
				return err
			}
			if fingerprint != req.QuiescenceFingerprint {
				return fmt.Errorf("stale quiescence fingerprint")
			}
			token, path, err := createClaimToken(record)
			if err != nil {
				return err
			}
			tokenPath = path
			lease.Status = model.LeaseStatusClaimable
			lease.Holder = nil
			lease.ClaimTokenSHA256 = tokenSHA256(token)
			persisted, err = persistExecutionTransition(stateRoot, record, nil)
			if err != nil {
				_ = os.Remove(path)
			}
			return err
		case ExecutionReplaceReseed:
			if lease.Status != model.LeaseStatusReleased && lease.Status != model.LeaseStatusClaimable {
				return fmt.Errorf("reseed requires a released or claimable lease")
			}
			fingerprint, err := executionInventoryFingerprint(ctx, record, req.Actor, deps)
			if err != nil {
				return err
			}
			if fingerprint != req.InventoryFingerprint {
				return fmt.Errorf("stale replacement inventory fingerprint")
			}
			removeClaimTokenIfPresent(record)
			lease.Generation++
			token, path, err := createClaimToken(record)
			if err != nil {
				return err
			}
			tokenPath = path
			lease.Status = model.LeaseStatusClaimable
			lease.Holder = nil
			lease.ClaimTokenSHA256 = tokenSHA256(token)
			lease.ReplacedAt = now
			lease.ReplacementReason = strings.TrimSpace(req.Reason)
			// 재봉인은 persist 이전에 수행한다: 실패하면 generation이 올라간
			// 상태만 남고 packet이 없는 중간 상태가 생기므로, 새 token을
			// 정리하고 아무것도 기록하지 않는다(brooks 반론 수용).
			reseal, err := resealOwnerContextForReseed(ctx, stateRoot, record, deps)
			if err != nil {
				_ = os.Remove(path)
				return err
			}
			resealed = reseal
			persisted, err = persistExecutionTransition(stateRoot, record, nil)
			if err != nil {
				_ = os.Remove(path)
			}
			return err
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
	return result, nil
}

// executionOwnerReseal은 reseed가 재봉인한 owner artifact의 정체다. direct 모드
// 사이클에서는 모든 필드가 빈 값으로 남는다(재봉인 대상이 아니다).
type executionOwnerReseal struct {
	issueBodySHA256 string
	packetPath      string
	packetSHA256    string
	promptPath      string
	promptSHA256    string
}

// resealOwnerContextForReseed는 새 generation과 새 claim token이 반영된 레코드를
// 기준으로 owner packet과 prompt를 다시 봉인한다.
//
// 봉인은 원래 prepare의 worktree 단계에서 단 1회 일어났고 reseed는 lease만
// 회전시켰다. 그 결과 새 세대 owner가 lease_generation과 claim_token_file이
// 어긋난 낡은 packet을 읽었고, 봉인 이후 이슈 본문이 정당하게 개정되면 재봉인
// 수단이 없어 claim이 영구 거부됐다. 여기서 현재 이슈 본문을 다시 읽어 봉인하면
// 두 문제가 함께 해소된다.
//
// 원격 읽기 실패는 통과가 아니라 거부다: 낡은 packet으로 owner를 띄우는 것보다
// reseed를 멈추는 편이 안전하고 재시도로 해소된다.
func resealOwnerContextForReseed(ctx context.Context, stateRoot string, record IssueOpsRecord, deps ExecutionReplaceDependencies) (executionOwnerReseal, error) {
	if record.Execution == nil || record.Execution.Mode != model.ExecutionModeOrca || record.Execution.Orca == nil {
		return executionOwnerReseal{}, nil
	}
	if deps.ReadIssue == nil {
		return executionOwnerReseal{}, fmt.Errorf("reseed cannot reseal the owner context without a remote issue reader")
	}
	snapshot, err := readExecutionOwnerSnapshot(ctx, record, deps.ReadIssue)
	if err != nil {
		return executionOwnerReseal{}, fmt.Errorf("reseed stopped because the remote issue could not be read for resealing: %w", err)
	}
	manifest, err := materializeStagedArtifacts(stateRoot, record)
	if err != nil {
		return executionOwnerReseal{}, err
	}
	binding := record.Execution.Orca
	artifacts, err := buildExecutionOwnerArtifacts(record, ExecutionPrepareRequest{
		ID: record.ID, Mode: string(model.ExecutionModeOrca), OwnerHost: binding.OwnerHost,
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

func executionRecordAtGeneration(stateRoot, id string, generation uint64) (IssueOpsRecord, error) {
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

func executionResult(record IssueOpsRecord) ExecutionResult {
	return ExecutionResult{OK: true, ID: record.ID, Execution: *record.Execution}
}

func replaceResult(record IssueOpsRecord, action, inventory, quiescence, tokenPath string) ExecutionReplaceResult {
	return ExecutionReplaceResult{
		OK: true, ID: record.ID, Action: action, Execution: *record.Execution,
		InventoryFingerprint: inventory, QuiescenceFingerprint: quiescence, ClaimTokenPath: tokenPath,
	}
}

func normalizeNativeActor(actor model.NativeActor) (model.NativeActor, error) {
	actor.Host = strings.ToLower(strings.TrimSpace(actor.Host))
	actor.SessionID = strings.TrimSpace(actor.SessionID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	if actor.SessionProcess != nil {
		receipt := *actor.SessionProcess
		receipt.StartedAt = strings.TrimSpace(receipt.StartedAt)
		receipt.Executable = strings.TrimSpace(receipt.Executable)
		actor.SessionProcess = &receipt
	}
	actor.ProcessAncestry = append([]model.NativeProcessReceipt(nil), actor.ProcessAncestry...)
	if err := model.ValidateNativeActor(actor); err != nil {
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
func refuseSelfRevoke(lifecycleID string, lease model.WriteLease, requester model.NativeActor) error {
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

func sameNativeActor(a, b *model.NativeActor) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return sameNativeActorIdentity(a, b) && sameNativeProcessReceipt(a.SessionProcess, b.SessionProcess)
}

func sameNativeActorIdentity(a, b *model.NativeActor) bool {
	return a != nil && b != nil && strings.EqualFold(a.Host, b.Host) && a.SessionID == b.SessionID && a.AgentID == b.AgentID
}

func sameNativeProcessReceipt(a, b *model.NativeProcessReceipt) bool {
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

func executionInventoryFingerprint(ctx context.Context, record IssueOpsRecord, requester model.NativeActor, deps ExecutionReplaceDependencies) (string, error) {
	snapshot, err := workspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", err
	}
	processStatus, orcaInventory, err := executionOwnerInventory(ctx, record, deps)
	if err != nil {
		return "", err
	}
	payload := struct {
		ID         string                           `json:"id"`
		Generation uint64                           `json:"generation"`
		Status     model.LeaseStatus                `json:"status"`
		Holder     *model.NativeActor               `json:"holder,omitempty"`
		Requester  model.NativeActor                `json:"requester"`
		Process    string                           `json:"process_status"`
		Orca       port.ExecutionOrcaOwnerInventory `json:"orca"`
		Snapshot   string                           `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, record.Execution.Lease.Status, record.Execution.Lease.Holder, requester, processStatus, orcaInventory, snapshot}
	return hashJSON(payload)
}

func executionQuiescenceFingerprint(ctx context.Context, record IssueOpsRecord, requester model.NativeActor, deps ExecutionReplaceDependencies) (string, error) {
	holder := record.Execution.Lease.Holder
	if holder == nil || holder.SessionProcess == nil {
		return "", fmt.Errorf("revoking lease is missing its old process receipt")
	}
	processStatus, _, err := inspectNativeProcessReceipt(*holder.SessionProcess)
	if err != nil {
		return "", err
	}
	if processStatus == "live" {
		return "", fmt.Errorf("old holder process is still live: pid=%d executable=%s", holder.SessionProcess.PID, holder.SessionProcess.Executable)
	}
	if processStatus != "dead" {
		return "", fmt.Errorf("old holder process identity is unsafe to finalize: pid=%d status=%s", holder.SessionProcess.PID, processStatus)
	}
	_, orcaInventory, err := executionOwnerInventory(ctx, record, deps)
	if err != nil {
		return "", err
	}
	if orcaInventory.TerminalLive || orcaInventory.TaskLive {
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
		for ancestor := range nativeProcessAncestryPIDs(pid) {
			excluded[ancestor] = true
		}
	}
	workspaceProcesses, err := inspectWorkspaceProcesses(record.Execution.Workspace.Root, excluded)
	if err != nil {
		return "", err
	}
	workspaceProcesses = dropRequesterOwnedProcesses(workspaceProcesses, requesterOwners)
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
		Holder     model.NativeActor                `json:"holder"`
		Requester  model.NativeActor                `json:"requester"`
		Process    model.NativeProcessReceipt       `json:"process"`
		Orca       port.ExecutionOrcaOwnerInventory `json:"orca"`
		Snapshot   string                           `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, *holder, requester, *holder.SessionProcess, orcaInventory, snapshot}
	return hashJSON(payload)
}

func executionOwnerInventory(ctx context.Context, record IssueOpsRecord, deps ExecutionReplaceDependencies) (string, port.ExecutionOrcaOwnerInventory, error) {
	status := "none"
	if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil {
		var err error
		status, _, err = inspectNativeProcessReceipt(*holder.SessionProcess)
		if err != nil {
			return "", port.ExecutionOrcaOwnerInventory{}, err
		}
	}
	if record.Execution.Mode != model.ExecutionModeOrca {
		return status, port.ExecutionOrcaOwnerInventory{}, nil
	}
	if record.Execution.Orca == nil || deps.OrcaOwner == nil {
		return "", port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca execution requires exact owner terminal and task inventory")
	}
	binding := record.Execution.Orca
	inventory, err := deps.OrcaOwner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, TaskID: binding.TaskID,
		DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
	})
	return status, inventory, err
}

func validateExecutionReplacementCWD(record IssueOpsRecord, cwd string) error {
	workspace := record.Execution.Workspace
	if !samePath(cwd, workspace.SourceRoot) && !samePath(cwd, workspace.Root) {
		return fmt.Errorf("execution replace cwd must be source_root or the canonical worktree")
	}
	return nil
}

func workspaceSnapshot(workspace model.Workspace) (string, error) {
	info, err := os.Lstat(workspace.Root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
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
	_, tracked, stderr := preflight.GitCmdRaw(workspace.Root, "diff", "--binary", "--no-ext-diff", "--")
	if stderr != "" {
		return "", fmt.Errorf("read tracked diff: %s", strings.TrimSpace(stderr))
	}
	_, staged, stderr := preflight.GitCmdRaw(workspace.Root, "diff", "--cached", "--binary", "--no-ext-diff", "--")
	if stderr != "" {
		return "", fmt.Errorf("read staged diff: %s", strings.TrimSpace(stderr))
	}
	code, untrackedRaw, stderr := preflight.GitCmdRaw(workspace.Root, "ls-files", "--others", "--exclude-standard", "-z")
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
	code, stdout, stderr := preflight.GitCmd(root, args...)
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

func claimTokenPath(record IssueOpsRecord) string {
	key := tokenSHA256(record.ID)[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "state", "issueops-v1", key, fmt.Sprintf("lease-%d.token", record.Execution.Lease.Generation))
}

func createClaimToken(record IssueOpsRecord) (string, string, error) {
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

func readClaimToken(record IssueOpsRecord, path string) (string, error) {
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

func removeClaimTokenIfPresent(record IssueOpsRecord) {
	if record.Execution != nil {
		_ = os.Remove(claimTokenPath(record))
	}
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
