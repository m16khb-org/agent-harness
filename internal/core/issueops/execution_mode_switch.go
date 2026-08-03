package issueops

import (
	"context"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/preflight"
)

// ExecutionSwitchModeRequest는 준비된 실행의 모드를 바꾸는 입력이다.
//
// prepare가 아니라 별도 명령인 이유는 파괴 범위다. Orca는 기존 브랜치나 경로를
// 입양하지 않고 path_collision으로 거부하므로(orca
// src/main/ipc/workspace-create-error-classifier.ts) 전환은 반드시 기존 워크트리와
// 로컬 브랜치 제거를 동반한다. cleanup abandon과 finish가 각자 이름을 가진 것과
// 같은 이유로, 그 조작을 준비 명령 안에 숨기지 않는다(이슈 #167).
type ExecutionSwitchModeRequest struct {
	ID          string
	Mode        string
	CWD         string
	Apply       bool
	Confirm     bool
	Fingerprint string
	Actor       issueops.NativeActor
}

// ExecutionSwitchModeDependencies는 게이트 평가와 정리의 외부 표면이다.
// Git이 nil이면 preflight를 쓴다 — cleanup 경로와 같은 관례다.
type ExecutionSwitchModeDependencies struct {
	Git func(dir string, args ...string) (int, string)
}

type ExecutionSwitchModeResult struct {
	OK            bool     `json:"ok"`
	ID            string   `json:"id"`
	Preview       bool     `json:"preview"`
	CurrentMode   string   `json:"current_mode,omitempty"`
	RequestedMode string   `json:"requested_mode,omitempty"`
	Missing       []string `json:"missing,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	WorktreeRoot  string   `json:"worktree_root,omitempty"`
	Branch        string   `json:"branch,omitempty"`
	// WorktreePresent와 BranchPresent는 apply가 실제로 지울 대상이다. preview가
	// 이 둘을 보여주지 않으면 사용자는 무엇을 승인하는지 모른다.
	WorktreePresent bool `json:"worktree_present"`
	BranchPresent   bool `json:"branch_present"`
	// BranchFreeError는 orca 전환이 브랜치 이름에 막혔을 때의 원인이다.
	// missing 슬러그만으로는 이름이 로컬에 있는지 원격에 있는지 알 수 없다.
	BranchFreeError string `json:"branch_free_error,omitempty"`
	NextCommand     string `json:"next_command,omitempty"`
	SwitchedAt      string `json:"switched_at,omitempty"`
}

// switchModeInventory는 fingerprint 입력이 되는 현재 관측 상태다.
type switchModeInventory struct {
	ID              string `json:"id"`
	Repo            string `json:"repo"`
	Branch          string `json:"branch"`
	CurrentMode     string `json:"current_mode"`
	RequestedMode   string `json:"requested_mode"`
	WorktreeRoot    string `json:"worktree_root"`
	WorktreePresent bool   `json:"worktree_present"`
	BranchOID       string `json:"branch_oid"`
	LeaseStatus     string `json:"lease_status"`
	LeaseGeneration uint64 `json:"lease_generation"`
	PendingID       string `json:"pending_operation_id"`
}

// SwitchExecutionMode는 게이트를 평가하고, apply에서 워크스페이스를 정리한 뒤
// execution record를 제거해 다음 prepare가 요청한 모드로 새로 준비하게 한다.
//
// record를 새 모드로 덮어쓰지 않고 제거하는 이유는 워크스페이스 때문이다. 새
// 모드의 워크스페이스는 그 모드의 provisioner가 만들어야 하고(driver가 다르다),
// prepare가 이미 그 일을 한다. 여기서 절반만 채운 record를 남기면 두 곳이 같은
// 상태를 만들게 되어 어긋난다.
func SwitchExecutionMode(ctx context.Context, stateRoot string, req ExecutionSwitchModeRequest, deps ExecutionSwitchModeDependencies) (ExecutionSwitchModeResult, error) {
	if deps.Git == nil {
		deps.Git = func(dir string, args ...string) (int, string) {
			code, stdout, stderr := preflight.GitCmd(dir, args...)
			if code != 0 && stderr != "" {
				return code, stderr
			}
			return code, stdout
		}
	}
	requested, err := normalizeExecutionSwitchMode(req.Mode)
	if err != nil {
		return ExecutionSwitchModeResult{OK: false, ID: req.ID}, err
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return ExecutionSwitchModeResult{OK: false, ID: req.ID}, err
	}
	if record.Execution == nil {
		return ExecutionSwitchModeResult{OK: false, ID: req.ID}, fmt.Errorf(
			"IssueOps execution is not prepared; run `agent-harness issueops execution prepare --id %s --mode %s` instead", record.ID, requested)
	}

	result := ExecutionSwitchModeResult{
		OK: true, ID: record.ID, Preview: !req.Apply,
		CurrentMode: string(record.Execution.Mode), RequestedMode: requested,
	}
	inventory, missing := switchModeGates(record, requested, deps, &result)
	result.Missing = missing
	if len(missing) > 0 {
		result.OK = false
		return result, fmt.Errorf("execution switch-mode is not ready: %s", strings.Join(missing, ", "))
	}
	fingerprint, err := hashJSON(inventory)
	if err != nil {
		return ExecutionSwitchModeResult{OK: false, ID: record.ID}, err
	}
	result.Fingerprint = fingerprint
	if !req.Apply {
		result.NextCommand = fmt.Sprintf(
			"agent-harness issueops execution switch-mode --id %s --mode %s --apply --confirm --fingerprint %s --json",
			record.ID, requested, fingerprint)
		return result, nil
	}
	if !req.Confirm {
		result.OK = false
		return result, fmt.Errorf("execution switch-mode --apply requires --confirm")
	}
	// TOCTOU: preview 이후 lease·pending·워크스페이스가 바뀌었다면 그 preview는
	// 다른 상태를 승인한 것이다(cleanup abandon과 같은 계약).
	if req.Fingerprint != fingerprint {
		result.OK = false
		return result, fmt.Errorf("stale switch-mode fingerprint; run the preview again and retry with the new value")
	}
	if err := removeSwitchModeWorkspace(record, inventory, deps); err != nil {
		result.OK = false
		return result, err
	}
	// 외부 Git 조작 뒤에는 다시 공통 span lock에서 fence와 record CAS를 확인한다.
	// cleanup abandon이 먼저 arm됐다면 그 receipt를 지우지 않고 record를 보존한다.
	expectedSHA := cleanupAbandonRecordSHA(record)
	err = withIssueOpsLock(ctx, stateRoot, record.ID, func(context.Context) error {
		current, readErr := ReadIssueOps(stateRoot, record.ID)
		if readErr != nil {
			return readErr
		}
		if cleanupAbandonRecordSHA(current) != expectedSHA {
			return fmt.Errorf("execution switch-mode authority changed before record mutation")
		}
		current.Execution = nil
		current.WorktreePath = ""
		_, writeErr := writeIssueOps(stateRoot, current)
		return writeErr
	})
	if err != nil {
		result.OK = false
		return result, err
	}
	result.WorktreePresent = false
	result.BranchPresent = false
	result.SwitchedAt = executionNow(nil)
	result.NextCommand = fmt.Sprintf(
		"agent-harness issueops execution prepare --id %s --mode %s --json", record.ID, requested)
	return result, nil
}

// switchModeGates는 게이트 전부를 평가하고 missing을 나열한다(첫 실패에 멈추지
// 않는다 — 운영자가 한 번의 preview로 모든 결격 사유를 본다).
func switchModeGates(record issueops.IssueOpsRecord, requested string, deps ExecutionSwitchModeDependencies, result *ExecutionSwitchModeResult) (switchModeInventory, []string) {
	missing := []string{}
	execution := record.Execution
	inventory := switchModeInventory{
		ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(execution.Workspace.Branch),
		CurrentMode: string(execution.Mode), RequestedMode: requested,
		WorktreeRoot:    strings.TrimSpace(execution.Workspace.Root),
		LeaseStatus:     string(execution.Lease.Status),
		LeaseGeneration: execution.Lease.Generation,
	}
	result.WorktreeRoot = inventory.WorktreeRoot
	result.Branch = inventory.Branch

	// ① 모드가 실제로 바뀌어야 한다. 같은 모드로의 전환은 지울 이유가 없고,
	// 파괴 조작이 아무 일도 하지 않는 것보다 요청을 거부하는 편이 안전하다.
	if inventory.CurrentMode == requested {
		missing = append(missing, "mode_actually_changes")
	}
	// ② lease가 writer를 쥐고 있으면 안 된다. 판정 기준은 상태 이름이 아니라
	// writer의 유무이며, cleanup abandon과 같은 함수를 쓴다 — 두 곳에 조건을
	// 따로 쓰면 abandon은 허용하는데 switch는 막거나 그 반대가 된다.
	if cleanupAbandonLeaseHoldsWriter(execution.Lease.Status) {
		missing = append(missing, "lease_holds_no_writer")
	}
	// ③ pending intent는 외부 mutation이 미해소라는 뜻이다. 그 상태에서 지우면
	// 무엇이 남았는지 영영 알 수 없다.
	if execution.Pending != nil {
		inventory.PendingID = strings.TrimSpace(execution.Pending.OperationID)
		missing = append(missing, "pending_intent_absent")
	}
	// ④ 잃을 작업이 없어야 한다. 워크트리가 없으면 지울 것도 없으므로 통과다.
	if inventory.WorktreeRoot != "" {
		if _, err := os.Stat(inventory.WorktreeRoot); err == nil {
			inventory.WorktreePresent = true
			result.WorktreePresent = true
			if code, out := deps.Git(inventory.WorktreeRoot, "status", "--porcelain=v1"); code != 0 || strings.TrimSpace(out) != "" {
				missing = append(missing, "worktree_clean")
			}
			// 푸시되지 않은 커밋은 워크트리를 지우면 사라진다. upstream이 없으면
			// 비교할 대상이 없으므로 커밋 존재 자체를 잃을 작업으로 본다.
			if inventory.Branch != "" {
				if code, out := deps.Git(inventory.WorktreeRoot, "rev-list", "--count", "refs/remotes/origin/"+inventory.Branch+".."+"HEAD"); code == 0 {
					if strings.TrimSpace(out) != "0" {
						missing = append(missing, "worktree_commits_pushed")
					}
				} else if code, out := deps.Git(inventory.WorktreeRoot, "rev-list", "--count", strings.TrimSpace(execution.Workspace.BaseHead)+".."+"HEAD"); code != 0 || strings.TrimSpace(out) != "0" {
					// upstream을 못 읽으면 base 대비로 판정한다. 둘 다 실패하면
					// 관측 불가이므로 fail-closed다.
					missing = append(missing, "worktree_commits_pushed")
				}
			}
		}
	}
	if inventory.Branch != "" {
		if code, out := deps.Git(record.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+inventory.Branch); code == 0 {
			inventory.BranchOID = strings.TrimSpace(out)
			result.BranchPresent = true
		}
	}
	// ⑤ orca로 가려면 정리가 끝난 뒤에도 브랜치 이름이 비어 있어야 한다. Orca는
	// 언제나 새 브랜치를 만들고 이름이 쓰이고 있으면 접미사를 붙이므로
	// (#149·#154), 여기서 통과시키면 전환은 성공하는데 바로 다음 prepare가
	// orca_branch_name_taken으로 막힌다 — 워크트리만 잃고 제자리다.
	//
	// prepare의 ensureOrcaBranchIsFree를 재사용하지 않는다. 그 함수는 "지금 이름이
	// 비어 있는가"를 묻고 로컬 refs까지 보는데, 여기서 로컬 브랜치는 전환이 지울
	// 대상이다. 그것을 이유로 막으면 게이트가 자기가 치울 것을 근거로 거부한다.
	// 이쪽 질문은 "정리 후에도 비어 있을 것인가"이고 답은 원격에만 있다.
	//
	// 원격 브랜치는 provider가 이슈에 연결한 것이므로 switch-mode가 지우지
	// 않는다. #163이 정한 순서대로 orca 준비 뒤에 `gh issue develop`을 다시
	// 붙이는 것이 이 상태를 푸는 경로다.
	if requested == string(issueops.ExecutionModeOrca) && inventory.Branch != "" {
		if code, _ := deps.Git(record.Repo, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+inventory.Branch); code == 0 {
			missing = append(missing, "orca_branch_name_free")
			result.BranchFreeError = fmt.Sprintf(
				"branch %q still exists on origin, so Orca would take a suffixed name after the switch: "+
					"delete the remote branch if it holds no work, run `git fetch --prune` if it is already gone, "+
					"or prepare Orca first and re-attach the linked branch afterwards",
				inventory.Branch)
		}
	}
	return inventory, missing
}

// removeSwitchModeWorkspace는 워크트리와 로컬 브랜치를 지운다. 원격은 건드리지
// 않는다 — provider-linked 브랜치는 이슈 연결을 담고 있고, 새 모드의 준비가 그
// 이름을 다시 쓴다.
func removeSwitchModeWorkspace(record issueops.IssueOpsRecord, inventory switchModeInventory, deps ExecutionSwitchModeDependencies) error {
	if inventory.WorktreePresent {
		if code, out := deps.Git(record.Repo, "worktree", "remove", "--force", inventory.WorktreeRoot); code != 0 {
			return fmt.Errorf("switch-mode could not remove the canonical worktree (record preserved): %s", strings.TrimSpace(out))
		}
	}
	if inventory.BranchOID != "" {
		if code, out := deps.Git(record.Repo, "branch", "-D", inventory.Branch); code != 0 {
			return fmt.Errorf("switch-mode could not remove the local branch (record preserved): %s", strings.TrimSpace(out))
		}
	}
	return nil
}

func normalizeExecutionSwitchMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(issueops.ExecutionModeDirect):
		return string(issueops.ExecutionModeDirect), nil
	case string(issueops.ExecutionModeOrca):
		return string(issueops.ExecutionModeOrca), nil
	default:
		// auto는 "실행 가능한 모드를 골라 달라"는 요청이지 전환 대상이 아니다.
		// 파괴 조작의 목표를 harness가 고르게 두지 않는다.
		return "", fmt.Errorf("execution switch-mode requires an explicit --mode direct or orca")
	}
}

// executionSwitchModeCommand는 prepare가 모드 불일치를 거부할 때 안내하는
// 다음 명령이다. 거부만 하고 해소 경로를 주지 않으면 사용자가 갇힌다.
func executionSwitchModeCommand(id, mode string) string {
	return fmt.Sprintf("agent-harness issueops execution switch-mode --id %s --mode %s --json", id, mode)
}
