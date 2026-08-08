package lifecycle_test

import (
	"testing"

	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	lifecycledomain "agent-harness/internal/domain/lifecycle"
)

func TestDecidePreToolUseProjectsOneAllowOrBlock(t *testing.T) {
	if decision := lifecycledomain.DecidePreToolUse(nil); decision.Action != lifecycledomain.ActionAllow || decision.Reason != "" {
		t.Fatalf("unexpected allow decision: %+v", decision)
	}
	reason := &lifecyclecontract.IssueOpsDenyReason{Code: "wrong_worktree", Reason: "use the canonical worktree"}
	if decision := lifecycledomain.DecidePreToolUse(reason); decision.Action != lifecycledomain.ActionBlock || decision.Reason != reason.Reason {
		t.Fatalf("unexpected block decision: %+v", decision)
	}
}
