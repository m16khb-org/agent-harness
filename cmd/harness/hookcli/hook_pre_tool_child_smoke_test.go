package hookcli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"

	"agent-harness/internal/adapter/core"
)

func TestRunHookPreToolUseAdmitsChildSmokeFromInstalledHookArguments(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	temporaryRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(temporaryRoot, "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	child := createLinkedIssueOpsWorktree(t, source, "69-child-smoke")
	coordinator := createLinkedIssueOpsWorktree(t, source, "228-coordinator")
	setReleasedHookExecution(t, child, source)
	setReleasedHookExecution(t, coordinator, source)
	childRecord, err := core.ReadIssueOps(core.IssueOpsStateRoot(), child.id)
	if err != nil {
		t.Fatal(err)
	}
	childRecord.IssueURL = "https://github.com/acme/repo/issues/69"
	childRecord.Delegation = &issueopscontract.IssueOpsDelegationContract{
		ParentCycleID: coordinator.id, TaskScope: "exact child smoke", DelegatedAt: "2026-08-02T00:00:00Z",
	}
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), childRecord); err != nil {
		t.Fatal(err)
	}
	coordinatorRecord, err := core.ReadIssueOps(core.IssueOpsStateRoot(), coordinator.id)
	if err != nil {
		t.Fatal(err)
	}
	coordinatorRecord.ChildCycles = []issueopscontract.IssueOpsChildCycleRef{{
		CycleID: child.id, Branch: childRecord.Branch, ChildIssueURL: childRecord.IssueURL,
	}}
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), coordinatorRecord); err != nil {
		t.Fatal(err)
	}

	script := filepath.Join(coordinator.path, "scripts", "verify-child-host-smoke.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	outputDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(outputDir, 0o700); err != nil {
		t.Fatal(err)
	}
	command := fmt.Sprintf(
		"%s --issue 69 --source-root %s --child-root %s --head 0123456789012345678901234567890123456789 --remote-ref refs/heads/69-child-smoke --json-out %s --confirm-user-activation",
		script,
		source,
		child.path,
		filepath.Join(outputDir, "receipt.json"),
	)
	payload, err := json.Marshal(map[string]any{
		"cwd": source, "tool_name": "exec_command",
		"tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{
			"--host", "codex", "--enforce-worktree", "--json",
		})
	})
	if got["decision"] != "allow" {
		t.Fatalf("installed hook arguments must admit the exact coordinator child smoke: %+v", got)
	}
	unsafeCommand := strings.Replace(command, script, filepath.Join(source, "scripts", "verify-child-host-smoke.sh"), 1)
	unsafePayload, err := json.Marshal(map[string]any{
		"cwd": source, "tool_name": "exec_command",
		"tool_input": map[string]any{"command": unsafeCommand},
	})
	if err != nil {
		t.Fatal(err)
	}
	blocked := runHookCapture(t, string(unsafePayload), func() error {
		return runHookPreToolUse([]string{
			"--host", "codex", "--enforce-worktree", "--json",
		})
	})
	if blocked["decision"] != "block" {
		t.Fatalf("source script must not be admitted from coordinator script authority: %+v", blocked)
	}
}

func setReleasedHookExecution(t *testing.T, linked linkedIssueOpsWorktree, source string) {
	t.Helper()
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), linked.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: source, Root: linked.path, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-08-02T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusReleased,
			ReleasedAt: "2026-08-02T00:00:00Z",
		},
	}
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
}
