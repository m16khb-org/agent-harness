package issueopsauthorization

import (
	"fmt"
	"strings"

	issueopsauthorizationcontract "agent-harness/internal/contract/issueopsauthorization"
)

type SamePath func(string, string) bool

func AuthorizeExecutionMutation(
	record issueopsauthorizationcontract.Record,
	actor *issueopsauthorizationcontract.Actor,
	samePath SamePath,
) error {
	if record.Execution == nil {
		return nil
	}
	lease := record.Execution.Lease
	if lease.Status != issueopsauthorizationcontract.LeaseStatusActive || lease.Holder == nil {
		return fmt.Errorf(
			"IssueOps execution generation %d has no active write lease",
			lease.Generation,
		)
	}
	if actor == nil ||
		!strings.EqualFold(strings.TrimSpace(actor.Host), lease.Holder.Host) ||
		strings.TrimSpace(actor.SessionID) != lease.Holder.SessionID ||
		strings.TrimSpace(actor.AgentID) != lease.Holder.AgentID {
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
	if samePath == nil || !samePath(actor.CWD, record.Execution.Workspace.Root) {
		return fmt.Errorf("IssueOps execution mutation requires the canonical worktree cwd")
	}
	return nil
}
