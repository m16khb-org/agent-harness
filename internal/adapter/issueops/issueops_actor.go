package issueops

import (
	"fmt"
	"strings"

	"issueops/internal/contract/issueops"
	issueopsartifactdomain "issueops/internal/domain/issueopsartifact"
)

// validateExecutionMutation binds every durable IssueOps mutation to the
// current write lease once execution has been prepared. Planning mutations are
// intentionally actor-optional until the execution record exists.
func validateExecutionMutation(record issueops.IssueOpsRecord, actor *IssueOpsActor) error {
	if record.Execution == nil {
		return nil
	}
	if err := issueops.ValidateExecution(*record.Execution); err != nil {
		return fmt.Errorf("invalid IssueOps execution v1 record: %w", err)
	}
	lease := record.Execution.Lease
	if lease.Status != issueops.LeaseStatusActive || lease.Holder == nil {
		return fmt.Errorf("IssueOps execution generation %d has no active write lease", lease.Generation)
	}
	if actor == nil {
		return fmt.Errorf("IssueOps execution mutation requires the current write lease holder")
	}
	candidate := &issueops.NativeActor{
		Host:      strings.ToLower(strings.TrimSpace(actor.Host)),
		SessionID: strings.TrimSpace(actor.SessionID),
		AgentID:   strings.TrimSpace(actor.AgentID),
	}
	if !sameNativeActorIdentity(candidate, lease.Holder) {
		return fmt.Errorf("IssueOps execution mutation requires the current write lease holder")
	}
	processMatches := false
	for _, observed := range actor.NativeProcessAncestry {
		if lease.Holder.SessionProcess != nil && observed == *lease.Holder.SessionProcess {
			processMatches = true
			break
		}
	}
	if !processMatches {
		return fmt.Errorf("IssueOps execution mutation requires the current write lease holder")
	}
	if !samePath(actor.CWD, record.Execution.Workspace.Root) {
		return fmt.Errorf("IssueOps execution mutation requires the canonical worktree cwd")
	}
	return nil
}

func validatePlanLinkMutation(record issueops.IssueOpsRecord, actor *IssueOpsActor) error {
	if record.Execution == nil || !issueopsartifactdomain.CanStage(record, "plan") {
		return validateExecutionMutation(record, actor)
	}
	host := ""
	if actor != nil {
		host = strings.ToLower(strings.TrimSpace(actor.Host))
	}
	if actor == nil || (host != "codex" && host != "claude" && host != "omo") ||
		strings.TrimSpace(actor.SessionID) == "" || len(actor.NativeProcessAncestry) == 0 ||
		!samePath(actor.CWD, record.Execution.Workspace.Root) {
		return fmt.Errorf("released Orca plan linking requires a native coordinator in the canonical worktree")
	}
	return nil
}

func validateWorkspacePreparationMutation(record issueops.IssueOpsRecord, actor *IssueOpsActor) error {
	return validateExecutionMutation(record, actor)
}

// validateRetargetMutation은 base 재타깃 기록을 lease 상태에 따라 나눠 묶는다.
//
// 재타깃은 두 시점에 일어난다. 사이클 중에는 write lease 홀더만 base를 옮길 수
// 있어야 한다 — 진행 중인 작업의 합류 지점을 다른 세션이 바꾸면 안 된다. 반면
// 재타깃이 cleanup 시점에야 드러나는 경우(자식 MR이 우산 브랜치로 옮겨져 머지된
// 뒤 finish가 drift로 막는 흐름)에는 lease가 이미 해제돼 홀더가 존재하지 않는다.
// 그 시점의 durable write는 remote reflect-completion·cleanup finish와 같은 완료
// 계열 표면이며, 이들은 lease가 아니라 관측으로 보호된다. 재타깃도 같은 규율을
// 따른다: 해제 이후에는 provider readback과 origin 존재 관측이 유일한 근거이고,
// 손으로 base를 주장하는 경로는 어느 상태에서도 없다.
func validateRetargetMutation(record issueops.IssueOpsRecord, actor *IssueOpsActor) error {
	if record.Execution == nil {
		return nil
	}
	if err := issueops.ValidateExecution(*record.Execution); err != nil {
		return fmt.Errorf("invalid IssueOps execution v1 record: %w", err)
	}
	if record.Execution.Lease.Status != issueops.LeaseStatusActive {
		return nil
	}
	return validateExecutionMutation(record, actor)
}

func ValidateIssueOpsMutationActor(stateRoot, id string, actor IssueOpsActor) error {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return err
	}
	return validateWorkspacePreparationMutation(record, &actor)
}

// validatePostTransferMutation keeps current-contract durable writes bound to the
// owner even when callers bypass lifecycle hooks through a direct CLI or MCP
// request. Legacy cycles retain their existing actor-optional behavior.
func validatePostTransferMutation(record issueops.IssueOpsRecord, actor *IssueOpsActor) error {
	return validateExecutionMutation(record, actor)
}
