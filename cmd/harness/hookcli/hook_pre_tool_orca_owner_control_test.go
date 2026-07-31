package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunHookPreToolUseAllowsCurrentOrcaOwnerControlCommands(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "194-orca-owner-control")
	actor := activateIssueOpsHookExecution(t, cycle.id)

	for name, command := range map[string]string{
		"dispatch capability worker done": "orca orchestration send --from term_worker --dispatch-capability dcap_test --type worker_done --subject stopped --body safe-checkpoint --task-id task-1 --dispatch-id ctx-1 --outcome failed --files-modified '' --json",
		"generated GitHub branch reader":  "gh issue develop --list 194 --repo example/agent-harness",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{
				"cwd": cycle.path, "host": actor.Host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
				"tool_name": "Bash", "tool_input": map[string]any{"command": command},
			})
			if err != nil {
				t.Fatal(err)
			}
			got := runHookCapture(t, string(raw), func() error {
				return runHookPreToolUse([]string{"--host", actor.Host, "--repo", cycle.path, "--enforce-worktree", "--json"})
			})
			if got["decision"] != "allow" {
				t.Fatalf("현재 owner 제어 명령은 hook 입력 전체에서도 허용해야 한다: %+v", got)
			}
		})
	}
}
