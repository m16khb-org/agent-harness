package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunHookPreToolUseAllowsAtomicPreflightForCurrentHolder(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "191-atomic-preflight-hook")
	actor := activateIssueOpsHookExecution(t, cycle.id)
	command := "python3 skills/atomic-commit-push/scripts/git_preflight.py ."

	raw, err := json.Marshal(map[string]any{
		"cwd": cycle.path, "host": actor.Host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
		"tool_name": "Bash", "tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := runHookCapture(t, string(raw), func() error {
		return runHookPreToolUse([]string{"--host", actor.Host, "--enforce-worktree", "--json"})
	})
	if got["decision"] != "allow" {
		t.Fatalf("현재 holder의 atomic preflight는 hook 표면에서도 허용해야 한다: %+v", got)
	}
}

func TestRunHookPreToolUseAllowsAtomicStagedDiffReaderForCurrentHolder(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "191-atomic-staged-diff-hook")
	actor := activateIssueOpsHookExecution(t, cycle.id)
	command := `test -d .codegraph && echo present || echo absent
git diff --cached --stat
git diff --cached --name-only
git diff --cached --check`

	raw, err := json.Marshal(map[string]any{
		"cwd": cycle.path, "host": actor.Host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
		"tool_name": "Bash", "tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := runHookCapture(t, string(raw), func() error {
		return runHookPreToolUse([]string{"--host", actor.Host, "--enforce-worktree", "--json"})
	})
	if got["decision"] != "allow" {
		t.Fatalf("현재 holder의 고정 staged-diff reader는 hook 표면에서도 허용해야 한다: %+v", got)
	}
}

func TestRunHookPreToolUseUsesCodexExecCommandWorkdir(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "191-codex-atomic-preflight-hook")
	actor := activateIssueOpsHookExecution(t, cycle.id)
	command := "python3 skills/atomic-commit-push/scripts/git_preflight.py ."

	raw, err := json.Marshal(map[string]any{
		"cwd": source, "host": actor.Host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
		"tool_name": "exec_command", "tool_input": map[string]any{"cmd": command, "workdir": cycle.path},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := runHookCapture(t, string(raw), func() error {
		return runHookPreToolUse([]string{"--host", actor.Host, "--repo", cycle.path, "--enforce-worktree", "--json"})
	})
	if got["decision"] != "allow" {
		t.Fatalf("Codex exec_command의 canonical workdir preflight는 허용해야 한다: %+v", got)
	}

	raw, err = json.Marshal(map[string]any{
		"cwd": cycle.path, "host": actor.Host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
		"tool_name": "exec_command", "tool_input": map[string]any{"cmd": command, "workdir": source},
	})
	if err != nil {
		t.Fatal(err)
	}
	got = runHookCapture(t, string(raw), func() error {
		return runHookPreToolUse([]string{"--host", actor.Host, "--repo", cycle.path, "--enforce-worktree", "--json"})
	})
	if got["decision"] != "block" {
		t.Fatalf("Codex exec_command의 실제 workdir가 canonical root 밖이면 차단해야 한다: %+v", got)
	}
}

func TestRunHookPreToolUseRejectsAtomicScriptPathSpoof(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "191-atomic-script-spoof-hook")
	actor := activateIssueOpsHookExecution(t, cycle.id)

	for name, command := range map[string]string{
		"외부 skills 경로": "python3 /tmp/x/skills/atomic-commit-push/scripts/git_preflight.py .",
		"부모 상대 경로":     "python3 ../skills/atomic-commit-push/scripts/api_doc_gate.py .",
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
			if got["decision"] != "block" {
				t.Fatalf("정식 skill root 밖 atomic script 위장 경로는 차단해야 한다: %+v", got)
			}
		})
	}
}

func TestRunHookPreToolUseRejectsRelativeAtomicScriptFromSubdirectory(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "191-atomic-subdir-hook")
	actor := activateIssueOpsHookExecution(t, cycle.id)
	subdir := filepath.Join(cycle.path, "internal", "core")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	command := "python3 skills/atomic-commit-push/scripts/git_preflight.py ."

	raw, err := json.Marshal(map[string]any{
		"cwd": subdir, "host": actor.Host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
		"tool_name": "Bash", "tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := runHookCapture(t, string(raw), func() error {
		return runHookPreToolUse([]string{"--host", actor.Host, "--repo", cycle.path, "--enforce-worktree", "--json"})
	})
	if got["decision"] != "block" {
		t.Fatalf("canonical root 하위 cwd의 상대 atomic script 위장은 차단해야 한다: %+v", got)
	}
}

func TestRunHookPreToolUseRejectsAbsoluteAtomicScriptUnderGenericRepoSubdirectory(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "191-atomic-absolute-subdir-hook")
	actor := activateIssueOpsHookExecution(t, cycle.id)
	subdir := filepath.Join(cycle.path, "internal", "core")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(subdir, "skills", "atomic-commit-push", "scripts", "git_preflight.py")
	command := "python3 " + script + " ."

	raw, err := json.Marshal(map[string]any{
		"cwd": subdir, "host": actor.Host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
		"tool_name": "Bash", "tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := runHookCapture(t, string(raw), func() error {
		return runHookPreToolUse([]string{"--host", actor.Host, "--enforce-worktree", "--json"})
	})
	if got["decision"] != "block" {
		t.Fatalf("generic repo로 해석된 하위 cwd의 절대 atomic script 위장은 차단해야 한다: %+v", got)
	}
}
