package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
)

func TestCoordinatorChildHostSmokeAdmitsExactCommandWithoutExplicitSourceCheckout(t *testing.T) {
	useCanonicalSmokeTempDir(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, childRoot := executionActiveLifecycleRecord(t)
	record.Execution.Lease.Status = issueopscontract.LeaseStatusReleased
	record.Execution.Lease.Holder = nil
	record.Execution.Lease.ReleasedAt = "2026-08-02T00:00:00Z"
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	coordinator := linkIssueOpsWorktreeForGuardTest(t, source, "228-coordinator")
	coordinatorRoot := coordinator.path
	coordinatorRecord, err := ReadIssueOps(IssueOpsStateRoot(), coordinator.id)
	if err != nil {
		t.Fatal(err)
	}
	coordinatorRecord.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: source, Root: coordinatorRoot, Branch: coordinatorRecord.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-08-02T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusReleased,
			ReleasedAt: "2026-08-02T00:00:00Z",
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), coordinatorRecord); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(coordinatorRoot, "scripts", "verify-child-host-smoke.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sourceArg := canonicalSmokeTestPath(t, source)
	childArg := canonicalSmokeTestPath(t, childRoot)
	coordinatorArg := canonicalSmokeTestPath(t, coordinatorRoot)
	outputDirArg := canonicalSmokeTestPath(t, outputDir)

	command := fmt.Sprintf(
		"scripts/verify-child-host-smoke.sh --issue 69 --source-root %s --child-root %s --head 0123456789012345678901234567890123456789 --remote-ref refs/heads/69-v1-observation --json-out %s --confirm-user-activation",
		sourceArg,
		childArg,
		filepath.Join(outputDirArg, "receipt.json"),
	)
	req := lifecyclecontract.HookToolUseLifecycleRequest{
		Repo: sourceArg, CWD: sourceArg,
		Tool: "exec_command", Command: command,
		ToolInput:       map[string]any{"workdir": coordinatorArg},
		EnforceWorktree: true,
	}

	if !exactCoordinatorChildHostSmoke(req) {
		t.Fatalf("exact coordinator smoke command was not classified: %s", command)
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact coordinator smoke must bypass the released child mutation fence: %+v", got)
	}
}

func TestCoordinatorChildHostSmokeRejectsNearMisses(t *testing.T) {
	useCanonicalSmokeTempDir(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, _, childRoot := executionActiveLifecycleRecord(t)
	coordinator := linkIssueOpsWorktreeForGuardTest(t, source, "228-coordinator")
	coordinatorRoot := coordinator.path
	coordinatorRecord, err := ReadIssueOps(IssueOpsStateRoot(), coordinator.id)
	if err != nil {
		t.Fatal(err)
	}
	coordinatorRecord.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: source, Root: coordinatorRoot, Branch: coordinatorRecord.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-08-02T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusReleased,
			ReleasedAt: "2026-08-02T00:00:00Z",
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), coordinatorRecord); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(coordinatorRoot, "scripts", "verify-child-host-smoke.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sourceArg := canonicalSmokeTestPath(t, source)
	childArg := canonicalSmokeTestPath(t, childRoot)
	coordinatorArg := canonicalSmokeTestPath(t, coordinatorRoot)
	output := filepath.Join(canonicalSmokeTestPath(t, outputDir), "receipt.json")
	exact := fmt.Sprintf(
		"scripts/verify-child-host-smoke.sh --issue 69 --source-root %s --child-root %s --head 0123456789012345678901234567890123456789 --remote-ref refs/heads/69-v1-observation --json-out %s --confirm-user-activation",
		sourceArg,
		childArg,
		output,
	)
	base := lifecyclecontract.HookToolUseLifecycleRequest{
		Repo: sourceArg, CWD: sourceArg,
		Tool: "exec_command", ToolInput: map[string]any{"workdir": coordinatorArg},
		EnforceWorktree: true,
	}

	for name, command := range map[string]string{
		"wrapper":              "bash " + exact,
		"control operator":     exact + " && true",
		"missing confirmation": strings.Replace(exact, " --confirm-user-activation", "", 1),
		"extra flag":           exact + " --force",
		"duplicate issue":      strings.Replace(exact, "--issue 69", "--issue 69 --issue 69", 1),
		"relative child root":  strings.Replace(exact, childArg, "69-v1-observation", 1),
		"bad head":             strings.Replace(exact, "0123456789012345678901234567890123456789", "deadbeef", 1),
		"uppercase head":       strings.Replace(exact, "0123456789012345678901234567890123456789", "ABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCD", 1),
		"branch mismatch":      strings.Replace(exact, "refs/heads/69-v1-observation", "refs/heads/69-other", 1),
		"substitution":         strings.Replace(exact, "--issue 69", "--issue $(printf 69)", 1),
	} {
		t.Run(name, func(t *testing.T) {
			req := base
			req.Command = command
			if exactCoordinatorChildHostSmoke(req) {
				t.Fatalf("near miss was classified: %s", command)
			}
			got := BuildLifecyclePreToolUseDecision(req)
			if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "unsafe_mutation" {
				t.Fatalf("near miss must remain fail-closed: %+v", got)
			}
		})
	}

	t.Run("untrusted workdir", func(t *testing.T) {
		req := base
		req.Command = exact
		other := t.TempDir()
		otherScript := filepath.Join(other, "scripts", "verify-child-host-smoke.sh")
		if err := os.MkdirAll(filepath.Dir(otherScript), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(otherScript, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		req.ToolInput = map[string]any{"workdir": other}
		if exactCoordinatorChildHostSmoke(req) {
			t.Fatal("arbitrary workdir was classified as coordinator-owned")
		}
	})

	t.Run("public output directory", func(t *testing.T) {
		publicDir := t.TempDir()
		if err := os.Chmod(publicDir, 0o755); err != nil {
			t.Fatal(err)
		}
		req := base
		req.Command = strings.Replace(exact, output, filepath.Join(canonicalSmokeTestPath(t, publicDir), "receipt.json"), 1)
		if exactCoordinatorChildHostSmoke(req) {
			t.Fatal("non-private receipt directory was classified")
		}
	})

	t.Run("explicit source checkout mismatch", func(t *testing.T) {
		req := base
		req.Command = exact
		req.SourceCheckout = childArg
		if exactCoordinatorChildHostSmoke(req) {
			t.Fatal("explicit source checkout mismatch was classified")
		}
	})

	t.Run("repo source mismatch", func(t *testing.T) {
		req := base
		req.Command = exact
		req.Repo = childArg
		if exactCoordinatorChildHostSmoke(req) {
			t.Fatal("repo source mismatch was classified without an explicit source checkout")
		}
	})
}

func canonicalSmokeTestPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func useCanonicalSmokeTempDir(t *testing.T) {
	t.Helper()
	temporaryRoot := os.TempDir()
	resolved, err := filepath.EvalSymlinks(temporaryRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", resolved)
}
