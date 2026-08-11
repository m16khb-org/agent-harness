package issueopsauthorization

import (
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestAuthorizeRequiresExactLeaseHolderProcessAndWorkspace(t *testing.T) {
	process := issueopscontract.NativeProcessReceipt{
		PID:        42,
		StartedAt:  "2026-08-11T05:00:00Z",
		Executable: "/opt/codex",
	}
	record := issueopscontract.IssueOpsRecord{
		Execution: &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/decision"},
			Lease: issueopscontract.WriteLease{
				Generation: 3,
				Status:     issueopscontract.LeaseStatusActive,
				Holder: &issueopscontract.NativeActor{
					Host:           "codex",
					SessionID:      "session",
					AgentID:        "agent",
					SessionProcess: &process,
				},
			},
		},
	}
	actor := &issueopscontract.IssueOpsActor{
		Host:                  "CODEX",
		SessionID:             "session",
		AgentID:               "agent",
		CWD:                   "/repo.worktrees/decision",
		NativeProcessAncestry: []issueopscontract.NativeProcessReceipt{process},
	}
	if err := AuthorizeExecutionMutation(
		record,
		actor,
		func(left, right string) bool { return left == right },
	); err != nil {
		t.Fatal(err)
	}
	actor.NativeProcessAncestry = nil
	if err := AuthorizeExecutionMutation(
		record,
		actor,
		func(string, string) bool { return true },
	); err == nil || !strings.Contains(err.Error(), "write lease holder") {
		t.Fatalf("missing process receipt must fail closed: %v", err)
	}
}
