package issueops

import (
	"fmt"
	"strings"

	"agent-harness/internal/contract/issueops"
	issueopsartifactdomain "agent-harness/internal/domain/issueopsartifact"
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
