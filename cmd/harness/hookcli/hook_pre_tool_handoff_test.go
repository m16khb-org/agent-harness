package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestRunHookPreToolUseSupervisedMultiCycleControlPlaneMatrix(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first := createLinkedIssueOpsWorktree(t, source, "47-first-cycle")
	second := createLinkedIssueOpsWorktree(t, source, "48-second-cycle")
	putCoordinatorPreparingHookRecord(t, source, first, "epoch-1", "wt-1")
	putCoordinatorPreparingHookRecord(t, source, second, "epoch-2", "wt-2")

	tests := []struct {
		name       string
		tool       string
		input      map[string]any
		want       string
		wantReason bool
	}{
		{name: "get goal", tool: "get_goal", input: map[string]any{}, want: "allow"},
		{name: "update goal", tool: "update_goal", input: map[string]any{"status": "complete"}, want: "allow"},
		{name: "update plan", tool: "update_plan", input: map[string]any{"plan": []any{}}, want: "allow"},
		{name: "request user input", tool: "request_user_input", input: map[string]any{"questions": []any{}}, want: "allow"},
		{name: "exact codegraph", tool: "exec_command", input: map[string]any{"command": "codegraph explore 'handoff selection'"}, want: "allow"},
		{name: "exact status", tool: "issueops_status", input: map[string]any{"id": first.id}, want: "allow"},
		{name: "exact resume", tool: "mcp__agent_harness__issueops_resume", input: map[string]any{"id": second.id, "repo": source, "bind": false}, want: "allow"},
		{name: "bounded hooks list", tool: "exec_command", input: map[string]any{"command": "agent-harness issueops handoff codex-hooks-list --id " + first.id + " --json"}, want: "allow"},
		{name: "control lookalike", tool: "functions__update_goal", input: map[string]any{}, want: "block", wantReason: true},
		{name: "foreign status", tool: "issueops_status", input: map[string]any{"id": "io-foreign-cycle"}, want: "block", wantReason: true},
		{name: "codegraph input redirect", tool: "exec_command", input: map[string]any{"command": "codegraph explore </tmp/input"}, want: "block", wantReason: true},
		{name: "direct app server", tool: "exec_command", input: map[string]any{"command": "codex -C " + first.path + " app-server --stdio"}, want: "block", wantReason: true},
		{name: "synthetic write stdin", tool: "write_stdin", input: map[string]any{"session_id": "unbound", "chars": `{"method":"hooks/list"}`}, want: "block", wantReason: true},
		{name: "synthetic config batch write", tool: "write_stdin", input: map[string]any{"session_id": "unbound", "chars": `{"method":"config/batchWrite"}`}, want: "block", wantReason: true},
		{name: "helper bypass flag", tool: "exec_command", input: map[string]any{"command": "agent-harness issueops handoff codex-hooks-list --id " + first.id + " --allow-codex-hook-trust-bypass --json"}, want: "block", wantReason: true},
		{name: "helper worker flag", tool: "exec_command", input: map[string]any{"command": "agent-harness issueops handoff codex-hooks-list --id " + first.id + " --worker " + first.path + " --json"}, want: "block", wantReason: true},
		{name: "helper method flag", tool: "exec_command", input: map[string]any{"command": "agent-harness issueops handoff codex-hooks-list --id " + first.id + " --method hooks/list --json"}, want: "block", wantReason: true},
		{name: "helper stdin flag", tool: "exec_command", input: map[string]any{"command": "agent-harness issueops handoff codex-hooks-list --id " + first.id + " --stdin payload --json"}, want: "block", wantReason: true},
		{name: "helper config flag", tool: "exec_command", input: map[string]any{"command": "agent-harness issueops handoff codex-hooks-list --id " + first.id + " --config payload --json"}, want: "block", wantReason: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]any{
				"cwd": source, "host": "codex", "session_id": "coordinator", "tool_name": tt.tool, "tool_input": tt.input,
			})
			if err != nil {
				t.Fatal(err)
			}
			got := runHookCapture(t, string(payload), func() error {
				return runHookPreToolUse([]string{"--enforce-worktree", "--expected-worktree", source, "--source-checkout", source, "--host", "codex", "--json"})
			})
			if got["decision"] != tt.want {
				t.Fatalf("tool=%s input=%#v got=%+v want=%s", tt.tool, tt.input, got, tt.want)
			}
			if tt.wantReason {
				reason, _ := got["reason"].(string)
				if strings.TrimSpace(reason) == "" {
					t.Fatalf("blocked row must retain an actionable reason: %+v", got)
				}
			}
		})
	}
}

func putCoordinatorPreparingHookRecord(t *testing.T, source string, cycle linkedIssueOpsWorktree, epoch, worktreeID string) {
	t.Helper()
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), cycle.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = core.IssueOpsPhaseImplement
	record.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{
		ProtocolVersion:          handoff.ProtocolVersion,
		State:                    handoff.StateCoordinatorPreparing,
		Attempt:                  1,
		OwnershipEpoch:           epoch,
		AttemptBaseHead:          strings.Repeat("a", 40),
		Driver:                   "orca",
		Agent:                    "codex",
		CoordinatorRoot:          source,
		WorkerRoot:               cycle.path,
		CoordinatorMailboxHandle: "term_coordinator",
		CoordinatorSession:       &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "coordinator"},
		Orca: &issueopsmodel.IssueOpsOrcaIdentity{
			RuntimeID: "runtime-" + worktreeID, RepoID: "repo-1", BaseRef: "refs/remotes/origin/" + record.Branch,
			WorktreeID: worktreeID, WorktreeInstanceID: "instance-" + worktreeID, WorktreePath: cycle.path,
		},
	}
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
}

func TestRunHookPreToolUseDefaultsHostlessClaimedSessionToCodex(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "16-hostless-codex")
	if _, err := core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), cycle.id, string(core.IssueOpsPhaseImplement)); err != nil {
		t.Fatal(err)
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), cycle.id)
	if err != nil {
		t.Fatal(err)
	}
	record.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{
		ProtocolVersion:     handoff.ProtocolVersion,
		Driver:              "orca",
		State:               handoff.StateClaimed,
		Attempt:             1,
		OwnershipEpoch:      "epoch-1",
		CoordinatorRoot:     source,
		WorkerRoot:          cycle.path,
		AttemptBaseHead:     "0000000000000000000000000000000000000000",
		ContextVersion:      handoff.ContextVersion,
		ContextSHA256:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ContextSourceSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ContextOptions:      &issueopsmodel.IssueOpsExecutionHandoffContextOptions{},
		DeliveryMode:        "inject",
		Orca: &issueopsmodel.IssueOpsOrcaIdentity{
			RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-hostless-codex",
			WorktreeID: "worktree-1", WorktreeInstanceID: "instance-1", WorktreePath: cycle.path,
			WorkerPTYID: "pty-1", WorkerTerminalHandle: "term-1", WorkerMailboxHandle: "term-1", TaskID: "task-1", DispatchID: "dispatch-1",
		},
		WorkerSession: &issueopsmodel.IssueOpsHostSessionIdentity{
			Host:      "codex",
			SessionID: "session-1",
		},
	}
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(t.TempDir(), "codex-session.jsonl")

	for _, tt := range []struct {
		name, session, payloadHost, flagHost, want string
	}{
		{name: "exact session", session: "session-1", want: "allow"},
		{name: "explicit payload codex", session: "session-1", payloadHost: "codex", want: "allow"},
		{name: "explicit cli codex", session: "session-1", flagHost: "codex", want: "allow"},
		{name: "matching explicit codex", session: "session-1", payloadHost: "codex", flagHost: "codex", want: "allow"},
		{name: "payload codex cli claude conflict", session: "session-1", payloadHost: "codex", flagHost: "claude", want: "block"},
		{name: "payload claude cli codex conflict", session: "session-1", payloadHost: "claude", flagHost: "codex", want: "block"},
		{name: "explicit payload claude", session: "session-1", payloadHost: "claude", want: "block"},
		{name: "explicit cli claude", session: "session-1", flagHost: "claude", want: "block"},
		{name: "empty session", session: "", want: "block"},
		{name: "wrong session", session: "session-2", want: "block"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]any{
				"cwd":             cycle.path,
				"session_id":      tt.session,
				"transcript_path": transcriptPath,
				"tool_name":       "apply_patch",
				"tool_input": map[string]any{
					"patch": "*** Begin Patch\n*** Add File: " + filepath.Join(cycle.path, "evidence.md") + "\n+evidence\n*** End Patch\n",
				},
			}
			if tt.payloadHost != "" {
				input["host"] = tt.payloadHost
			}
			payload, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{
				"--enforce-worktree",
				"--expected-worktree", cycle.path,
				"--source-checkout", source,
				"--json",
			}
			if tt.flagHost != "" {
				args = append(args, "--host", tt.flagHost)
			}
			got := runHookCapture(t, string(payload), func() error {
				return runHookPreToolUse(args)
			})
			if got["decision"] != tt.want {
				t.Fatalf("session %q: got %+v, want decision %q", tt.session, got, tt.want)
			}
		})
	}

	for _, tool := range []string{"Bash", "shell_command", "exec_command", "unified_exec"} {
		for _, tt := range []struct {
			name, command, want string
		}{
			{name: "mutation", command: "git add .", want: "block"},
			{name: "read only", command: "git status --short", want: "allow"},
		} {
			t.Run(tool+" "+tt.name, func(t *testing.T) {
				payload, err := json.Marshal(map[string]any{
					"cwd": cycle.path, "session_id": "wrong-session", "tool_name": tool,
					"tool_input": map[string]any{"command": tt.command},
				})
				if err != nil {
					t.Fatal(err)
				}
				got := runHookCapture(t, string(payload), func() error {
					return runHookPreToolUse([]string{"--enforce-worktree", "--expected-worktree", cycle.path, "--source-checkout", source, "--json"})
				})
				if got["decision"] != tt.want {
					t.Fatalf("tool=%s command=%q got=%+v want=%s", tool, tt.command, got, tt.want)
				}
			})
		}
	}
}
