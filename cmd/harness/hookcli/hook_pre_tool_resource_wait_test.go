package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunHookPreToolUseClassifiesExactOwnedResourceWait(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "74-resource-wait-hook")
	actor := activateIssueOpsHookExecution(t, cycle.id)
	command := func(root string) string {
		return "./bin/agent-harness resource wait --workspace-root " + root +
			" --profile e2e --timeout 1m --interval 5s --progress jsonl --json"
	}
	payload := func(sessionID, commandText string) string {
		raw, err := json.Marshal(map[string]any{
			"cwd": cycle.path, "host": actor.Host, "session_id": sessionID, "agent_id": actor.AgentID,
			"tool_name": "Bash", "tool_input": map[string]any{"command": commandText},
		})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	evaluate := func(sessionID, commandText string) map[string]any {
		return runHookCapture(t, payload(sessionID, commandText), func() error {
			return runHookPreToolUse([]string{"--host", "codex", "--enforce-worktree", "--json"})
		})
	}

	if got := evaluate(actor.SessionID, command(cycle.path)); got["decision"] != "allow" {
		t.Fatalf("active hook adapter must allow exact canonical resource wait for the holder: %+v", got)
	}
	for name, root := range map[string]string{
		"source root":  source,
		"foreign root": t.TempDir(),
	} {
		t.Run(name, func(t *testing.T) {
			got := evaluate(actor.SessionID, command(root))
			assertIssueOpsDenyFields(t, got["deny"], cycle.id, cycle.path, 1, "write_lease_required")
		})
	}
	for name, commandText := range map[string]string{
		"other subcommand": "./bin/agent-harness resource inspect --workspace-root " + cycle.path,
		"shell wrapper":    "sh -c '" + command(cycle.path) + "'",
	} {
		t.Run(name, func(t *testing.T) {
			got := evaluate(actor.SessionID, commandText)
			assertIssueOpsDenyFields(t, got["deny"], cycle.id, cycle.path, 1, "unsafe_mutation")
		})
	}
	wrongIdentity := evaluate("wrong-session", command(cycle.path))
	assertIssueOpsDenyFields(t, wrongIdentity["deny"], cycle.id, cycle.path, 1, "write_lease_required")
}
