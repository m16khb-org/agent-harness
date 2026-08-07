package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/core"
)

func TestRunHookPreToolUseGeneratedProvenanceHostParity(t *testing.T) {
	for _, host := range []string{"codex", "claude"} {
		t.Run(host, func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			source := filepath.Join(t.TempDir(), "agent-harness")
			if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			cycle := createLinkedIssueOpsWorktree(t, source, "303-generated-provenance-"+host)
			actor := activateIssueOpsHookExecution(t, cycle.id)
			if host == "claude" {
				record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), cycle.id)
				if err != nil {
					t.Fatal(err)
				}
				actor.Host = host
				record.Execution.Lease.Holder.Host = host
				if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
					t.Fatal(err)
				}
			}
			binaryDir := filepath.Join(cycle.path, "bin")
			if err := os.MkdirAll(binaryDir, 0o755); err != nil {
				t.Fatal(err)
			}
			binary := filepath.Join(binaryDir, "agent-harness")
			if err := os.WriteFile(binary, []byte("current binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			binary, err := filepath.EvalSymlinks(binary)
			if err != nil {
				t.Fatal(err)
			}
			command := binary + " issueops execution resume --id " + cycle.id +
				" --expected-generation 1 --confirm" +
				" --generated-by-executable " + binary +
				" --generated-by-sha256 " + strings.Repeat("a", 64) +
				" --generated-for-generation 1 --json"
			toolName := "Bash"
			toolInput := map[string]any{"command": command}
			if host == "codex" {
				toolName = "exec_command"
				toolInput = map[string]any{"cmd": command, "workdir": cycle.path}
			}
			payload, err := json.Marshal(map[string]any{
				"cwd": cycle.path, "host": actor.Host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
				"tool_name": toolName, "tool_input": toolInput,
			})
			if err != nil {
				t.Fatal(err)
			}
			got := runHookCapture(t, string(payload), func() error {
				return runHookPreToolUse([]string{"--host", host, "--repo", cycle.path, "--enforce-worktree", "--json"})
			})
			if got["decision"] != "allow" {
				t.Fatalf("%s hook rejected shared provenance envelope: %+v", host, got)
			}
			installedDir := filepath.Join(source, "bin")
			if err := os.MkdirAll(installedDir, 0o755); err != nil {
				t.Fatal(err)
			}
			installed := filepath.Join(installedDir, "agent-harness")
			if err := os.WriteFile(installed, []byte("trusted installed binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			installed, err = filepath.EvalSymlinks(installed)
			if err != nil {
				t.Fatal(err)
			}
			installedCommand := strings.ReplaceAll(command, binary, installed)
			installedPayload := strings.Replace(string(payload), command, installedCommand, 1)
			installedAllowed := runHookCapture(t, installedPayload, func() error {
				return runHookPreToolUse([]string{"--host", host, "--repo", cycle.path, "--enforce-worktree", "--json"})
			})
			if installedAllowed["decision"] != "allow" {
				t.Fatalf("%s hook rejected durable source installation target: %+v", host, installedAllowed)
			}

			unsafePayload := strings.Replace(string(payload), binary, "$(command -v agent-harness)", 1)
			blocked := runHookCapture(t, unsafePayload, func() error {
				return runHookPreToolUse([]string{"--host", host, "--repo", cycle.path, "--enforce-worktree", "--json"})
			})
			if blocked["decision"] != "block" {
				t.Fatalf("%s hook accepted shell substitution in provenance: %+v", host, blocked)
			}

			outside := filepath.Join(t.TempDir(), "evil-harness")
			if err := os.WriteFile(outside, []byte("not the trusted binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			outsideCommand := strings.ReplaceAll(command, binary, outside)
			outsidePayload := strings.Replace(string(payload), command, outsideCommand, 1)
			outsideBlocked := runHookCapture(t, outsidePayload, func() error {
				return runHookPreToolUse([]string{"--host", host, "--repo", cycle.path, "--enforce-worktree", "--json"})
			})
			if outsideBlocked["decision"] != "block" {
				t.Fatalf("%s hook accepted self-declared executable outside trusted roots: %+v", host, outsideBlocked)
			}
		})
	}
}
