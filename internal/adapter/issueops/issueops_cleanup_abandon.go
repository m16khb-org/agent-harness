package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"agent-harness/internal/adapter/issueops/pathutil"
	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/contract/issueops"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/port"
)

// cleanupAbandonReasonLimit는 --reason 상한이다. 감사 문자열의 표현력 한계가
// 아니라, lease 가드의 exact-command 파싱(commandparse)이 명령 전체를 토큰
// 단위로 재구성한다는 사실에서 오는 보수적 여유값이다.
const cleanupAbandonReasonLimit = 512

// cleanupAbandonReasonForbidden은 exact-command 파서가 "활성 셸 구성"으로
// 판정하는 문자 집합이다. 이 문자가 --reason에 들어가면 명령이 가드 단계에서
// 거부되므로, core가 먼저 거부해 진단을 앞당긴다(사유 없는 unsafe_mutation
// 거부보다 reason_required가 훨씬 읽기 쉽다).
const cleanupAbandonReasonForbidden = "\"'`$\\|&;<>()*?~"

// CleanupAbandonDeps는 게이트 평가의 외부 표면이다.
//
// Orca는 pending_intent_safe 게이트가 sealed marker로 orca 인벤토리를 실조회할
// 때 쓴다. 레코드의 Failure.Code("external_operation_ambiguous")는 5가지 상이한
// 애매성 경로에서 동일하게 기록되므로(execution_orca_intent.go:113-157) 레코드
// 만으로는 "orca에 아무것도 없음"을 증명할 수 없다. 어댑터 부재는 통과가
// 아니라 거부다(brooks 라운드 2 차단 1).
type CleanupAbandonDeps struct {
	Git  func(dir string, args ...string) (int, string)
	Orca port.ExecutionOrcaProvisioner
	// OrcaOwner는 게이트 ⑨가 orca 자원 잔여를 실조회할 때 쓴다. 게이트 ⑥은
	// 로컬 디렉터리만 보므로 orca 레지스트리에 남은 task를 놓친다(#136).
	//
	// Orca와 같은 이유로 부재는 통과가 아니라 거부다. 다만 조회 자체는 orca
	// 바인딩이 있는 레코드에서만 일어난다 — direct 사이클까지 어댑터를
	// 요구하면 아무 관련 없는 정리가 막힌다.
	OrcaOwner port.ExecutionOrcaOwnerInspector
}

// cleanupAbandonInventory는 fingerprint 입력이 되는 현재 관측 상태다.
// cleanupFinishInventory와 달리 원격 completion 권한을 다루지 않고, 레코드가
// 지목한 로컬 worktree와 branch의 삭제 권한만 봉인한다.
type cleanupAbandonInventory struct {
	ID                 string `json:"id"`
	Repo               string `json:"repo"`
	Branch             string `json:"branch"`
	WorktreeRoot       string `json:"worktree_root"`
	WorktreePresent    bool   `json:"worktree_present"`
	WorktreeCanonical  bool   `json:"worktree_canonical"`
	WorktreeBranch     string `json:"worktree_branch"`
	WorktreeClean      bool   `json:"worktree_clean"`
	WorktreeHead       string `json:"worktree_head"`
	BranchOID          string `json:"branch_oid"`
	BranchCheckoutPath string `json:"branch_checkout_path"`
	RecordSHA          string `json:"record_sha"`
	Phase              string `json:"phase"`
	LeaseStatus        string `json:"lease_status"`
	PendingOperationID string `json:"pending_operation_id"`
}

// CleanupAbandon은 게이트를 평가하고, apply에서 원격을 건드리지 않은 채 로컬
// worktree와 branch를 먼저 제거한 뒤 레코드와 intent 행을 원자 삭제한다.
func CleanupAbandon(ctx context.Context, stateRoot string, req CleanupAbandonRequest, deps CleanupAbandonDeps) (CleanupAbandonResult, error) {
	if deps.Git == nil {
		deps.Git = func(dir string, args ...string) (int, string) {
			code, stdout, stderr := GitCmd(dir, args...)
			if code != 0 && stderr != "" {
				return code, stderr
			}
			return code, stdout
		}
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return CleanupAbandonResult{OK: false, ID: req.ID}, err
	}
	result := CleanupAbandonResult{
		OK: true, ID: record.ID, Preview: !req.Apply, Reason: strings.TrimSpace(req.Reason),
		RemoteBranchDeletion: "not_planned",
	}
	inventory, missing := cleanupAbandonGates(ctx, stateRoot, record, req, deps, &result)
	result.Missing = missing
	if len(missing) > 0 {
		result.OK = false
		return result, fmt.Errorf("cleanup abandon is not ready: %s", strings.Join(missing, ", "))
	}
	fingerprint, err := cleanupAbandonFingerprint(inventory)
	if err != nil {
		return CleanupAbandonResult{OK: false, ID: record.ID}, err
	}
	result.Fingerprint = fingerprint
	result.RemovalPlan = cleanupAbandonRemovalPlan(record, inventory)
	snapshot := record
	result.Record = &snapshot
	if !req.Apply {
		result.NextCommand = fmt.Sprintf("agent-harness issueops cleanup abandon --id %s --reason %q --apply --confirm --fingerprint %s --json",
			record.ID, result.Reason, fingerprint)
		return result, nil
	}
	if !req.Confirm {
		result.OK = false
		return result, fmt.Errorf("cleanup abandon --apply requires --confirm")
	}
	// TOCTOU: apply 직전 재계산 일치. 게이트가 통과했더라도 preview 이후 phase·
	// lease·pending이 바뀌었다면 그 preview는 다른 상태를 승인한 것이다.
	if req.Fingerprint != fingerprint {
		result.OK = false
		return result, fmt.Errorf("stale cleanup fingerprint; run --preview again and retry with the new value")
	}
	record, err = armCleanupAbandon(ctx, stateRoot, record, fingerprint, inventory)
	if err != nil {
		result.OK = false
		return result, err
	}
	if inventory.WorktreePresent {
		if code, out := deps.Git(record.Repo, "worktree", "remove", inventory.WorktreeRoot); code != 0 {
			if _, statErr := os.Lstat(inventory.WorktreeRoot); !os.IsNotExist(statErr) {
				result.OK = false
				result.FailedStep = "worktree_remove"
				receiptErr := recordCleanupAbandonFailure(stateRoot, record.ID, result.FailedStep, fmt.Errorf("%s", out), fingerprint, inventory)
				result.NextCommand = cleanupAbandonPreviewCommand(record.ID, result.Reason)
				return result, cleanupAbandonApplyError(fmt.Sprintf("cleanup abandon worktree removal failed (record preserved): %s", out), receiptErr)
			}
		}
		result.WorktreeRemoved = true
	}
	if inventory.BranchOID != "" {
		// finish와 같은 순서 결함이다: 앞선 worktree 제거가 linked branch ref를
		// 함께 회수하면 이 시점의 대상은 이미 없다. 부재는 삭제의 목표 상태이므로
		// 재관측으로 확인한 뒤 idempotent success로 정규화한다(#291).
		if code, out := deps.Git(record.Repo, "update-ref", "-d", "refs/heads/"+inventory.Branch, inventory.BranchOID); code != 0 &&
			branchRefPresent(deps.Git, record.Repo, inventory.Branch) {
			result.OK = false
			result.FailedStep = "branch_delete"
			receiptErr := recordCleanupAbandonFailure(stateRoot, record.ID, result.FailedStep, fmt.Errorf("%s", out), fingerprint, inventory)
			result.NextCommand = cleanupAbandonPreviewCommand(record.ID, result.Reason)
			return result, cleanupAbandonApplyError(fmt.Sprintf("cleanup abandon branch deletion failed (record preserved): %s", out), receiptErr)
		}
		result.BranchDeleted = true
	}
	deleted, err := deleteAbandonedIssueOps(ctx, stateRoot, record, cleanupAbandonIntentOperationIDs(record))
	if err != nil {
		result.OK = false
		result.FailedStep = "record_delete"
		receiptErr := recordCleanupAbandonFailure(stateRoot, record.ID, result.FailedStep, err, fingerprint, inventory)
		result.NextCommand = cleanupAbandonPreviewCommand(record.ID, result.Reason)
		return result, cleanupAbandonApplyError(fmt.Sprintf("cleanup abandon deletion failed (record preserved): %v", err), receiptErr)
	}
	result.IntentRowsDeleted = deleted
	result.RecordDeleted = true
	result.AbandonedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

func cleanupAbandonPreviewCommand(id, reason string) string {
	return fmt.Sprintf("agent-harness issueops cleanup abandon --id %s --reason %q --preview --json", id, reason)
}

func recordCleanupAbandonFailure(stateRoot, id, step string, stepErr error, fingerprint string, inventory cleanupAbandonInventory) error {
	return withCleanupAbandonLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.CleanupAbandonFailure == nil || record.CleanupAbandonFailure.Fingerprint != fingerprint {
			return fmt.Errorf("cleanup abandon attempt changed before failure receipt")
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		failure := &issueops.IssueOpsCleanupAbandonFailure{
			Step: step, Message: stepErr.Error(), Fingerprint: fingerprint,
			RecordSHA:    inventory.RecordSHA,
			WorktreePath: inventory.WorktreeRoot, Branch: inventory.Branch,
			WorktreeHead: inventory.WorktreeHead, BranchOID: inventory.BranchOID, At: now,
		}
		failure.InventorySHA256 = cleanupAbandonFailureSeal(record, failure)
		record.CleanupAbandonFailure = failure
		record.UpdatedAt = now
		_, err = writeIssueOps(stateRoot, record)
		return err
	})
}

func cleanupAbandonApplyError(message string, receiptErr error) error {
	if receiptErr == nil {
		return fmt.Errorf("%s", message)
	}
	return fmt.Errorf("%s; failure receipt update failed: %v", message, receiptErr)
}

// cleanupAbandonGates는 모든 게이트를 평가하고 missing을 나열한다(첫 실패에
// 멈추지 않는다 — 운영자가 한 번의 preview로 모든 결격 사유를 본다).
func cleanupAbandonGates(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, req CleanupAbandonRequest, deps CleanupAbandonDeps, result *CleanupAbandonResult) (cleanupAbandonInventory, []string) {
	missing := []string{}
	// ① 사유 필수. 삭제 근거 없는 수명 종료는 감사 불가능하다.
	if err := validateCleanupAbandonReason(req.Reason); err != nil {
		missing = append(missing, "reason_required")
		result.ReasonError = err.Error()
	}
	// ② 보존해야 할 것은 phase 이름이 아니라 머지 증적이다.
	//
	// 원래 이 게이트는 done을 통째로 거부했다. abandon이 done을 삼키면 머지 증적
	// 보존 경로(reflect→finish)를 우회하는 탈출구가 된다는 우려였고, 그 우려의
	// 실체는 게이트 ④가 이미 막는다. done 자체를 거부하면 finish가 성립하지 않는
	// 결말 — artifact가 없거나 병합되지 않은 채 완료된 사이클 — 이 어느 경로로도
	// 은퇴하지 못한다(#342에서 실측: 6건이 이 사유로 차단).
	//
	// finish는 remote_artifact_merged를 요구하므로 미머지 레코드에 쓸 수 없고,
	// 그 요구를 완화하는 것은 잘못된 정리를 여는 일이다. 따라서 판정을 ④로 옮긴다.
	// ③ lease allowlist: 홀더가 없는 상태만 통과한다.
	//
	// 판정 기준은 상태 이름이 아니라 writer의 유무다. validateWriteLease가
	// claimable에 홀더 부재를 강제하므로(model/execution.go) claimable과
	// released는 같은 성질이고, 거부해야 할 것은 살아 있는 writer를 가진
	// active와 fenced holder를 여전히 보유한 revoking이다(brooks F5).
	//
	// claimable을 함께 거부하던 것은 안전한 기본값이었지만 정리 경로를 막았다.
	// 운영자는 claim→release로 lease를 한 바퀴 돌려야 했고 그 두 단계는
	// 아무것도 정리하지 않았다(#140, #139에서 실측). claimable을 허용해도
	// pending intent(⑤), 워크트리·브랜치 잔여(⑥·⑦), orca 자원 잔여(⑨)가
	// 각각 막으므로 실제로 열리는 것은 홀더도 자원도 없는 레코드뿐이다.
	if record.Execution != nil && cleanupAbandonLeaseHoldsWriter(record.Execution.Lease.Status) {
		missing = append(missing, "lease_terminal")
	}
	// ④ 머지 증적을 가진 레코드의 정답은 reflect→finish다. artifact가 있는데
	// 미병합을 관측하지 못했다면 — 조회 실패든 미관측이든 — 닫힌 채로 둔다.
	if record.RemoteArtifact != nil && !req.ArtifactUnmerged {
		missing = append(missing, "remote_artifact_unmerged")
	}
	// ⑧ 자식 고아 방지(finish의 child_tasks_closed 대응물, brooks F6).
	if cleanupAbandonHasChildren(record) {
		missing = append(missing, "no_children")
	}

	inventory := cleanupAbandonInventory{
		ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(record.Branch),
		Phase: string(record.Phase), LeaseStatus: "none",
	}
	inventory.RecordSHA = cleanupAbandonRecordSHA(record)
	if record.Execution != nil {
		inventory.LeaseStatus = string(record.Execution.Lease.Status)
		inventory.WorktreeRoot = strings.TrimSpace(record.Execution.Workspace.Root)
		if branch := strings.TrimSpace(record.Execution.Workspace.Branch); branch != "" {
			inventory.Branch = branch
		}
		if record.Execution.Pending != nil {
			inventory.PendingOperationID = strings.TrimSpace(record.Execution.Pending.OperationID)
		}
	}
	// 레거시/직접 사이클은 record.WorktreePath만 가질 수 있다. 폴백하되, 두 값이
	// 모두 있고 다르면 어느 쪽도 신뢰하지 않는다(C2-F7 준용).
	if linked := strings.TrimSpace(record.WorktreePath); linked != "" {
		if inventory.WorktreeRoot == "" {
			inventory.WorktreeRoot = linked
		} else if pathutil.CleanAbsPath(inventory.WorktreeRoot) != pathutil.CleanAbsPath(linked) {
			missing = append(missing, "worktree_identity_conflict")
		}
	}
	if inventory.WorktreeRoot != "" {
		if info, err := os.Lstat(inventory.WorktreeRoot); err == nil && info.IsDir() {
			inventory.WorktreePresent = true
		}
	}
	result.WorktreePath = inventory.WorktreeRoot
	result.Branch = inventory.Branch
	result.WorktreePresent = inventory.WorktreePresent
	result.PendingOperationID = inventory.PendingOperationID
	if inventory.WorktreePresent {
		inventory, missing = cleanupAbandonInspectWorktree(inventory, deps, missing)
	}
	result.WorktreeCanonical = inventory.WorktreeCanonical
	result.WorktreeClean = inventory.WorktreeClean
	result.WorktreeHead = inventory.WorktreeHead
	if inventory.Branch != "" {
		if code, out := deps.Git(record.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+inventory.Branch); code == 0 {
			inventory.BranchOID = strings.TrimSpace(out)
			result.BranchPresent = true
		}
	}
	result.BranchOID = inventory.BranchOID
	if result.BranchPresent {
		if code, out := deps.Git(record.Repo, "worktree", "list", "--porcelain"); code != 0 {
			missing = append(missing, "worktree_registry_observable")
		} else {
			inventory.BranchCheckoutPath = cleanupAbandonBranchCheckoutPath(out, inventory.Branch)
			result.BranchCheckoutPath = inventory.BranchCheckoutPath
			if inventory.BranchCheckoutPath != "" && !samePath(inventory.BranchCheckoutPath, inventory.WorktreeRoot) {
				missing = append(missing, "branch_checked_out_elsewhere")
			}
		}
	}
	receiptMatches := cleanupAbandonFailureInventoryMatches(record, inventory, result.BranchPresent)
	if record.CleanupAbandonFailure != nil && !receiptMatches {
		missing = append(missing, "cleanup_failure_inventory")
	}
	partialBranchRetry := receiptMatches && cleanupAbandonPartialBranchRetry(record, inventory, result.BranchPresent)
	if partialBranchRetry {
		inventory.WorktreeHead = record.CleanupAbandonFailure.WorktreeHead
		result.WorktreeHead = inventory.WorktreeHead
	}
	if (inventory.WorktreePresent || result.BranchPresent) && record.Execution == nil {
		missing = append(missing, "local_residue_execution")
	}
	// 비대칭 잔여물(한쪽만 남음)은 거부하지 않는다(#433).
	//
	// 예전에는 이 비대칭 자체가 거부 사유였고, abandon 자신이 남긴 retry
	// receipt가 있을 때만 예외가 열렸다. 그래서 worktree가 **다른 경로로**
	// 사라진 lifecycle에는 typed 출구가 없었다 — cleanup finish는 머지 증거를,
	// cleanup orphan은 worktree를 요구하므로 둘 다 해당되지 않았다.
	// 실측: io-268bd6ac6e7a가 다른 모든 게이트를 통과하고도 이 하나로 영구히
	// 막혔다.
	//
	// 완화해도 안전한 근거는 각 축의 검사가 따로 서 있기 때문이다. worktree는
	// canonical·clean·head로, branch는 다른 곳에 체크아웃되지 않았는지로 각각
	// 검증되고, fingerprint가 두 축의 관측(존재 여부와 값)을 모두 결속하므로
	// apply 직전에 없던 쪽이 생기면 stale로 멈춘다.
	//
	// 쌍이 온전할 때의 head 일치 요구는 아래에서 그대로 유지한다 — 그것은
	// 비대칭 문제가 아니라 두 관측이 서로 모순되는 경우다.
	if inventory.WorktreePresent && result.BranchPresent && inventory.WorktreeHead != inventory.BranchOID {
		missing = append(missing, "local_branch_head")
	}
	// ⑤ pending external intent.
	if record.Execution != nil && record.Execution.Pending != nil {
		if err := cleanupAbandonPendingSafe(ctx, stateRoot, record, inventory, deps); err != nil {
			missing = append(missing, "pending_intent_safe")
			result.PendingIntentError = cleanupAbandonPendingRecovery(record.ID, err)
		}
	}
	// ⑨ orca 자원 잔여. 게이트 ⑥은 로컬 디렉터리만 보므로 orca 레지스트리에
	// 남은 task를 놓친다.
	if err := cleanupAbandonOrcaResourcesAbsent(ctx, record, deps); err != nil {
		missing = append(missing, "orca_resources_absent")
		result.OrcaResidueError = err.Error()
	}
	return inventory, missing
}

func cleanupAbandonPartialBranchRetry(record issueops.IssueOpsRecord, inventory cleanupAbandonInventory, branchPresent bool) bool {
	failure := record.CleanupAbandonFailure
	return !inventory.WorktreePresent && branchPresent && failure != nil &&
		(failure.Step == "applying" || failure.Step == "branch_delete") &&
		samePath(failure.WorktreePath, inventory.WorktreeRoot) &&
		failure.Branch == inventory.Branch && failure.BranchOID == inventory.BranchOID &&
		failure.WorktreeHead == inventory.BranchOID
}

func cleanupAbandonFailureInventoryMatches(record issueops.IssueOpsRecord, inventory cleanupAbandonInventory, branchPresent bool) bool {
	failure := record.CleanupAbandonFailure
	if failure == nil {
		return true
	}
	if !cleanupAbandonValidFingerprint(failure.Fingerprint) || !cleanupAbandonValidFingerprint(failure.RecordSHA) ||
		!cleanupAbandonValidFingerprint(failure.InventorySHA256) ||
		failure.InventorySHA256 != cleanupAbandonFailureSeal(record, failure) || failure.Branch != inventory.Branch ||
		!samePath(failure.WorktreePath, inventory.WorktreeRoot) {
		return false
	}
	originalAbsent := failure.WorktreeHead == "" && failure.BranchOID == ""
	originalPaired := failure.WorktreePath != "" && failure.WorktreeHead != "" &&
		failure.WorktreeHead == failure.BranchOID
	if !originalAbsent && !originalPaired {
		return false
	}
	switch failure.Step {
	case "applying":
		switch {
		case inventory.WorktreePresent && branchPresent:
			return originalPaired && inventory.WorktreeHead == failure.WorktreeHead && inventory.BranchOID == failure.BranchOID
		case !inventory.WorktreePresent && branchPresent:
			return originalPaired && inventory.BranchOID == failure.BranchOID
		case !inventory.WorktreePresent && !branchPresent:
			return originalAbsent || originalPaired
		}
	case "worktree_remove":
		return inventory.WorktreePresent && branchPresent && originalPaired &&
			inventory.WorktreeHead == failure.WorktreeHead && inventory.BranchOID == failure.BranchOID
	case "branch_delete":
		return !inventory.WorktreePresent && branchPresent && originalPaired && inventory.BranchOID == failure.BranchOID
	case "record_delete":
		return !inventory.WorktreePresent && !branchPresent && (originalAbsent || originalPaired)
	}
	return false
}

func cleanupAbandonValidFingerprint(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func cleanupAbandonFailureSeal(record issueops.IssueOpsRecord, failure *issueops.IssueOpsCleanupAbandonFailure) string {
	sealed := struct {
		ID           string `json:"id"`
		Repo         string `json:"repo"`
		Fingerprint  string `json:"fingerprint"`
		RecordSHA    string `json:"record_sha"`
		WorktreePath string `json:"worktree_path"`
		Branch       string `json:"branch"`
		WorktreeHead string `json:"worktree_head"`
		BranchOID    string `json:"branch_oid"`
	}{
		ID: record.ID, Repo: record.Repo, Fingerprint: failure.Fingerprint,
		RecordSHA: failure.RecordSHA, WorktreePath: failure.WorktreePath,
		Branch: failure.Branch, WorktreeHead: failure.WorktreeHead, BranchOID: failure.BranchOID,
	}
	data, _ := json.Marshal(sealed)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cleanupAbandonBranchCheckoutPath(output, branch string) string {
	currentPath := ""
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.TrimSpace(line) == "branch refs/heads/"+branch:
			return currentPath
		}
	}
	return ""
}

func cleanupAbandonInspectWorktree(inventory cleanupAbandonInventory, deps CleanupAbandonDeps, missing []string) (cleanupAbandonInventory, []string) {
	if code, out := deps.Git(inventory.WorktreeRoot, "rev-parse", "--show-toplevel"); code == 0 &&
		samePath(out, inventory.WorktreeRoot) {
		inventory.WorktreeCanonical = true
	} else {
		missing = append(missing, "worktree_canonical")
	}
	if code, out := deps.Git(inventory.WorktreeRoot, "symbolic-ref", "--quiet", "--short", "HEAD"); code == 0 {
		inventory.WorktreeBranch = strings.TrimSpace(out)
	}
	if inventory.WorktreeBranch != inventory.Branch {
		missing = append(missing, "worktree_branch_match")
	}
	if code, out := deps.Git(inventory.WorktreeRoot, "rev-parse", "HEAD"); code == 0 {
		inventory.WorktreeHead = strings.TrimSpace(out)
	} else {
		missing = append(missing, "worktree_head")
	}
	if code, out := deps.Git(inventory.WorktreeRoot, "status", "--porcelain=v1"); code == 0 && strings.TrimSpace(out) == "" {
		inventory.WorktreeClean = true
	} else {
		missing = append(missing, "worktree_clean")
	}
	return inventory, missing
}

func cleanupAbandonRemovalPlan(record issueops.IssueOpsRecord, inventory cleanupAbandonInventory) []string {
	plan := []string{}
	if inventory.WorktreePresent {
		plan = append(plan, "local_worktree:"+inventory.WorktreeRoot)
	}
	if inventory.BranchOID != "" {
		plan = append(plan, "local_branch:"+inventory.Branch+"@"+inventory.BranchOID)
	}
	plan = append(plan, "record:"+record.ID)
	for _, operationID := range cleanupAbandonIntentOperationIDs(record) {
		plan = append(plan, "intent:"+operationID)
	}
	return plan
}

// cleanupAbandonPendingRecovery는 pending intent가 안전하다고 증명되지 않았을 때
// reconcile부터 Orca worktree 회수까지 남은 운영 경로를 항상 함께 제시한다.
func cleanupAbandonPendingRecovery(id string, cause error) string {
	detail := cause.Error()
	if strings.Contains(detail, "execution reconcile") && strings.Contains(detail, "worktree") {
		return detail
	}
	return fmt.Sprintf("%s; run `agent-harness issueops execution reconcile --id %s --preview --json` until it settles, then reclaim the Orca worktree with `orca worktree remove` before retrying abandon",
		detail, id)
}

// cleanupAbandonLeaseHoldsWriter는 lease가 아직 writer를 붙들고 있는지 본다.
// 알 수 없는 상태는 writer 보유로 다룬다 — 모르는 상태를 통과시키면 게이트가
// fail-open이 된다.
func cleanupAbandonLeaseHoldsWriter(status issueops.LeaseStatus) bool {
	switch status {
	case issueops.LeaseStatusClaimable, issueops.LeaseStatusReleased:
		return false
	default:
		return true
	}
}

// cleanupAbandonOrcaResourcesAbsent는 레코드를 지워도 orca 자원이 소유자를 잃지
// 않는지 판정한다.
//
// abandon은 Orca 자원을 지우지 않는다. orca task가 살아 있는데 레코드를 지우면
// 그 task는 소유자 조회가 영구히 0건이 되어 operational_task_residue로 계속
// 보고된다. 그래서 Orca 잔여물을 지울 수 있는 경로로 보낸다.
//
// 레코드에 바인딩이 있다는 것만으로 막지 않고 실조회한다. 그렇게 하면 orca에서
// 이미 정리된 사이클까지 차단해 중도 포기 경로가 사라진다.
//
// 조회할 수 없으면 통과가 아니라 거부다(#106 pending_intent_safe와 같은 계약).
func cleanupAbandonOrcaResourcesAbsent(ctx context.Context, record issueops.IssueOpsRecord, deps CleanupAbandonDeps) error {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return nil
	}
	binding := *record.Execution.Orca
	if strings.TrimSpace(binding.TaskID) == "" {
		return nil
	}
	if deps.OrcaOwner == nil {
		return fmt.Errorf("Orca owner inspector is not configured; resolve this cycle with `agent-harness issueops cleanup finish` or `agent-harness issueops cleanup orphan`")
	}
	// Orca 런타임이 롤오버되면 봉인된 runtime ID로는 아무것도 조회할 수 없고,
	// 어댑터는 bounded 권한 없이 바뀐 runtime의 인벤토리를 돌려주지 않는다. 그
	// 조회 실패를 ambiguous로 취급하면 롤오버를 겪은 레코드는 finish(머지 증적
	// 필요)로도 abandon으로도 은퇴하지 못한다(#342에서 실측: 4건이 이 사유로 차단).
	//
	// 권한은 holderless에서만 연다. 살아 있는 writer가 있으면 이전 런타임의 자원
	// 부재를 증명할 수 없기 때문이다. 게이트 ③이 같은 판정으로 active/revoking을
	// 이미 거부하므로 두 게이트는 같은 방향으로 닫힌다.
	allowRuntimeRollover := !cleanupAbandonLeaseHoldsWriter(record.Execution.Lease.Status) &&
		record.Execution.Lease.Holder == nil
	inventory, err := deps.OrcaOwner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, RunID: binding.RunID, TaskID: binding.TaskID,
		DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
		AllowRuntimeRollover: allowRuntimeRollover,
	})
	if err != nil {
		return fmt.Errorf("Orca owner inventory is ambiguous; resolve this cycle with `agent-harness issueops cleanup finish` or `agent-harness issueops cleanup orphan`: %w", err)
	}
	if inventory.TaskLive || inventory.TerminalLive {
		return fmt.Errorf("Orca resources are still live (task_status=%q dispatch_status=%q terminal_live=%t); abandon leaves them without an owner, so use `agent-harness issueops cleanup finish` or `agent-harness issueops cleanup orphan`",
			inventory.TaskStatus, inventory.DispatchStatus, inventory.TerminalLive)
	}
	return nil
}

// cleanupAbandonPendingSafe는 pending external intent를 가진 레코드를 삭제해도
// 안전한지 판정한다. 기본값은 거부이고, 아래 조건 전부를 통과할 때만 허용한다.
//
// owner 단계는 현재 단계만 비어 있다고 충분하지 않다. 예를 들어 dispatch가
// 없더라도 terminal/task가 남을 수 있다. 따라서 worktree부터 현재 단계까지
// 봉인된 인벤토리를 모두 authoritative zero로 확인하고, 별도 게이트 ⑨에서
// 이전 generation의 owner binding도 확인한다.
func cleanupAbandonPendingSafe(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, inventory cleanupAbandonInventory, deps CleanupAbandonDeps) error {
	pending := record.Execution.Pending
	// (a) kind allowlist — 로컬 orca mutation 한정. remote PR/MR 계열 kind는
	// 원격 고아 PR을 남길 수 있으므로 무조건 거부하고 reconcile로 보낸다.
	if !pendingKindForOrcaStageFromKind(pending.Kind) {
		// reconcile을 지시하는 것만으로는 부족했다. 실측에서 운영자는 reconcile을
		// 완주한 뒤 무엇을 해야 하는지 알 수 없었다(#139) — 남은 절차를 함께
		// 알려야 게이트 응답이 탈출 경로가 된다(#140).
		return fmt.Errorf("pending external intent kind %q is not local-only; run `agent-harness issueops execution reconcile --id %s --preview --json` until it settles, then reclaim the Orca worktree with `orca worktree remove` before retrying abandon",
			pending.Kind, record.ID)
	}
	if record.Execution.Mode != issueops.ExecutionModeOrca {
		return fmt.Errorf("Orca intent requires an Orca execution record")
	}
	// (b) 기록된 canonical workspace가 디스크에 없어야 한다.
	if inventory.WorktreePresent {
		return fmt.Errorf("recorded workspace root still exists on disk")
	}
	// (c) sealed marker 실조회. 레코드는 "없음"을 증명하지 못한다.
	payload, err := readExternalOrcaIntentPayload(stateRoot, pending.OperationID)
	if err != nil {
		return err
	}
	if payload.LifecycleID != record.ID || payload.Marker != pending.Marker ||
		payload.Generation != record.Execution.Lease.Generation ||
		pending.Kind != pendingKindForOrcaStage(payload.Stage) {
		return fmt.Errorf("Orca external intent row does not belong to this lifecycle")
	}
	if deps.Orca == nil {
		return fmt.Errorf("Orca intent inspector is not configured")
	}
	requests, err := cleanupAbandonIntentInspectionRequests(record, payload)
	if err != nil {
		return err
	}
	for _, request := range requests {
		inspected, inspectErr := deps.Orca.InspectIntent(ctx, request)
		if inspectErr != nil {
			return fmt.Errorf("Orca %s intent inventory is ambiguous; intent retained: %w", request.Stage, inspectErr)
		}
		if len(inspected.Candidates) != 0 {
			return fmt.Errorf("Orca %s intent inventory found %d candidate(s); the mutation may have landed",
				request.Stage, len(inspected.Candidates))
		}
		if !inspected.AuthoritativeZero {
			return fmt.Errorf("Orca %s intent inventory returned a non-authoritative zero", request.Stage)
		}
	}
	return nil
}

func cleanupAbandonIntentInspectionRequests(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) ([]port.ExecutionOrcaIntentRequest, error) {
	current, err := executionOrcaIntentInspectionRequest(record, payload)
	if err != nil {
		return nil, err
	}
	requests := make([]port.ExecutionOrcaIntentRequest, 0, 4)
	worktree := current
	worktree.Stage = port.ExecutionOrcaIntentWorktree
	worktree.Prepared = nil
	worktree.Launch = nil
	worktree.TerminalPTYID = ""
	worktree.RunID = ""
	worktree.RunBound = false
	worktree.TaskID = ""
	requests = append(requests, worktree)
	if payload.Stage == preparationcontract.IntentStageWorktree {
		return requests, nil
	}

	terminal := current
	terminal.Stage = port.ExecutionOrcaIntentTerminal
	terminal.TerminalPTYID = ""
	terminal.RunID = ""
	terminal.RunBound = false
	terminal.TaskID = ""
	requests = append(requests, terminal)
	if payload.Stage == preparationcontract.IntentStageTerminal ||
		payload.Stage == preparationcontract.IntentStageRun ||
		payload.Stage == preparationcontract.IntentStageRunBind {
		return requests, nil
	}

	// Run은 삭제 수단이 없는 경량 namespace라 cleanup residue가 아니다.
	// 생성·바인딩 후보는 무시하고, 실제 실행 자원인 task부터 다시 검사한다.
	task := current
	task.Stage = port.ExecutionOrcaIntentTask
	task.RunBound = true
	task.TaskID = ""
	requests = append(requests, task)
	if payload.Stage == preparationcontract.IntentStageTask {
		return requests, nil
	}
	if payload.Stage != preparationcontract.IntentStageDispatch {
		return nil, fmt.Errorf("unsupported Orca cleanup intent stage %q", payload.Stage)
	}
	return append(requests, current), nil
}

// cleanupAbandonIntentOperationIDs는 삭제 대상 external intent 행 키를
// {Pending.OperationID} ∪ {Failure.OperationID}로 모은다(공백·중복 제거).
// Failure는 Pending과 같은 operation을 가리키는 것이 보통이지만, 실패 접수 후
// pending이 정리된 레코드에서는 Failure만 행을 참조할 수 있다.
func cleanupAbandonIntentOperationIDs(record issueops.IssueOpsRecord) []string {
	if record.Execution == nil {
		return nil
	}
	candidates := []string{}
	if record.Execution.Pending != nil {
		candidates = append(candidates, record.Execution.Pending.OperationID)
	}
	if record.Execution.Failure != nil {
		candidates = append(candidates, record.Execution.Failure.OperationID)
	}
	out := []string{}
	seen := map[string]bool{}
	for _, id := range candidates {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// deleteAbandonedIssueOps는 abandon 전용 원자 삭제다. deleteIssueOps는
// finish/prune의 계약이므로 건드리지 않고, external intent 행 삭제는 여기서만
// 같은 sqlstore.Apply 배치에 넣는다 — 레코드만 지우고 intent 행이 남으면 그
// 행은 어떤 lifecycle도 소유하지 않는 영구 고아가 된다(brooks 라운드 2 차단 2).
//
// 소유자 가드는 lease 인덱스 규율(execution_state.go:150-159)을 준용한다:
// 행이 없으면 성공(멱등 — normalizeOrcaRemoveWorktreeErr 계약 동형), 있는데
// 소유자가 다르거나 소유자를 읽을 수 없으면 하드 에러.
func deleteAbandonedIssueOps(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, operationIDs []string) ([]string, error) {
	id, err := normalizeIssueOpsID(record.ID)
	if err != nil {
		return nil, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	var deleted []string
	err = withCleanupAbandonLock(ctx, stateRoot, id, func(context.Context) error {
		// 임계구역 재검사: fingerprint는 lock 밖에서 계산됐다. 권위 필드가
		// 그 사이 바뀌었다면 이 apply는 다른 상태를 지우는 것이 된다.
		current, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if cleanupAbandonRecordSHA(current) != cleanupAbandonRecordSHA(record) {
			return fmt.Errorf("abandon authority changed before deletion CAS")
		}
		rows := []string{}
		mutations := []port.RecordMutation{}
		for _, operationID := range operationIDs {
			data, ok, err := db.Get(externalIntentBucket, operationID)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			var owner struct {
				LifecycleID string `json:"lifecycle_id"`
			}
			if err := json.Unmarshal(data, &owner); err != nil {
				return fmt.Errorf("decode external intent payload %s: %w", operationID, err)
			}
			if owner.LifecycleID != id {
				return fmt.Errorf("refusing to delete external intent row %s owned by another lifecycle", operationID)
			}
			mutations = append(mutations, port.RecordMutation{Bucket: externalIntentBucket, ID: operationID, Delete: true})
			rows = append(rows, operationID)
		}
		// 스테이징 artifact는 레코드와 수명을 같이한다(C4a-F1 ②).
		mutations = append(mutations,
			port.RecordMutation{Bucket: artifactStageBucket, ID: id, Delete: true},
			port.RecordMutation{Bucket: issueOpsBucket, ID: id, Delete: true},
		)
		if err := db.Apply(ctx, mutations); err != nil {
			return err
		}
		deleted = rows
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func cleanupAbandonHasChildren(record issueops.IssueOpsRecord) bool {
	if len(record.ChildCycles) > 0 {
		return true
	}
	issueURL := strings.TrimSpace(record.IssueURL)
	for _, link := range record.IssueLinks {
		if link.Type == "child" && strings.TrimSpace(link.CloseVerifiedAt) == "" &&
			(issueURL == "" || strings.TrimSpace(link.URL) != issueURL) {
			return true
		}
	}
	return false
}

func validateCleanupAbandonReason(reason string) error {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return fmt.Errorf("--reason is required")
	}
	if len(trimmed) > cleanupAbandonReasonLimit {
		return fmt.Errorf("--reason must not exceed %d bytes", cleanupAbandonReasonLimit)
	}
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("--reason must not contain control characters")
		}
		if strings.ContainsRune(cleanupAbandonReasonForbidden, r) {
			return fmt.Errorf("--reason must not contain %q; the lease guard parses this command exactly and rejects active shell characters", r)
		}
	}
	return nil
}

func armCleanupAbandon(ctx context.Context, stateRoot string, expected issueops.IssueOpsRecord, fingerprint string, inventory cleanupAbandonInventory) (issueops.IssueOpsRecord, error) {
	var armed issueops.IssueOpsRecord
	err := withCleanupAbandonLock(ctx, stateRoot, expected.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if cleanupAbandonRecordSHA(current) != cleanupAbandonRecordSHA(expected) {
			return fmt.Errorf("abandon authority changed before local cleanup CAS")
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		failure := &issueops.IssueOpsCleanupAbandonFailure{
			Step: "applying", Fingerprint: fingerprint,
			RecordSHA:    inventory.RecordSHA,
			WorktreePath: inventory.WorktreeRoot, Branch: inventory.Branch,
			WorktreeHead: inventory.WorktreeHead, BranchOID: inventory.BranchOID, At: now,
		}
		failure.InventorySHA256 = cleanupAbandonFailureSeal(current, failure)
		current.CleanupAbandonFailure = failure
		if current.Execution != nil && current.Execution.Lease.Status == issueops.LeaseStatusClaimable {
			current.Execution.Lease.Status = issueops.LeaseStatusReleased
			current.Execution.Lease.ClaimTokenSHA256 = ""
			current.Execution.Lease.ReleasedAt = now
		}
		current.UpdatedAt = now
		armed, err = writeIssueOps(stateRoot, current)
		return err
	})
	return armed, err
}

func cleanupAbandonRecordSHA(record issueops.IssueOpsRecord) string {
	data, err := json.Marshal(record)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cleanupAbandonFingerprint(inventory cleanupAbandonInventory) (string, error) {
	data, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
