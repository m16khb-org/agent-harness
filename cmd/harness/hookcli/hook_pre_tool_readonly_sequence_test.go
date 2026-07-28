package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunHookPreToolUseAllowsBoundedReadOnlySequence(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "75-readonly-sequence-hook")
	_ = activateIssueOpsHookExecution(t, cycle.id)
	command := `if [ -d .codegraph ]; then printf 'codegraph-present\n'; else printf 'codegraph-absent\n'; fi; git status --short; git branch --show-current; git rev-parse HEAD; git diff --stat; git diff --cached --stat`
	raw, err := json.Marshal(map[string]any{
		"cwd": cycle.path, "host": "codex", "session_id": "observer-session",
		"tool_name": "Bash", "tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := runHookCapture(t, string(raw), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-worktree", "--json"})
	})
	if got["decision"] != "allow" {
		t.Fatalf("hook은 정적으로 판정 가능한 읽기 전용 시퀀스를 허용해야 한다: %+v", got)
	}
}
