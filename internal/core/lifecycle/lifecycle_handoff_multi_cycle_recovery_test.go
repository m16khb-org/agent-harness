package lifecycle

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
)

func TestHandoffMultiCycleAllowsOrdinarySourceWriters(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	addSecondLifecycleHandoffRecord(t, repo, first)

	tests := []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "apply_patch", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "go test ./...", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "orca terminal send --terminal term-unknown --text guidance --enter --json", EnforceWorktree: true, SourceCheckout: repo},
	}
	for _, req := range tests {
		got := BuildLifecyclePreToolUseDecision(req)
		if got.Decision != "allow" {
			t.Fatalf("ordinary source work must remain available: request=%#v result=%#v", req, got)
		}
	}
}

func TestHandoffMultiCycleAllowsExactLifecycleAndReadOnlyObservations(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	second := addSecondLifecycleHandoffRecord(t, repo, first)

	tests := []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "bin/agent-harness issueops status --id " + first.ID + " --json", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "bin/agent-harness issueops resume --repo " + repo + " --id " + second.ID + " --json", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "pwd", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "rg -n handoff internal", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "git status --short", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "mcp__filesystem__read_text_file", Paths: []string{repo + "/AGENTS.md"}, EnforceWorktree: true, SourceCheckout: repo},
	}
	for _, req := range tests {
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact lifecycle and proven read-only observations must not deadlock: request=%#v result=%#v", req, got)
		}
	}
}

func TestHandoffMultiCycleKeepsMalformedAndMutatingRequestsBlocked(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	addSecondLifecycleHandoffRecord(t, repo, first)

	tests := []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "bin/agent-harness issueops status --id " + first.ID + "; pwd", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "bin/agent-harness issueops status --id " + first.ID + " --unknown value", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "go test ./...", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "apply_patch", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "orca terminal send --terminal term-unknown --text guidance --enter --json", EnforceWorktree: true, SourceCheckout: repo},
	}
	for index, req := range tests {
		want := "allow"
		if index < 2 {
			want = "block"
		}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != want {
			t.Fatalf("multi-cycle request=%#v result=%#v want=%s", req, got, want)
		}
	}
}

func TestHandoffMultiCycleAllowsExactHostControlPlaneTools(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	addSecondLifecycleHandoffRecord(t, repo, first)

	for _, tool := range []string{"get_goal", "update_goal", "update_plan", "request_user_input"} {
		req := HookToolUseLifecycleRequest{Repo: repo, CWD: repo, Tool: tool, EnforceWorktree: true, SourceCheckout: repo}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact host control-plane tool %q must not deadlock: %#v", tool, got)
		}
	}
}

func TestHandoffMultiCycleAllowsExactCodeGraphExplore(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	addSecondLifecycleHandoffRecord(t, repo, first)
	req := HookToolUseLifecycleRequest{
		Repo: repo, CWD: repo, Tool: "exec_command", Command: "codegraph explore 'lifecycle handoff ownership'",
		EnforceWorktree: true, SourceCheckout: repo,
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact CodeGraph observation must not deadlock: %#v", got)
	}
}

func TestHandoffMultiCycleSelectsExactIssueOpsStatusResumeMCP(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	second := addSecondLifecycleHandoffRecord(t, repo, first)

	tests := []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "issueops_status", ToolInput: map[string]any{"id": first.ID}, EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "mcp__agent_harness__issueops_status", ToolInput: map[string]any{"id": second.ID}, EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "issueops_resume", ToolInput: map[string]any{"id": second.ID, "repo": repo}, EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "mcp__agent_harness__issueops_resume", ToolInput: map[string]any{"id": first.ID, "repo": repo, "bind": false}, EnforceWorktree: true, SourceCheckout: repo},
	}
	for _, req := range tests {
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact IssueOps observation/control must select its record: request=%#v result=%#v", req, got)
		}
	}
}

func TestHandoffMultiCycleRejectsInexactIssueOpsStatusResumeMCP(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	addSecondLifecycleHandoffRecord(t, repo, first)

	tests := []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "mcp__other__issueops_status", ToolInput: map[string]any{"id": first.ID}},
		{Repo: repo, CWD: repo, Tool: "issueops_status_extra", ToolInput: map[string]any{"id": first.ID}},
		{Repo: repo, CWD: repo, Tool: "issueops_status", ToolInput: map[string]any{}},
		{Repo: repo, CWD: repo, Tool: "issueops_status", ToolInput: map[string]any{"id": ""}},
		{Repo: repo, CWD: repo, Tool: "issueops_status", ToolInput: map[string]any{"id": "io-foreign-cycle"}},
		{Repo: repo, CWD: repo, Tool: "issueops_status", ToolInput: map[string]any{"flags": map[string]any{"id": first.ID}}},
		{Repo: repo, CWD: repo, Tool: "issueops_status", ToolInput: map[string]any{"id": first.ID, "unknown": true}},
		{Repo: repo, CWD: repo, Tool: "issueops_resume", ToolInput: map[string]any{"id": first.ID, "repo": repo + "-other"}},
		{Repo: repo, CWD: repo, Tool: "issueops_resume", ToolInput: map[string]any{"id": first.ID, "bind": true}},
		{Repo: repo, CWD: repo, Tool: "issueops_resume", ToolInput: map[string]any{"id": first.ID, "bind": "false"}},
		{Repo: repo, CWD: repo, Tool: "issueops_resume", ToolInput: map[string]any{"id": first.ID, "unknown": false}},
	}
	for _, req := range tests {
		req.EnforceWorktree = true
		req.SourceCheckout = repo
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("inexact IssueOps observation/control must remain blocked: request=%#v result=%#v", req, got)
		}
	}
}

func TestHandoffMultiCycleAllowsExactCoordinatorPreparingCodexTrustReview(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	second := addSecondLifecycleHandoffRecord(t, repo, first)
	for _, id := range []string{first.ID, second.ID} {
		req := HookToolUseLifecycleRequest{
			Repo: repo, CWD: repo, Host: "codex", Tool: "exec_command",
			Command:         "agent-harness issueops handoff codex-hooks-list --id " + id + " --json",
			EnforceWorktree: true, SourceCheckout: repo,
		}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact bounded Codex trust review for %s must select its record: %#v", id, got)
		}
	}
}

func TestHandoffMultiCycleRejectsInexactControlPlaneAndTrustReview(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	addSecondLifecycleHandoffRecord(t, repo, first)

	tests := []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "functions__update_goal"},
		{Repo: repo, CWD: repo, Tool: "UpdateGoal"},
		{Repo: repo, CWD: repo, Tool: "update_goal_extra"},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "codegraph explore --path /tmp query"},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "codegraph explore one two"},
		{Repo: repo, CWD: repo, Host: "codex", Tool: "exec_command", Command: "agent-harness issueops handoff codex-hooks-list --json"},
		{Repo: repo, CWD: repo, Host: "codex", Tool: "exec_command", Command: "agent-harness issueops handoff codex-hooks-list --id io-foreign-cycle --json"},
		{Repo: repo, CWD: repo, Host: "codex", Tool: "exec_command", Command: "agent-harness issueops handoff codex-hooks-list --id " + first.ID + " --worker /tmp/other --json"},
		{Repo: repo, CWD: repo, Host: "codex", Tool: "exec_command", Command: "codex -C " + first.WorktreePath + " app-server --stdio"},
		{Repo: repo, CWD: repo, Host: "codex", Tool: "write_stdin", ToolInput: map[string]any{"session_id": "unbound", "chars": `{"method":"hooks/list"}`}},
	}
	for index, req := range tests {
		req.EnforceWorktree = true
		req.SourceCheckout = repo
		want := "allow"
		if index >= 5 && index <= 8 {
			want = "block"
		}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != want {
			t.Fatalf("control-plane/trust request=%#v result=%#v want=%s", req, got, want)
		}
	}
}

func TestHandoffForeignObservationIDHasBoundedNonReflectiveDenial(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	addSecondLifecycleHandoffRecord(t, repo, first)
	secret := "token=foreign-review-secret-value"
	req := HookToolUseLifecycleRequest{
		Repo: repo, CWD: repo, Host: "codex", Tool: "exec_command",
		Command:         "agent-harness issueops handoff codex-hooks-list --id " + secret + " --json",
		EnforceWorktree: true, SourceCheckout: repo,
	}
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || !strings.Contains(got.Reason, "does not match a supervised cycle") {
		t.Fatalf("foreign helper id must receive a dedicated denial: %#v", got)
	}
	if strings.Contains(got.Reason, "foreign-review-secret-value") || len(got.Reason) > 512 {
		t.Fatalf("foreign helper denial reflected untrusted id: %q", got.Reason)
	}
	for _, missing := range []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Host: "codex", Tool: "exec_command", Command: "agent-harness issueops handoff codex-hooks-list --json", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Host: "codex", Tool: "issueops_status", ToolInput: map[string]any{}, EnforceWorktree: true, SourceCheckout: repo},
	} {
		decision := BuildLifecyclePreToolUseDecision(missing)
		if decision.Decision != "block" {
			t.Fatalf("missing observation id must remain denied: request=%#v result=%#v", missing, decision)
		}
	}
}

func TestHandoffRealForeignSourceIDCannotEscapeRecordTargetedGuard(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	addSecondLifecycleHandoffRecord(t, repo, first)
	foreignRepo := guardRepoWithCycle(t, "99-foreign-source", IssueOpsPhaseImplement)
	foreign, ok := ActiveIssueOpsCycleForBranch(foreignRepo, "99-foreign-source")
	if !ok {
		t.Fatal("foreign source cycle missing")
	}
	if _, err := ReadIssueOps(IssueOpsStateRoot(), foreign.ID); err != nil {
		t.Fatalf("foreign lifecycle id is not a real durable record: %v", err)
	}
	req := HookToolUseLifecycleRequest{
		Repo: repo, CWD: repo, Host: "codex", Tool: "exec_command",
		Command:         "agent-harness issueops handoff codex-hooks-list --id " + foreign.ID + " --json",
		EnforceWorktree: true, SourceCheckout: repo,
	}
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || !strings.Contains(got.Reason, "does not match a supervised cycle for this source checkout") {
		t.Fatalf("real foreign-source id escaped record-targeted selection: %#v", got)
	}
}

func TestHandoffCodexHooksListRequiresValidSourceCoordinatorPreparingRecord(t *testing.T) {
	t.Run("valid source coordinator preparing", func(t *testing.T) {
		repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
		req := HookToolUseLifecycleRequest{Repo: repo, CWD: repo, Host: "codex", Tool: "exec_command", Command: "agent-harness issueops handoff codex-hooks-list --id " + record.ID + " --json", EnforceWorktree: true, SourceCheckout: repo}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("valid source coordinator_preparing helper must be allowed: %#v", got)
		}
	})

	for _, tt := range []struct {
		name  string
		state string
		cwd   string
		host  string
		id    string
	}{
		{name: "claimed state", state: handoff.StateClaimed, host: "codex"},
		{name: "claimed worker cwd", state: handoff.StateCoordinatorPreparing, cwd: "worker", host: "codex"},
		{name: "wrong host", state: handoff.StateCoordinatorPreparing, host: "claude"},
		{name: "foreign id", state: handoff.StateCoordinatorPreparing, host: "codex", id: "io-foreign-cycle"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo, record, worker := lifecycleHandoffRecord(t, tt.state)
			cwd := repo
			if tt.cwd == "worker" {
				cwd = worker
			}
			id := record.ID
			if tt.id != "" {
				id = tt.id
			}
			req := HookToolUseLifecycleRequest{Repo: cwd, CWD: cwd, Host: tt.host, Tool: "exec_command", Command: "agent-harness issueops handoff codex-hooks-list --id " + id + " --json", EnforceWorktree: true, SourceCheckout: repo}
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("invalid helper context must remain blocked: request=%#v result=%#v", req, got)
			}
		})
	}

	t.Run("invalid envelope", func(t *testing.T) {
		repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
		record.ExecutionHandoff.ProtocolVersion++
		putRawLifecycleIssueOpsRecord(t, record)
		req := HookToolUseLifecycleRequest{Repo: repo, CWD: repo, Host: "codex", Tool: "exec_command", Command: "agent-harness issueops handoff codex-hooks-list --id " + record.ID + " --json", EnforceWorktree: true, SourceCheckout: repo}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("invalid helper envelope must remain blocked: %#v", got)
		}
	})
}

func addSecondLifecycleHandoffRecord(t *testing.T, repo string, first IssueOpsRecord) IssueOpsRecord {
	t.Helper()
	second := first
	second.ID = "io-second-cycle"
	second.Branch = "2-demo"
	second.WorktreePath = makeIssueOpsGuardWorktreeForTest(t, repo, second.Branch)
	second.ExecutionHandoff = cloneLifecycleHandoffForTest(t, first.ExecutionHandoff)
	second.ExecutionHandoff.WorkerRoot = second.WorktreePath
	second.ExecutionHandoff.OwnershipEpoch = "epoch-2"
	second.ExecutionHandoff.Orca.WorktreeID = "wt-2"
	second.ExecutionHandoff.Orca.WorktreePath = second.WorktreePath
	if second.ExecutionHandoff.Orca.TaskID != "" {
		second.ExecutionHandoff.Orca.TaskID = "task-2"
		second.ExecutionHandoff.Orca.DispatchID = "dispatch-2"
		second.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-2"
		second.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-2"
	}
	written, err := writeIssueOps(IssueOpsStateRoot(), second)
	if err != nil {
		t.Fatal(err)
	}
	return written
}
