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

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/core/sqlstore"
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

// CleanupAbandonRequest는 폐기된 비-done 사이클의 로컬 레코드 수명 종료
// (issue #106)의 입력이다.
//
// 이 경로는 cleanup finish와 두 가지 축에서 다르다.
//   - 원격 무접촉: 이슈 본문·PR/MR·원격 브랜치 어느 것도 읽지도 쓰지도 않는다.
//     ReflectCleanupAudit를 재사용하지 않는 이유는 빈 completion payload가 열린
//     이슈에 가짜 "완료 기록" 섹션을 append하고, 그 마커만 보는
//     `completion_reflected` 게이트가 미래 사이클의 파괴적 finish를 영구
//     개방하기 때문이다(brooks F3).
//   - 보존 불변식 C2-F6("레코드 삭제 전 보존")의 의도적 예외: 원격에 남길
//     자리가 없으므로 삭제 대상 레코드 전문을 결과 JSON에 담는 것이 유일한
//     보존 채널이다(brooks F7 완화책).
type CleanupAbandonRequest struct {
	ID          string
	Reason      string
	Apply       bool
	Confirm     bool
	Fingerprint string
}

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

type CleanupAbandonResult struct {
	OK                 bool     `json:"ok"`
	ID                 string   `json:"id"`
	Preview            bool     `json:"preview"`
	Reason             string   `json:"reason,omitempty"`
	Missing            []string `json:"missing,omitempty"`
	ReasonError        string   `json:"reason_error,omitempty"`
	PendingIntentError string   `json:"pending_intent_error,omitempty"`
	OrcaResidueError   string   `json:"orca_residue_error,omitempty"`
	Fingerprint        string   `json:"fingerprint,omitempty"`
	WorktreePath       string   `json:"worktree_path,omitempty"`
	Branch             string   `json:"branch,omitempty"`
	WorktreePresent    bool     `json:"worktree_present"`
	BranchPresent      bool     `json:"branch_present"`
	PendingOperationID string   `json:"pending_operation_id,omitempty"`
	IntentRowsDeleted  []string `json:"intent_rows_deleted,omitempty"`
	RecordDeleted      bool     `json:"record_deleted,omitempty"`
	AbandonedAt        string   `json:"abandoned_at,omitempty"`
	FailedStep         string   `json:"failed_step,omitempty"`
	NextCommand        string   `json:"next_command,omitempty"`
	// Record는 삭제 대상 레코드 전문이다. C2-F6 예외의 유일한 보존 채널이므로
	// preview와 apply 양쪽 결과에 담는다 — preview에만 담으면 preview 이후
	// apply 직전까지의 변경분이 어디에도 남지 않는다.
	Record *IssueOpsRecord `json:"record,omitempty"`
}

// cleanupAbandonInventory는 fingerprint 입력이 되는 현재 관측 상태다.
// cleanupFinishInventory의 패턴을 차용하되 재사용하지는 않는다(brooks F12):
// abandon의 준비 상태는 "잔여물이 없음"이고 finish의 준비 상태는 "잔여물을
// 지금 지울 수 있음"이라, 같은 필드가 정반대 의미를 갖는다.
type cleanupAbandonInventory struct {
	ID                 string `json:"id"`
	Repo               string `json:"repo"`
	Branch             string `json:"branch"`
	WorktreeRoot       string `json:"worktree_root"`
	WorktreePresent    bool   `json:"worktree_present"`
	BranchOID          string `json:"branch_oid"`
	Phase              string `json:"phase"`
	LeaseStatus        string `json:"lease_status"`
	PendingOperationID string `json:"pending_operation_id"`
}

// CleanupAbandon은 게이트 8종을 평가하고, apply에서 원격을 건드리지 않은 채
// 레코드와 그 external intent 행들을 하나의 원자 배치로 삭제한다.
func CleanupAbandon(ctx context.Context, stateRoot string, req CleanupAbandonRequest, deps CleanupAbandonDeps) (CleanupAbandonResult, error) {
	if deps.Git == nil {
		deps.Git = func(dir string, args ...string) (int, string) {
			code, stdout, stderr := preflight.GitCmd(dir, args...)
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
	result := CleanupAbandonResult{OK: true, ID: record.ID, Preview: !req.Apply, Reason: strings.TrimSpace(req.Reason)}
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
	deleted, err := deleteAbandonedIssueOps(ctx, stateRoot, record, cleanupAbandonIntentOperationIDs(record))
	if err != nil {
		result.OK = false
		result.FailedStep = "record_delete"
		result.NextCommand = fmt.Sprintf("agent-harness issueops cleanup abandon --id %s --reason %q --preview --json", record.ID, result.Reason)
		return result, fmt.Errorf("cleanup abandon deletion failed (record preserved): %w", err)
	}
	result.IntentRowsDeleted = deleted
	result.RecordDeleted = true
	result.AbandonedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

// cleanupAbandonGates는 게이트 8종을 전부 평가하고 missing을 나열한다(첫 실패에
// 멈추지 않는다 — 운영자가 한 번의 preview로 모든 결격 사유를 본다).
func cleanupAbandonGates(ctx context.Context, stateRoot string, record IssueOpsRecord, req CleanupAbandonRequest, deps CleanupAbandonDeps, result *CleanupAbandonResult) (cleanupAbandonInventory, []string) {
	missing := []string{}
	// ① 사유 필수. 삭제 근거 없는 수명 종료는 감사 불가능하다.
	if err := validateCleanupAbandonReason(req.Reason); err != nil {
		missing = append(missing, "reason_required")
		result.ReasonError = err.Error()
	}
	// ② done은 finish/prune 전용이다. abandon이 done을 삼키면 머지 증적 보존
	// 경로(reflect→finish)를 우회하는 탈출구가 된다.
	if record.Phase == IssueOpsPhaseDone {
		missing = append(missing, "phase_not_done")
	}
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
	// ④ 머지 증적을 가진 레코드의 정답은 reflect→finish다.
	if record.RemoteArtifact != nil {
		missing = append(missing, "no_remote_artifact")
	}
	// ⑧ 자식 고아 방지(finish의 child_tasks_closed 대응물, brooks F6).
	if cleanupAbandonHasChildren(record) {
		missing = append(missing, "no_children")
	}

	inventory := cleanupAbandonInventory{
		ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(record.Branch),
		Phase: string(record.Phase), LeaseStatus: "none",
	}
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
	// ⑥ 로컬 워크트리 잔여가 있으면 거부. abandon은 아무것도 지우지 않는
	// 경로이므로, 잔여물이 있으면 그것을 지울 수 있는 경로(finish/orphan)가
	// 정답이다. 원격 브랜치 잔여는 비범위다(brooks F8 — doctor가 별도 추적).
	if inventory.WorktreePresent {
		missing = append(missing, "worktree_absent")
	}
	// ⑦ 로컬 브랜치 ref 잔여.
	if inventory.Branch != "" {
		if code, out := deps.Git(record.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+inventory.Branch); code == 0 {
			inventory.BranchOID = strings.TrimSpace(out)
			result.BranchPresent = true
			missing = append(missing, "branch_absent")
		}
	}
	// ⑤ pending external intent.
	if record.Execution != nil && record.Execution.Pending != nil {
		if err := cleanupAbandonPendingSafe(ctx, stateRoot, record, inventory, deps); err != nil {
			missing = append(missing, "pending_intent_safe")
			result.PendingIntentError = err.Error()
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

// cleanupAbandonLeaseHoldsWriter는 lease가 아직 writer를 붙들고 있는지 본다.
// 알 수 없는 상태는 writer 보유로 다룬다 — 모르는 상태를 통과시키면 게이트가
// fail-open이 된다.
func cleanupAbandonLeaseHoldsWriter(status model.LeaseStatus) bool {
	switch status {
	case model.LeaseStatusClaimable, model.LeaseStatusReleased:
		return false
	default:
		return true
	}
}

// cleanupAbandonOrcaResourcesAbsent는 레코드를 지워도 orca 자원이 소유자를 잃지
// 않는지 판정한다.
//
// abandon은 아무것도 지우지 않는 경로다. orca task가 살아 있는데 레코드를
// 지우면 그 task는 소유자 조회가 영구히 0건이 되어 operational_task_residue로
// 계속 보고된다 — #130이 정상 완료 경로에서 고친 것과 똑같은 증상이 이 경로로
// 재현된다. 그래서 잔여물을 지울 수 있는 경로로 보낸다.
//
// 레코드에 바인딩이 있다는 것만으로 막지 않고 실조회한다. 그렇게 하면 orca에서
// 이미 정리된 사이클까지 차단해 중도 포기 경로가 사라진다.
//
// 조회할 수 없으면 통과가 아니라 거부다(#106 pending_intent_safe와 같은 계약).
func cleanupAbandonOrcaResourcesAbsent(ctx context.Context, record IssueOpsRecord, deps CleanupAbandonDeps) error {
	if record.Execution == nil || record.Execution.Mode != model.ExecutionModeOrca || record.Execution.Orca == nil {
		return nil
	}
	binding := *record.Execution.Orca
	if strings.TrimSpace(binding.TaskID) == "" {
		return nil
	}
	if deps.OrcaOwner == nil {
		return fmt.Errorf("Orca owner inspector is not configured; resolve this cycle with `agent-harness issueops cleanup finish` or `agent-harness issueops cleanup orphan`")
	}
	inventory, err := deps.OrcaOwner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, TaskID: binding.TaskID,
		DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
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
// 경고 — stage 불변식 의존성(brooks C-1·C-2):
// 이 게이트의 안전성은 전적으로 validateExternalOrcaIntentPayload
// (execution_orca_intent.go:363-367)의 worktree 단계 불변식에 의존한다. 그
// 검사는 stage == worktree_create인 payload에 Prepared·Launch·ClaimTokenSHA256·
// TerminalPTYID·TaskID가 전부 공백임을 강제한다 — 즉 이 단계에서 존재할 수 있는
// 유일한 산출물은 워크트리 디렉토리 하나뿐이다. kind allowlist를
// worktree_create 밖으로 넓히는 순간 claim token·owner prompt·context packet·
// terminal PTY·orca task가 함께 열리며, 이 게이트는 그것들의 부재를 검사하지
// 않는다. 넓히려면 이 함수가 아니라 저 불변식부터 다시 설계해야 한다.
func cleanupAbandonPendingSafe(ctx context.Context, stateRoot string, record IssueOpsRecord, inventory cleanupAbandonInventory, deps CleanupAbandonDeps) error {
	pending := record.Execution.Pending
	// (a) kind allowlist — 로컬 orca mutation 한정. remote PR/MR 계열 kind는
	// 원격 고아 PR을 남길 수 있으므로 무조건 거부하고 reconcile로 보낸다.
	if pending.Kind != string(port.ExecutionOrcaIntentWorktree) {
		// reconcile을 지시하는 것만으로는 부족했다. 실측에서 운영자는 reconcile을
		// 완주한 뒤 무엇을 해야 하는지 알 수 없었다(#139) — 남은 절차를 함께
		// 알려야 게이트 응답이 탈출 경로가 된다(#140).
		return fmt.Errorf("pending external intent kind %q is not local-only; run `agent-harness issueops execution reconcile --id %s --preview --json` until it settles, then reclaim the Orca worktree with `orca worktree remove` before retrying abandon",
			pending.Kind, record.ID)
	}
	if record.Execution.Mode != model.ExecutionModeOrca {
		return fmt.Errorf("worktree_create intent requires an Orca execution record")
	}
	// (b) 그 단계의 유일한 산출물 위치가 디스크에 없어야 한다.
	if inventory.WorktreePresent {
		return fmt.Errorf("recorded workspace root still exists on disk")
	}
	// (c) sealed marker 실조회. 레코드는 "없음"을 증명하지 못한다.
	payload, err := readExternalOrcaIntentPayload(stateRoot, pending.OperationID)
	if err != nil {
		return err
	}
	if payload.LifecycleID != record.ID || payload.Marker != pending.Marker {
		return fmt.Errorf("Orca external intent row does not belong to this lifecycle")
	}
	if deps.Orca == nil {
		return fmt.Errorf("Orca intent inspector is not configured")
	}
	request, err := executionOrcaIntentRequest(record, payload)
	if err != nil {
		return err
	}
	inspected, err := deps.Orca.InspectIntent(ctx, request)
	if err != nil {
		return fmt.Errorf("Orca intent inventory is ambiguous; intent retained: %w", err)
	}
	if len(inspected.Candidates) != 0 {
		return fmt.Errorf("Orca intent inventory found %d candidate(s); the mutation may have landed", len(inspected.Candidates))
	}
	if !inspected.AuthoritativeZero {
		return fmt.Errorf("Orca intent inventory returned a non-authoritative zero")
	}
	return nil
}

// cleanupAbandonIntentOperationIDs는 삭제 대상 external intent 행 키를
// {Pending.OperationID} ∪ {Failure.OperationID}로 모은다(공백·중복 제거).
// Failure는 Pending과 같은 operation을 가리키는 것이 보통이지만, 실패 접수 후
// pending이 정리된 레코드에서는 Failure만 행을 참조할 수 있다.
func cleanupAbandonIntentOperationIDs(record IssueOpsRecord) []string {
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
func deleteAbandonedIssueOps(ctx context.Context, stateRoot string, record IssueOpsRecord, operationIDs []string) ([]string, error) {
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return nil, err
	}
	id, err := normalizeIssueOpsID(record.ID)
	if err != nil {
		return nil, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	var deleted []string
	err = withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		// 임계구역 재검사: fingerprint는 lock 밖에서 계산됐다. 권위 필드가
		// 그 사이 바뀌었다면 이 apply는 다른 상태를 지우는 것이 된다.
		current, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		// lease 판정은 게이트 ③과 같은 함수를 쓴다. 같은 조건을 두 곳에서 각각
		// 표현하면 한쪽만 바뀌었을 때 preview가 fingerprint까지 발급한 뒤 apply가
		// 거부되는 상태가 생긴다 — 운영자에게는 이유 없이 막히는 것으로 보인다
		// (#143, #142에서 실측).
		if current.Phase != record.Phase || current.RemoteArtifact != nil || cleanupAbandonHasChildren(current) ||
			(current.Execution != nil && cleanupAbandonLeaseHoldsWriter(current.Execution.Lease.Status)) {
			return fmt.Errorf("abandon authority changed before deletion CAS")
		}
		rows := []string{}
		mutations := []sqlstore.Mutation{}
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
			mutations = append(mutations, sqlstore.Mutation{Bucket: externalIntentBucket, ID: operationID, Delete: true})
			rows = append(rows, operationID)
		}
		// 스테이징 artifact는 레코드와 수명을 같이한다(C4a-F1 ②).
		mutations = append(mutations,
			sqlstore.Mutation{Bucket: artifactStageBucket, ID: id, Delete: true},
			sqlstore.Mutation{Bucket: issueOpsBucket, ID: id, Delete: true},
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

func cleanupAbandonHasChildren(record IssueOpsRecord) bool {
	if len(record.ChildCycles) > 0 {
		return true
	}
	for _, link := range record.IssueLinks {
		if link.Type == "child" && strings.TrimSpace(link.CloseVerifiedAt) == "" {
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

func cleanupAbandonFingerprint(inventory cleanupAbandonInventory) (string, error) {
	data, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
