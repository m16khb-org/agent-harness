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
	"agent-harness/internal/port"
)

// CleanupFinishRequest는 record-backed 머지 후 정리(설계 v5 WS3)의 입력이다.
// Merged/CompletionReflected/IssueClosed는 caller가 원격 readback으로 검증한
// 값이며, readback 실패는 caller에서 fail-closed로 끝난다(값이 오지 않는다).
type CleanupFinishRequest struct {
	ID                  string
	CWD                 string
	Merged              bool
	CompletionReflected bool
	IssueClosed         bool
	// MergedBaseBranch는 머지 readback이 함께 관측한 원격 artifact의 현재 base
	// ref다. done 전이는 draft PR 생성 직후에 일어나고 finish는 머지 이후에
	// 실행되므로 그 사이 구간에서 base가 바뀔 수 있다. 레코드의 base 값은 done
	// 시점에 검증된 과거이므로 drift를 구조적으로 검출하지 못한다 — 원격 관측만이
	// 유효한 증거다.
	MergedBaseBranch string
	Apply            bool
	Confirm          bool
	Fingerprint      string
}

// CleanupFinishDeps는 파괴 단계의 외부 표면 주입점이다. Git은 (dir, args...)를
// 실행해 (exitCode, stdout)을 돌려준다. RemoveOrcaWorktree는 orca 관리
// 워크스페이스 회수이며 "이미 없음"을 성공으로 정규화해야 한다(멱등 계약).
// ReflectAudit는 ④' 감사 라인의 멱등 병합(UpdateIssueBodySection 재사용)이다.
type CleanupFinishDeps struct {
	Git                func(dir string, args ...string) (int, string)
	RemoveOrcaWorktree func(ctx context.Context, worktreeID string) error
	// ReflectAudit는 ②(파괴 시작) 이전에 스냅샷한 completion payload에 감사
	// 라인을 더해 멱등 병합한다 — 삭제된 워크트리를 다시 읽어 보존 본문을
	// 빈 값으로 덮어쓰는 사고를 구조적으로 차단한다(C2-F1 (c)).
	ReflectAudit     func(record IssueOpsRecord, completion port.IssueProviderCompletionSection, audit string) error
	InspectProcesses func(root string) ([]string, error)
}

type CleanupFinishResult struct {
	OK              bool     `json:"ok"`
	ID              string   `json:"id"`
	Preview         bool     `json:"preview"`
	Missing         []string `json:"missing,omitempty"`
	Fingerprint     string   `json:"fingerprint,omitempty"`
	WorktreePath    string   `json:"worktree_path,omitempty"`
	Branch          string   `json:"branch,omitempty"`
	WorktreePresent bool     `json:"worktree_present"`
	BranchPresent   bool     `json:"branch_present"`
	OrcaWorktreeID  string   `json:"orca_worktree_id,omitempty"`
	OrcaRemoved     bool     `json:"orca_removed,omitempty"`
	WorktreeRemoved bool     `json:"worktree_removed,omitempty"`
	BranchDeleted   bool     `json:"branch_deleted,omitempty"`
	AuditReflected  bool     `json:"audit_reflected,omitempty"`
	AuditError      string   `json:"audit_error,omitempty"`
	RecordDeleted   bool     `json:"record_deleted,omitempty"`
	FailedStep      string   `json:"failed_step,omitempty"`
	NextCommand     string   `json:"next_command,omitempty"`
}

// cleanupFinishInventory는 fingerprint 입력이 되는 현재 관측 상태다. 부분
// 정리로 상태가 바뀌면 fingerprint도 바뀌므로 이전 preview의 값은 무효가
// 된다(5차 m3: 재실행 전 preview 재발급).
type cleanupFinishInventory struct {
	ID              string `json:"id"`
	Repo            string `json:"repo"`
	Branch          string `json:"branch"`
	WorktreeRoot    string `json:"worktree_root"`
	WorktreePresent bool   `json:"worktree_present"`
	BranchOID       string `json:"branch_oid"`
	OrcaWorktreeID  string `json:"orca_worktree_id"`
	RemoteURL       string `json:"remote_url"`
}

// CleanupFinish는 preview 게이트를 평가하고, apply에서 orca→git 순의 멱등
// 정리 후 레코드를 삭제한다. ②~④ 중 실패하면 레코드를 삭제하지 않고 실패
// 지점을 record에 남긴 채 반환한다(resumable).
func CleanupFinish(ctx context.Context, stateRoot string, req CleanupFinishRequest, deps CleanupFinishDeps) (CleanupFinishResult, error) {
	if deps.Git == nil {
		// remote_branch_absent 게이트가 finish에 첫 네트워크 호출(ls-remote)을
		// 들여온다. preflight.GitCmd에는 비대화·timeout 계약이 없어 자격증명
		// 프롬프트에 걸리면 세션을 붙잡으므로 sync-base가 확립한 계약을 쓴다.
		deps.Git = func(dir string, args ...string) (int, string) {
			return defaultExecutionSyncBaseGit(ctx, dir, args...)
		}
	}
	if deps.InspectProcesses == nil {
		deps.InspectProcesses = func(root string) ([]string, error) {
			procs, err := inspectWorkspaceProcesses(root, nil)
			if err != nil {
				return nil, err
			}
			names := make([]string, 0, len(procs))
			for _, proc := range procs {
				names = append(names, fmt.Sprintf("%d:%s", proc.PID, proc.Command))
			}
			return names, nil
		}
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return CleanupFinishResult{OK: false, ID: req.ID}, err
	}
	result := CleanupFinishResult{OK: true, ID: record.ID, Preview: !req.Apply}
	inventory, missing := cleanupFinishGates(record, req, deps, &result)
	result.Missing = missing
	if len(missing) > 0 {
		result.OK = false
		return result, fmt.Errorf("cleanup finish is not ready: %s", strings.Join(missing, ", "))
	}
	fingerprint, err := cleanupFinishFingerprint(inventory)
	if err != nil {
		return CleanupFinishResult{OK: false, ID: record.ID}, err
	}
	result.Fingerprint = fingerprint
	if !req.Apply {
		result.NextCommand = fmt.Sprintf("agent-harness issueops cleanup finish --id %s --apply --confirm --fingerprint %s --json", record.ID, fingerprint)
		return result, nil
	}
	if !req.Confirm {
		result.OK = false
		return result, fmt.Errorf("cleanup finish --apply requires --confirm")
	}
	// ① TOCTOU: apply 직전 재계산 일치. 부분 정리·외부 변경이 있었다면 여기서
	// 멈추고 preview 재발급을 요구한다.
	if req.Fingerprint != fingerprint {
		result.OK = false
		return result, fmt.Errorf("stale cleanup fingerprint; run --preview again and retry with the new value")
	}
	// C2-F1: 파괴 단계에 들어가기 전에 보존 payload를 스냅샷한다. ④'는 이
	// 스냅샷으로만 렌더하므로 워크트리 삭제 이후에도 보존 본문이 유지된다.
	completionSnapshot := gatherCompletionSection(record)
	fail := func(step string, stepErr error) (CleanupFinishResult, error) {
		result.OK = false
		result.FailedStep = step
		recordCleanupFinishFailure(stateRoot, record.ID, step, stepErr)
		result.NextCommand = fmt.Sprintf("agent-harness issueops cleanup finish --id %s --preview --json", record.ID)
		return result, fmt.Errorf("cleanup finish step %s failed (record preserved; re-run preview then apply): %w", step, stepErr)
	}
	// ② orca 회수 먼저(인벤토리 정합), force=false.
	if inventory.OrcaWorktreeID != "" {
		if deps.RemoveOrcaWorktree == nil {
			return fail("orca_remove", fmt.Errorf("orca worktree remover is not configured"))
		}
		if err := deps.RemoveOrcaWorktree(ctx, inventory.OrcaWorktreeID); err != nil {
			return fail("orca_remove", err)
		}
		result.OrcaRemoved = true
	}
	// ③ git worktree 제거(부재 = 성공).
	if inventory.WorktreePresent {
		if code, out := deps.Git(record.Repo, "worktree", "remove", inventory.WorktreeRoot); code != 0 {
			return fail("worktree_remove", fmt.Errorf("git worktree remove: %s", out))
		}
		result.WorktreeRemoved = true
	}
	// ④ 로컬 브랜치 삭제(head OID CAS, 부재 = 생략).
	if inventory.BranchOID != "" {
		if code, out := deps.Git(record.Repo, "update-ref", "-d", "refs/heads/"+inventory.Branch, inventory.BranchOID); code != 0 {
			return fail("branch_delete", fmt.Errorf("git update-ref -d: %s", out))
		}
		result.BranchDeleted = true
	}
	// ④' 감사 라인 best-effort 멱등 반영 — 실패해도 ⑤를 막지 않는다.
	if deps.ReflectAudit != nil {
		audit := fmt.Sprintf("cleanup 완료: worktree=%s branch=%s oid=%s at=%s",
			orNone(inventory.WorktreeRoot), orNone(inventory.Branch), orNone(inventory.BranchOID),
			time.Now().UTC().Format(time.RFC3339))
		if err := deps.ReflectAudit(record, completionSnapshot, audit); err == nil {
			result.AuditReflected = true
		} else {
			// best-effort지만 무흔적 실패는 금지 — 결과에 표면화한다.
			result.AuditError = err.Error()
		}
	}
	// ⑤ 레코드 삭제 — 결정적 ID 재사용과의 충돌을 끝내는 수명 종료.
	if err := deleteIssueOps(stateRoot, record.ID); err != nil {
		return fail("record_delete", err)
	}
	result.RecordDeleted = true
	return result, nil
}

func cleanupFinishGates(record IssueOpsRecord, req CleanupFinishRequest, deps CleanupFinishDeps, result *CleanupFinishResult) (cleanupFinishInventory, []string) {
	missing := []string{}
	if record.Phase != IssueOpsPhaseDone {
		missing = append(missing, "phase_done")
	}
	if record.Execution != nil && record.Execution.Lease.Status != model.LeaseStatusReleased {
		missing = append(missing, "lease_released")
	}
	if !req.Merged {
		missing = append(missing, "remote_artifact_merged")
	}
	if !req.CompletionReflected {
		missing = append(missing, "completion_reflected")
	}
	if !req.IssueClosed {
		missing = append(missing, "issue_closed")
	}
	// base_branch_drifted: finish는 레코드를 지우므로 여기서 통과하면 준비된
	// base가 아닌 브랜치로 머지된 사실을 다시 확인할 근거가 사라진다. 관측
	// 불가는 통과가 아니라 거부다. 이 관측은 fingerprint 입력이 아니다 —
	// 네트워크 관측을 인벤토리에 섞으면 일시적 원격 오류가 preview 재발급
	// 루프를 만든다(remote_branch_absent와 같은 규율).
	if preparedBase := preparedBaseBranch(record); preparedBase != "" {
		observedBase := strings.TrimSpace(req.MergedBaseBranch)
		switch {
		case observedBase == "":
			missing = append(missing, "merged_base_branch_unobserved")
		case observedBase != preparedBase:
			missing = append(missing, "base_branch_drifted")
		}
	}
	for _, link := range record.IssueLinks {
		if link.Type == "child" && strings.TrimSpace(link.CloseVerifiedAt) == "" {
			missing = append(missing, "child_tasks_closed")
			break
		}
	}
	inventory := cleanupFinishInventory{ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(record.Branch)}
	if record.RemoteArtifact != nil {
		inventory.RemoteURL = record.RemoteArtifact.URL
	}
	if record.Execution != nil {
		inventory.WorktreeRoot = strings.TrimSpace(record.Execution.Workspace.Root)
		if branch := strings.TrimSpace(record.Execution.Workspace.Branch); branch != "" {
			inventory.Branch = branch
		}
		if record.Execution.Orca != nil {
			inventory.OrcaWorktreeID = record.Execution.Orca.WorktreeID
		}
	}
	// 레거시/직접 사이클은 record.WorktreePath만 가질 수 있다. 폴백하되, 두
	// 값이 모두 있고 다르면 어느 쪽도 신뢰하지 않고 거부한다(C2-F7).
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
	result.OrcaWorktreeID = inventory.OrcaWorktreeID
	// 자기파괴 방지: CWD가 대상 워크트리 안이면 거부. CWD를 해석하지 못한
	// 경우(Getwd 실패 등)는 fail-closed로 거부한다 — 그 실패의 대표 원인이
	// 바로 "서 있던 워크트리가 삭제됨"이다(C2-F4).
	if inventory.WorktreePresent {
		cwd := strings.TrimSpace(req.CWD)
		if cwd == "" {
			missing = append(missing, "cwd_unresolved")
		} else if pathutil.PathWithin(cwd, inventory.WorktreeRoot) {
			missing = append(missing, "cwd_outside_worktree")
		}
	}
	// 부분 정리 상태는 정상 입력: 워크트리 부재 = clean 충족, 브랜치 부재 = ④ 생략.
	if inventory.WorktreePresent {
		if deps.InspectProcesses != nil {
			if procs, err := deps.InspectProcesses(inventory.WorktreeRoot); err != nil || len(procs) > 0 {
				missing = append(missing, "workspace_processes_quiescent")
			}
		}
		if code, out := deps.Git(inventory.WorktreeRoot, "status", "--porcelain=v1"); code != 0 || strings.TrimSpace(out) != "" {
			missing = append(missing, "worktree_clean")
		}
	}
	if inventory.Branch != "" {
		if code, out := deps.Git(record.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+inventory.Branch); code == 0 {
			inventory.BranchOID = strings.TrimSpace(out)
			result.BranchPresent = true
		}
		// remote_branch_absent(brooks H8): finish는 레코드를 지우므로, 원격
		// 브랜치가 남은 채로 통과하면 typed 삭제 경로(cleanup remote-branch)가
		// 그 브랜치에 영원히 닿지 못한다. 관측만 하고 원격은 건드리지 않으며,
		// 관측 불가는 fail-closed다. 이 관측은 fingerprint 입력이 아니다.
		if code, out := deps.Git(record.Repo, "ls-remote", "--heads", "origin", "refs/heads/"+inventory.Branch); code != 0 ||
			len(strings.Fields(strings.TrimSpace(out))) > 0 {
			missing = append(missing, "remote_branch_absent")
		}
	}
	return inventory, missing
}

// preparedBaseBranch는 base drift 비교의 기준값이다. 비어 있으면 비교 대상이
// 없다는 뜻이며(레거시 레코드), 그 경우 게이트는 적용되지 않는다. execution
// complete가 base_branch 없는 done 전이를 거부하므로 현재 계약을 지나온
// 사이클에서는 항상 값이 있다.
func preparedBaseBranch(record IssueOpsRecord) string {
	if record.BranchPrepare == nil {
		return ""
	}
	return strings.TrimSpace(record.BranchPrepare.BaseBranch)
}

func cleanupFinishFingerprint(inventory cleanupFinishInventory) (string, error) {
	data, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// recordCleanupFinishFailure는 실패 지점을 record에 남긴다(resumable 계약).
// 기록 실패는 원 실패 보고를 막지 않는 best-effort다.
func recordCleanupFinishFailure(stateRoot, id, step string, stepErr error) {
	_ = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		rec.CleanupFinishFailure = &IssueOpsCleanupFinishFailure{Step: step, Message: stepErr.Error(), At: now}
		rec.UpdatedAt = now
		_, err = writeIssueOps(stateRoot, rec)
		return err
	})
}

func orNone(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(없음)"
	}
	return v
}

// ReflectCleanupAudit는 ④' 감사 라인을 completion 섹션의 멱등 병합으로
// 반영한다(CleanupAudit 필드 재사용 — 동일 내용 재실행은 같은 본문을 만든다).
// completion은 파괴 시작 전에 스냅샷된 payload여야 한다(C2-F1).
//
// 이 경로는 audit 라인만 더하는 것이 아니라 completion payload 전체를 원격에
// 쓴다. 따라서 성공하면 ReflectIssueCompletion과 같은 효과이며 로컬 캐시도 함께
// 채워야 한다 — 그러지 않으면 레코드를 유지하는 cleanup remote-branch 직후
// issueops list가 원격에 반영된 사이클을 거짓으로 미반영이라 보고한다(#128).
func ReflectCleanupAudit(stateRoot string, record IssueOpsRecord, completion port.IssueProviderCompletionSection, audit string, prov port.IssueProvider) error {
	if prov == nil {
		return fmt.Errorf("no issue provider configured")
	}
	completion.CleanupAudit = audit
	result, err := prov.UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{
		Repo:       record.Repo,
		IssueURL:   record.IssueURL,
		Section:    port.IssueBodySectionCompletion,
		Completion: &completion,
		Confirm:    true,
	})
	if err != nil {
		return err
	}
	if !result.Updated {
		return fmt.Errorf("cleanup audit reflection was not confirmed")
	}
	// 확인된 반영만 캐시에 남긴다. audit 반영은 best-effort이므로 실패가 캐시를
	// 원격보다 낙관적으로 만들어서는 안 된다. finish 경로에서는 직후 레코드가
	// 삭제되어 무해하고, 파괴 단계가 중간 실패해 레코드가 잔존하면 오히려
	// 정확해진다.
	_, err = stampRemoteCompletion(stateRoot, record.ID, func(rc *IssueOpsRemoteCompletion, now string) {
		rc.ReflectedAt = now
	})
	return err
}
