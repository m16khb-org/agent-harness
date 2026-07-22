package issueops

import (
	"fmt"
	"strings"

	"agent-harness/internal/core/issueops/model"
)

// IssueOpsActor identifies the native host session that is authorized to make
// a durable mutation for one IssueOps cycle. CWD is deliberately part of the
// request identity: a valid session in the source checkout is not authority
// for the isolated workspace, and vice versa.
type IssueOpsActor struct {
	Host                  string                         `json:"host"`
	SessionID             string                         `json:"session_id"`
	AgentID               string                         `json:"agent_id,omitempty"`
	CWD                   string                         `json:"cwd"`
	NativeProcessAncestry []model.NativeProcessReceiptV1 `json:"-"`
}

// validateExecutionMutation binds every durable IssueOps mutation to the
// current write lease once execution has been prepared. Planning mutations are
// intentionally actor-optional until the execution record exists.
func validateExecutionMutation(record IssueOpsRecord, actor *IssueOpsActor) error {
	if record.Execution == nil {
		return nil
	}
	if err := model.ValidateExecutionV1(*record.Execution); err != nil {
		return fmt.Errorf("invalid IssueOps execution v1 record: %w", err)
	}
	lease := record.Execution.Lease
	if lease.Status != model.LeaseStatusActive || lease.Holder == nil {
		return fmt.Errorf("IssueOps execution generation %d has no active write lease", lease.Generation)
	}
	if actor == nil {
		return fmt.Errorf("IssueOps execution mutation requires the current write lease holder")
	}
	candidate := &model.NativeActorV1{
		Host:      strings.ToLower(strings.TrimSpace(actor.Host)),
		SessionID: strings.TrimSpace(actor.SessionID),
		AgentID:   strings.TrimSpace(actor.AgentID),
	}
	if !sameNativeActorIdentityV1(candidate, lease.Holder) {
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
func validateWorkspacePreparationMutation(record IssueOpsRecord, actor *IssueOpsActor) error {
	return validateExecutionMutation(record, actor)
}

// validatePostTransferMutation keeps current-contract durable writes bound to the
// owner even when callers bypass lifecycle hooks through a direct CLI or MCP
// request. Legacy cycles retain their existing actor-optional behavior.
func validatePostTransferMutation(record IssueOpsRecord, actor *IssueOpsActor) error {
	return validateExecutionMutation(record, actor)
}
