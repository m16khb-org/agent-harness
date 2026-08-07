package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func TestExecutionMutationRequiresCurrentLeaseHolderInCanonicalWorktree(t *testing.T) {
	source := t.TempDir()
	root := filepath.Join(source+".worktrees", "issue-69")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	holder := issueops.NativeActor{
		Host:      "codex",
		SessionID: "session-1",
		AgentID:   "agent-1",
		SessionProcess: &issueops.NativeProcessReceipt{
			PID: 42, StartedAt: "2026-07-22T00:00:00Z", Executable: "/usr/bin/codex",
		},
	}
	record := issueops.IssueOpsRecord{Execution: &issueops.Execution{
		Mode: issueops.ExecutionModeDirect,
		Workspace: issueops.Workspace{
			SourceRoot: source, Root: root, Branch: "69-redesign", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueops.WriteLease{Generation: 3, Status: issueops.LeaseStatusActive, Holder: &holder, ClaimedAt: "2026-07-22T00:00:01Z"},
	}}

	exact := IssueOpsActor{Host: "codex", SessionID: "session-1", AgentID: "agent-1", CWD: root, NativeProcessAncestry: []issueops.NativeProcessReceipt{*holder.SessionProcess}}
	if err := validateExecutionMutation(record, &exact); err != nil {
		t.Fatalf("exact current holder rejected: %v", err)
	}

	for name, actor := range map[string]*IssueOpsActor{
		"missing":            nil,
		"missing process":    {Host: "codex", SessionID: "session-1", AgentID: "agent-1", CWD: root},
		"reused process":     {Host: "codex", SessionID: "session-1", AgentID: "agent-1", CWD: root, NativeProcessAncestry: []issueops.NativeProcessReceipt{{PID: 42, StartedAt: "2026-07-22T00:00:01Z", Executable: "/usr/bin/codex"}}},
		"wrong host":         {Host: "claude", SessionID: "session-1", AgentID: "agent-1", CWD: root, NativeProcessAncestry: exact.NativeProcessAncestry},
		"wrong session":      {Host: "codex", SessionID: "session-2", AgentID: "agent-1", CWD: root, NativeProcessAncestry: exact.NativeProcessAncestry},
		"wrong agent":        {Host: "codex", SessionID: "session-1", AgentID: "agent-2", CWD: root, NativeProcessAncestry: exact.NativeProcessAncestry},
		"source checkout":    {Host: "codex", SessionID: "session-1", AgentID: "agent-1", CWD: source, NativeProcessAncestry: exact.NativeProcessAncestry},
		"other worktree cwd": {Host: "codex", SessionID: "session-1", AgentID: "agent-1", CWD: filepath.Join(source+".worktrees", "other"), NativeProcessAncestry: exact.NativeProcessAncestry},
	} {
		if err := validateExecutionMutation(record, actor); err == nil {
			t.Fatalf("%s actor unexpectedly authorized: %#v", name, actor)
		}
	}
}

func TestExecutionMutationAllowsPreExecutionPlanningButFencesNonActiveLease(t *testing.T) {
	if err := validateExecutionMutation(issueops.IssueOpsRecord{}, nil); err != nil {
		t.Fatalf("pre-execution planning rejected: %v", err)
	}

	record := issueops.IssueOpsRecord{Execution: &issueops.Execution{
		Mode: issueops.ExecutionModeDirect,
		Workspace: issueops.Workspace{
			SourceRoot: "/tmp/source", Root: "/tmp/source.worktrees/issue-69", Branch: "69-redesign", BaseHead: strings.Repeat("b", 40), Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased, ReleasedAt: "2026-07-22T00:00:01Z"},
	}}
	actor := IssueOpsActor{Host: "codex", SessionID: "session-1", CWD: record.Execution.Workspace.Root}
	if err := validateExecutionMutation(record, &actor); err == nil {
		t.Fatal("released lease unexpectedly authorized a mutation")
	}
}
