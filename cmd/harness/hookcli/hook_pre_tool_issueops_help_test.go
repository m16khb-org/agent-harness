package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunHookPreToolUseAllowsRemoteCreatePRHelpDuringActiveLease(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "191-issueops-help-hook")
	actor := activateIssueOpsHookExecution(t, cycle.id)
	command := "agent-harness issueops remote create-pr --help"

	raw, err := json.Marshal(map[string]any{
		"cwd": cycle.path, "host": actor.Host, "session_id": "observer-session",
		"tool_name": "Bash", "tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := runHookCapture(t, string(raw), func() error {
		return runHookPreToolUse([]string{"--host", actor.Host, "--enforce-worktree", "--json"})
	})
	if got["decision"] != "allow" {
		t.Fatalf("IssueOps remote create-pr help-only 호출은 active lease 중에도 hook에서 허용해야 한다: %+v", got)
	}
}
