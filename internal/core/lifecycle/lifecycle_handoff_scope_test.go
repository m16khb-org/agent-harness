package lifecycle

import (
	"path/filepath"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
)

func TestIssueOpsFenceNeverCapturesOrdinarySourceMutation(t *testing.T) {
	cases := []struct {
		name    string
		fixture func(*testing.T) (string, IssueOpsRecord, string)
	}{
		{
			name: "coordinator preparing",
			fixture: func(t *testing.T) (string, IssueOpsRecord, string) {
				return lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
			},
		},
		{
			name: "dispatched",
			fixture: func(t *testing.T) (string, IssueOpsRecord, string) {
				return lifecycleHandoffRecord(t, handoff.StateDispatched)
			},
		},
		{
			name: "claimed",
			fixture: func(t *testing.T) (string, IssueOpsRecord, string) {
				return lifecycleHandoffRecord(t, handoff.StateClaimed)
			},
		},
		{
			name: "submitted",
			fixture: func(t *testing.T) (string, IssueOpsRecord, string) {
				return lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
			},
		},
		{
			name: "recovery required",
			fixture: func(t *testing.T) (string, IssueOpsRecord, string) {
				return strandedRecoveryRequiredRecord(t)
			},
		},
		{
			name: "closed",
			fixture: func(t *testing.T) (string, IssueOpsRecord, string) {
				return lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionWorkerFailed)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, record, _ := tc.fixture(t)
			target := filepath.Join(repo, "internal", "ordinary-source-work.go")
			requests := []HookToolUseLifecycleRequest{
				handoffEditRequest(record, repo, "codex", "unrelated-session", target),
				{
					Repo: repo, CWD: repo, Tool: "Bash", Command: "printf source-only > " + shellQuote(target),
					EnforceWorktree: true, ExpectedWorktree: record.WorktreePath, SourceCheckout: repo,
				},
				{
					Repo: repo, CWD: repo, Tool: "Bash", Command: "git add README.md",
					EnforceWorktree: true, ExpectedWorktree: record.WorktreePath, SourceCheckout: repo,
				},
				{
					Repo: repo, CWD: repo, Tool: "mcp__filesystem__write_file", Paths: []string{target},
					EnforceWorktree: true, ExpectedWorktree: record.WorktreePath, SourceCheckout: repo,
				},
			}
			for _, req := range requests {
				if _, selected, reason := selectSupervisedHandoffRecord(req); selected || reason != "" {
					t.Fatalf("source-only request must not select a supervised record: request=%+v selected=%v reason=%q", req, selected, reason)
				}
				if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
					t.Fatalf("ordinary source-root request must be allowed: request=%+v result=%+v", req, got)
				}
			}
		})
	}
}
