package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

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
			WorkerPTYID: "pty-1", WorkerMailboxHandle: "term-1", TaskID: "task-1", DispatchID: "dispatch-1",
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
