package lifecycle

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestHandoffGuardBlocksBeforeClaim(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go")))
	if got.Decision != "block" || !strings.Contains(got.Reason, "claim") {
		t.Fatalf("pre-claim mutation should block: %#v", got)
	}
}

func TestHandoffGuardAllowsMatchingClaimedWorkerInTree(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go")))
	if got.Decision != "allow" {
		t.Fatalf("claimed worker in tree should pass: %#v", got)
	}
}

func TestHandoffGuardBlocksCoordinatorAbsolutePathIntoWorkerTree(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, repo, "codex", "coordinator", filepath.Join(worktree, "internal", "x.go")))
	if got.Decision != "block" {
		t.Fatalf("coordinator absolute-path edit should block: %#v", got)
	}
}

func TestHandoffGuardBlocksWrongOrRestartedSession(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	for _, session := range []string{"other", ""} {
		got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", session, filepath.Join(worktree, "internal", "x.go")))
		if got.Decision != "block" {
			t.Fatalf("session %q should block: %#v", session, got)
		}
	}
}

func TestHandoffGuardBlocksWorkerEscape(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(repo, "internal", "x.go")))
	if got.Decision != "block" || !strings.Contains(got.Reason, "outside") {
		t.Fatalf("worker escape should block: %#v", got)
	}
}

func TestHandoffGuardAllowsExactLifecycleCommandsOnly(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	claim := handoffEditRequest(record, worktree, "codex", "session-1", "")
	claim.Tool = "Bash"
	claim.Command = handoffClaimCommand(record, worktree, "codex", "session-1", "worker-1", "wt-1")
	if got := BuildLifecyclePreToolUseDecision(claim); got.Decision != "allow" {
		t.Fatalf("exact claim command should pass: %#v", got)
	}
	claim.Command += " && touch internal/x.go"
	if got := BuildLifecyclePreToolUseDecision(claim); got.Decision != "block" {
		t.Fatalf("compound lifecycle command should block: %#v", got)
	}
}

func TestHandoffGuardAllowsQuotedFinishEvidenceAndBlocksUnquotedControlOperators(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	base := handoffEditRequest(record, worktree, "codex", "session-1", "")
	base.Tool = "Bash"
	base.Command = "agent-harness issueops handoff finish --id " + record.ID +
		" --verification 'commit parent is exact; tree clean & verified | complete'" +
		` --cleanup-receipt "no temp; coordinator owns task & worktree | branch"`
	if got := BuildLifecyclePreToolUseDecision(base); got.Decision != "allow" {
		t.Fatalf("quoted evidence punctuation must remain argument data: %#v", got)
	}

	for _, suffix := range []string{
		"; touch escaped.go",
		" & touch escaped.go",
		" | touch escaped.go",
		"\ntouch escaped.go",
		"\rtouch escaped.go",
	} {
		t.Run(strconv.Quote(suffix), func(t *testing.T) {
			req := base
			req.Command += suffix
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("unquoted shell control %q must fail closed: %#v", suffix, got)
			}
		})
	}
}

func TestHandoffGuardAuthenticatesClaimFlagsAgainstNativeIdentity(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	tests := []struct {
		name, host, session, agent, cwd, worktreeID string
		allow                                       bool
	}{
		{name: "exact", host: "codex", session: "session-1", agent: "worker-1", cwd: worktree, worktreeID: "wt-1", allow: true},
		{name: "session mismatch", host: "codex", session: "other", agent: "worker-1", cwd: worktree, worktreeID: "wt-1"},
		{name: "host mismatch", host: "claude", session: "session-1", agent: "worker-1", cwd: worktree, worktreeID: "wt-1"},
		{name: "agent mismatch", host: "codex", session: "session-1", agent: "other", cwd: worktree, worktreeID: "wt-1"},
		{name: "cwd mismatch", host: "codex", session: "session-1", agent: "worker-1", cwd: filepath.Join(worktree, "nested"), worktreeID: "wt-1"},
		{name: "worktree id mismatch", host: "codex", session: "session-1", agent: "worker-1", cwd: worktree, worktreeID: "other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := handoffEditRequest(record, worktree, "codex", "session-1", "")
			req.Tool = "Bash"
			req.Command = handoffClaimCommand(record, tt.cwd, tt.host, tt.session, tt.agent, tt.worktreeID)
			got := BuildLifecyclePreToolUseDecision(req)
			if tt.allow && got.Decision != "allow" {
				t.Fatalf("exact native identity should pass: %#v", got)
			}
			if !tt.allow && got.Decision != "block" {
				t.Fatalf("self-asserted claim identity should block: %#v", got)
			}
		})
	}
}

func TestHandoffGuardAllowsClaimWithoutAgentFlagWhenNativeAgentIsEmpty(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	req := handoffEditRequest(record, worktree, "codex", "session-1", "")
	req.AgentID = ""
	req.Tool = "Bash"
	req.Command = handoffClaimCommand(record, worktree, "codex", "session-1", "", "wt-1")
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("claim with absent native agent identity should pass without --agent-id: %#v", got)
	}
}

func TestUniqueFlagValueRejectsFollowingFlagAsValue(t *testing.T) {
	if value, ok := uniqueFlagValue([]string{"--agent-id", "--cwd", "/worker"}, "--agent-id"); ok {
		t.Fatalf("flag token must not become another flag's value: %q", value)
	}
}

func TestSessionStartRendersClaimWithoutMutation(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	before, _ := json.Marshal(record)
	guidance := BuildIssueOpsHandoffSessionGuidance(worktree, "codex", "session-1", "worker-1")
	for _, want := range []string{"role=worker", "handoff claim", "--id " + record.ID, "--attempt 1", "--ownership-epoch epoch-1", "--context-sha256 " + strings.Repeat("a", 64), "--session-id session-1"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
	afterRecord, err := ReadIssueOps(IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(afterRecord)
	if string(before) != string(after) {
		t.Fatal("SessionStart guidance mutated IssueOps state")
	}
}

func TestSessionStartOmitsEmptyAgentIDFromClaim(t *testing.T) {
	_, _, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	guidance := BuildIssueOpsHandoffSessionGuidance(worktree, "codex", "session-1", "")
	if strings.Contains(guidance, "--agent-id") {
		t.Fatalf("empty native agent id must be omitted so the next flag cannot be consumed: %s", guidance)
	}
	for _, want := range []string{"--session-id session-1", "--cwd " + worktree, "--orca-worktree-id wt-1"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
}

func lifecycleHandoffRecord(t *testing.T, state string) (string, IssueOpsRecord, string) {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-demo", IssueOpsPhaseImplement)
	record, ok := ActiveIssueOpsCycleForBranch(repo, "1-demo")
	if !ok {
		t.Fatal("active record missing")
	}
	worktree := makeIssueOpsGuardWorktreeForTest(t, repo, "1-demo")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-demo")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	record, err := ReadIssueOps(IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{
		ProtocolVersion: handoff.ProtocolVersion, State: state, Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64),
		CoordinatorRoot: repo, WorkerRoot: worktree, Orca: &issueopsmodel.IssueOpsOrcaIdentity{WorktreeID: "wt-1", WorktreePath: worktree},
	}
	if state == handoff.StateClaimed {
		record.ExecutionHandoff.WorkerSession = &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "worker-1"}
	}
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return repo, record, worktree
}

func handoffEditRequest(record IssueOpsRecord, cwd, host, session, target string) HookToolUseLifecycleRequest {
	paths := []string(nil)
	if target != "" {
		paths = []string{target}
	}
	return HookToolUseLifecycleRequest{
		Repo: cwd, CWD: cwd, Host: host, SessionID: session, AgentID: "worker-1", Tool: "Edit", Paths: paths,
		EnforceWorktree: true, ExpectedWorktree: record.WorktreePath, SourceCheckout: record.Repo,
	}
}

func handoffClaimCommand(record IssueOpsRecord, cwd, host, session, agent, worktreeID string) string {
	command := "agent-harness issueops handoff claim --id " + record.ID +
		" --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64) +
		" --host " + host + " --session-id " + session
	if agent != "" {
		command += " --agent-id " + agent
	}
	return command + " --cwd " + cwd + " --orca-worktree-id " + worktreeID
}
