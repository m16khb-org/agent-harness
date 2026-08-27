package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"agent-harness/internal/adapter/issueops/pathutil"
	"agent-harness/internal/contract/issueops"
	issueopsdomain "agent-harness/internal/domain/issueops"
	"agent-harness/internal/port"
)

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
	ReflectAudit func(record issueops.IssueOpsRecord, completion port.IssueProviderCompletionSection, audit string) error
	// Processes는 워크트리 점유 관측·종료 표면이고 OrcaTerminals는 워크트리에 매인
	// Orca 터미널 인벤토리·종료 표면이다. 둘 다 nil이면 기본 구현 또는 "Orca 없음"
	// 으로 동작한다(#477).
	Processes     CleanupProcessDeps
	OrcaTerminals port.CleanupOrcaTerminals
	// ObserveArtifact는 원격 artifact의 현재 상태를 provider에서 읽는다.
	// replacement 증거 검증의 유일한 근거이며, 주입되지 않으면 그 경로는 열리지
	// 않는다 — 관측 없이 증거를 인정하지 않는다(#283).
	ObserveArtifact func(url string) (issueopsdomain.ArtifactObservation, error)
}

// cleanupFinishRemedyCommand는 해소 경로가 하나로 정해지는 missing에만 그 명령을
// 돌려준다. 상황에 따라 갈리는 항목(worktree_clean, remote_branch_absent 등)은
// 담지 않는다 — 틀린 안내는 안내가 없는 것보다 나쁘다(이슈 #154).
func cleanupFinishRemedyCommand(id string, missing []string) string {
	if slices.Contains(missing, "completion_reflected") {
		return fmt.Sprintf("agent-harness issueops remote reflect-completion --id %s --confirm --json", id)
	}
	return ""
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
	// SupersededBy는 replacement 증거를 fingerprint 입력에 포함시킨다. 증거가
	// 바뀌면 preview는 무효가 되어야 한다.
	SupersededBy string `json:"superseded_by,omitempty"`
	// WorkspaceProcesses와 OrcaTerminals는 apply ①′가 종료할 집합이다. preview 뒤
	// 집합이 바뀌면 fingerprint가 달라져 apply가 멈춘다(#477).
	WorkspaceProcesses []issueops.NativeProcessReceipt `json:"workspace_processes,omitempty"`
	OrcaTerminals      []string                        `json:"orca_terminals,omitempty"`
	// OrcaAppPID는 ①′가 시그널에서 제외할 Orca 앱 pid다. fingerprint 입력이므로
	// preview 뒤 런타임이 사라지거나 재시작되면 apply가 멈춘다.
	OrcaAppPID       int  `json:"orca_app_pid,omitempty"`
	OrcaRuntimeReady bool `json:"orca_runtime_ready,omitempty"`
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
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return CleanupFinishResult{OK: false, ID: req.ID}, err
	}
	result := CleanupFinishResult{OK: true, ID: record.ID, Preview: !req.Apply}
	inventory, missing := cleanupFinishGates(ctx, record, req, deps, &result)
	result.Missing = missing
	if len(missing) > 0 {
		result.OK = false
		result.NextCommand = cleanupFinishRemedyCommand(record.ID, missing)
		return result, fmt.Errorf("cleanup finish is not ready: %s", strings.Join(missing, ", "))
	}
	fingerprint, err := cleanupFinishFingerprint(inventory)
	if err != nil {
		return CleanupFinishResult{OK: false, ID: record.ID}, err
	}
	result.Fingerprint = fingerprint
	if !req.Apply {
		result.NextCommand = fmt.Sprintf("agent-harness issueops cleanup finish --id %s --apply --confirm --fingerprint %s%s --json",
			record.ID, fingerprint, cleanupSupersededByFlag(result.SupersededBy))
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
	// ①′ 워크트리 점유 프로세스·Orca 터미널 종료. 재관측으로 점유 0을 증명하지
	// 못하면 아무것도 지우지 않고 멈춘다(#477).
	if inventory.WorktreePresent && (len(result.WorkspaceProcesses) > 0 || len(inventory.OrcaTerminals) > 0 || inventory.OrcaRuntimeReady) {
		stopped, terminals, err := cleanupStopWorkspace(ctx, inventory.WorktreeRoot, result.WorkspaceProcesses, inventory.OrcaTerminals, inventory.OrcaRuntimeReady, inventory.OrcaAppPID, deps.Processes, deps.OrcaTerminals)
		result.WorkspaceProcessesStopped = stopped
		result.OrcaTerminalsStopped = terminals
		if err != nil {
			return fail(issueops.CleanupFailureStepWorkspaceProcessesStop, err)
		}
	}
	// ② orca 회수 먼저(인벤토리 정합), force=false.
	if inventory.OrcaWorktreeID != "" {
		if deps.RemoveOrcaWorktree == nil {
			return fail(issueops.CleanupFailureStepOrcaRemove, fmt.Errorf("orca worktree remover is not configured"))
		}
		if err := deps.RemoveOrcaWorktree(ctx, inventory.OrcaWorktreeID); err != nil {
			return fail(issueops.CleanupFailureStepOrcaRemove, err)
		}
		result.OrcaRemoved = true
	}
	// ③ git worktree 제거(부재 = 성공). Orca는 정상 제거 시 연결된 Git
	// worktree까지 함께 없애므로, 성공 직후 경로를 다시 관측해 이중 삭제를
	// 피한다. 직접 정리와 경합해 명령 도중 경로가 사라진 경우도 멱등 성공이다.
	worktreePresent := inventory.WorktreePresent
	if worktreePresent && result.OrcaRemoved {
		if _, err := os.Lstat(inventory.WorktreeRoot); os.IsNotExist(err) {
			worktreePresent = false
			result.WorktreeRemoved = true
		}
	}
	if worktreePresent {
		if code, out := deps.Git(record.Repo, "worktree", "remove", inventory.WorktreeRoot); code != 0 {
			if _, err := os.Lstat(inventory.WorktreeRoot); !os.IsNotExist(err) {
				return fail(issueops.CleanupFailureStepWorktreeRemove, fmt.Errorf("git worktree remove: %s", out))
			}
		}
		result.WorktreeRemoved = true
	}
	// ④ 로컬 브랜치 삭제(head OID CAS, 부재 = 생략).
	if inventory.BranchOID != "" {
		if code, out := deps.Git(record.Repo, "update-ref", "-d", "refs/heads/"+inventory.Branch, inventory.BranchOID); code != 0 {
			// ③이 Orca/Git worktree를 제거하면서 linked branch ref까지 함께
			// 회수하는 순서가 있다. 그때 이 단계의 대상은 이미 없고, 부재는
			// 삭제가 목표로 하던 상태 그 자체다. 실패로 처리하면 첫 apply가
			// exact-once/idempotent 계약을 깨고 두 번 실행해야 수렴한다(#291).
			//
			// 판정 근거는 오류 문자열이 아니라 **재관측**이다. Git 메시지는
			// 버전·로케일에 따라 달라지지만 ref 존재 여부는 그렇지 않다.
			// permission, lock contention, OID drift는 ref가 남아 있으므로
			// 그대로 실패한다.
			if branchRefPresent(deps.Git, record.Repo, inventory.Branch) {
				return fail(issueops.CleanupFailureStepBranchDelete, fmt.Errorf("git update-ref -d: %s", out))
			}
		}
		result.BranchDeleted = true
	}
	// ④' 감사 라인 best-effort 멱등 반영 — 실패해도 ⑤를 막지 않는다.
	if deps.ReflectAudit != nil {
		audit := fmt.Sprintf("cleanup 완료: worktree=%s branch=%s oid=%s stopped=%d terminals=%d at=%s",
			orNone(inventory.WorktreeRoot), orNone(inventory.Branch), orNone(inventory.BranchOID),
			len(result.WorkspaceProcessesStopped), result.OrcaTerminalsStopped,
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
		return fail(issueops.CleanupFailureStepRecordDelete, err)
	}
	result.RecordDeleted = true
	return result, nil
}

func cleanupFinishGates(ctx context.Context, record issueops.IssueOpsRecord, req CleanupFinishRequest, deps CleanupFinishDeps, result *CleanupFinishResult) (cleanupFinishInventory, []string) {
	missing := []string{}
	if record.Phase != IssueOpsPhaseDone {
		missing = append(missing, "phase_done")
	}
	if record.Execution != nil && record.Execution.Lease.Status != issueops.LeaseStatusReleased {
		missing = append(missing, "lease_released")
	}
	if !req.Merged {
		// 원래 artifact가 unmerged여도, 후속 artifact가 그 변경을 명시적으로
		// 대체해 머지됐다면 정리할 수 있어야 한다. 그 경로가 없어서 finish도
		// abandon도 받지 않는 record가 실제로 생겼다(#283).
		if err := verifySupersedingArtifact(record, req, deps); err != nil {
			result.SupersedeError = err.Error()
			missing = append(missing, "remote_artifact_merged")
		} else {
			result.SupersededBy = strings.TrimSpace(req.SupersededBy)
		}
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
		// A verified replacement is a different artifact and may intentionally
		// target the parent branch's base. Its provider-observed base must exist,
		// but comparing it to the original child PR base is a category error.
		case result.SupersededBy != "":
		default:
			defaultBranch, basePresent, observed := observeMergedBaseRefs(record, preparedBase, deps)
			if slugs := classifyMergedBase(preparedBase, observedBase, defaultBranch, basePresent, observed); len(slugs) > 0 {
				missing = append(missing, slugs...)
			} else if observedBase != preparedBase {
				result.RetargetedBase = &issueops.CleanupRetargetedBase{
					PreparedBase: preparedBase, ObservedBase: observedBase,
					DefaultBranch: defaultBranch, PreparedBaseRemoteAbsent: true,
				}
			}
		}
	}
	for _, link := range record.IssueLinks {
		if link.Type == "child" && strings.TrimSpace(link.CloseVerifiedAt) == "" {
			missing = append(missing, "child_tasks_closed")
			break
		}
	}
	inventory := cleanupFinishInventory{
		ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(record.Branch),
		// replacement 증거는 fingerprint 입력이다. 증거가 바뀌면 preview는
		// 무효가 되어야 한다.
		SupersededBy: result.SupersededBy,
	}
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
		// 점유 프로세스는 차단 사유가 아니라 apply ①′의 종료 대상이다. 관측 불가,
		// 요청자 점유, 소스 체크아웃만 fail-closed로 남는다(#154, #477).
		observation, workspaceMissing := cleanupWorkspaceGatesForRecord(ctx, record, inventory.WorktreeRoot, deps.Processes, deps.OrcaTerminals)
		missing = append(missing, workspaceMissing...)
		inventory.WorkspaceProcesses = observation.Receipts
		inventory.OrcaTerminals = observation.Terminals
		inventory.OrcaAppPID = observation.AppPID
		inventory.OrcaRuntimeReady = observation.RuntimeReady
		result.WorkspaceProcesses = observation.Occupants
		result.OrcaTerminals = observation.Terminals
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

// classifyMergedBase는 준비 base와 관측 base의 관계를 판정한다. 빈 문자열이면
// 통과다.
//
// stacked PR의 부모 브랜치가 머지되어 삭제되면 provider는 자식 PR을 기본
// 브랜치로 재타깃한다. 그 흐름은 drift가 아니지만, 삭제된 브랜치의 base는
// 사후에 관측할 수 없다. 그래서 "준비 base가 원격에 없다 + 관측 base가 기본
// 브랜치다"라는 두 관측으로만 정상 재타깃을 인정한다(#490). 준비 base가 아직
// 살아 있거나 기본 브랜치가 아닌 곳으로 머지됐으면 그대로 drift이며, 관측
// 자체가 실패하면 통과가 아니라 거부다 — 자기주장 승인 플래그는 두지 않는다.
func classifyMergedBase(preparedBase, observedBase, defaultBranch string, preparedBaseRemotePresent, observed bool) []string {
	if preparedBase == "" || observedBase == preparedBase {
		return nil
	}
	// 관측 실패는 drift를 지우지 않는다. 준비 base와 다른 곳으로 머지된 것은
	// provider readback이 이미 관측한 사실이고, 관측하지 못한 것은 면제 조건뿐
	// 이므로 두 사실을 모두 보고한다.
	if !observed || strings.TrimSpace(defaultBranch) == "" {
		return []string{"base_branch_drifted", "merged_base_remote_unobserved"}
	}
	if !preparedBaseRemotePresent && observedBase == defaultBranch {
		return nil
	}
	return []string{"base_branch_drifted"}
}

// observeMergedBaseRefs는 준비 base의 원격 존재 여부와 저장소 기본 브랜치를
// 읽는다. remote_branch_absent와 같은 관측 표면(ls-remote)이며, 결과는
// fingerprint 입력이 아니다. 어떤 단계든 실패하면 observed=false로 돌려
// 판정을 fail-closed로 만든다.
func observeMergedBaseRefs(record issueops.IssueOpsRecord, preparedBase string, deps CleanupFinishDeps) (defaultBranch string, preparedBaseRemotePresent, observed bool) {
	if deps.Git == nil {
		return "", false, false
	}
	code, out := deps.Git(record.Repo, "ls-remote", "--heads", "origin", "refs/heads/"+preparedBase)
	if code != 0 {
		return "", false, false
	}
	preparedBaseRemotePresent = len(strings.Fields(strings.TrimSpace(out))) > 0
	code, out = deps.Git(record.Repo, "ls-remote", "--symref", "origin", "HEAD")
	if code != 0 {
		return "", preparedBaseRemotePresent, false
	}
	// `ref: refs/heads/<name>\tHEAD` 첫 줄만 유효하다. 원격 HEAD가 설정되지
	// 않았거나 형식이 다르면 관측 실패다.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), "ref: refs/heads/")
		if !found {
			continue
		}
		if name := strings.TrimSpace(strings.SplitN(rest, "\t", 2)[0]); name != "" {
			return name, preparedBaseRemotePresent, true
		}
	}
	return "", preparedBaseRemotePresent, false
}

// preparedBaseBranch는 base drift 비교의 기준값이다. 비어 있으면 비교 대상이
// 없다는 뜻이며(레거시 레코드), 그 경우 게이트는 적용되지 않는다. execution
// complete가 base_branch 없는 done 전이를 거부하므로 현재 계약을 지나온
// 사이클에서는 항상 값이 있다.
func preparedBaseBranch(record issueops.IssueOpsRecord) string {
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
		rec.CleanupFinishFailure = &issueops.IssueOpsCleanupFinishFailure{Step: step, Message: stepErr.Error(), At: now}
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
func ReflectCleanupAudit(stateRoot string, record issueops.IssueOpsRecord, completion port.IssueProviderCompletionSection, audit string, prov port.IssueProvider) error {
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
	_, err = stampRemoteCompletion(stateRoot, record.ID, func(rc *issueops.IssueOpsRemoteCompletion, now string) {
		rc.ReflectedAt = now
	})
	return err
}

// branchRefPresent는 exact local branch ref가 여전히 존재하는지 재관측한다.
//
// `update-ref -d` 실패를 부재로 정규화할지 판정하는 유일한 근거다. Git의 오류
// 문구는 버전과 로케일에 따라 달라지므로 문자열 매칭 대신 ref 자체를 다시
// 읽는다. 관측이 불가능하면(예: Git 호출 자체가 실패) 존재하는 것으로 보아
// fail-closed한다 — 부재를 증명하지 못한 상태에서 성공으로 정규화하면 실제
// 실패를 삼키게 된다.
func branchRefPresent(git func(dir string, args ...string) (int, string), repo, branch string) bool {
	if git == nil || strings.TrimSpace(branch) == "" {
		return true
	}
	code, _ := git(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return code == 0
}

// verifySupersedingArtifact는 replacement 증거를 provider readback으로 검증한다.
//
// 증거가 없으면 조용히 실패한다 — 기존 merged 게이트가 그대로 남는다는 뜻이다.
// 증거가 있는데 관측을 못 하거나 검증에 실패하면 그 사유를 돌려주어 결과에
// 표면화한다. 무흔적 실패는 사용자를 다시 dead-end로 보낸다.
func verifySupersedingArtifact(record issueops.IssueOpsRecord, req CleanupFinishRequest, deps CleanupFinishDeps) error {
	candidate := strings.TrimSpace(req.SupersededBy)
	if candidate == "" {
		return fmt.Errorf("no superseding artifact was provided")
	}
	if deps.ObserveArtifact == nil {
		return fmt.Errorf("superseding artifact cannot be verified: provider observation is not configured")
	}
	if record.RemoteArtifact == nil || strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		return fmt.Errorf("original artifact URL is unknown; cannot verify a supersede relation")
	}
	replacement, err := deps.ObserveArtifact(candidate)
	if err != nil {
		return fmt.Errorf("superseding artifact %s could not be observed: %w", candidate, err)
	}
	original := issueopsdomain.ArtifactObservation{
		URL:      record.RemoteArtifact.URL,
		Provider: record.RemoteArtifact.Provider,
	}
	return issueopsdomain.ValidateSupersedingArtifact(original, replacement)
}
