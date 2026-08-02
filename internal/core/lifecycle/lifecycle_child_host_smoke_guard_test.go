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
	activeChildLease := record.Execution.Lease
	record.Execution.Lease.Status = issueopscontract.LeaseStatusReleased
	record.Execution.Lease.Holder = nil
	record.Execution.Lease.ReleasedAt = "2026-08-02T00:00:00Z"

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
	record.IssueURL = "https://github.com/acme/repo/issues/69"
	record.Delegation = &issueopscontract.IssueOpsDelegationContract{
		ParentCycleID: coordinator.id, TaskScope: "exact child smoke", DelegatedAt: "2026-08-02T00:00:00Z",
	}
	coordinatorRecord.ChildCycles = []issueopscontract.IssueOpsChildCycleRef{{
		CycleID: record.ID, Branch: record.Branch, ChildIssueURL: record.IssueURL,
	}}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
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
		CWD:  sourceArg,
		Tool: "exec_command", Command: command,
		ToolInput:       map[string]any{"command": command},
		EnforceWorktree: true,
	}

	if !exactCoordinatorChildHostSmoke(req) {
		t.Fatalf("exact coordinator smoke command was not classified: %s", command)
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact coordinator smoke must bypass the released child mutation fence: %+v", got)
	}
	sourceRepo := req
	sourceRepo.Repo = sourceArg
	if !exactCoordinatorChildHostSmoke(sourceRepo) {
		t.Fatal("source repo fallback must remain valid for installed hook payloads")
	}
	coordinatorRepo := req
	coordinatorRepo.Repo = coordinatorArg
	coordinatorRepo.CWD = coordinatorArg
	if !exactCoordinatorChildHostSmoke(coordinatorRepo) {
		t.Fatal("coordinator repo fallback must remain valid when a host provides it")
	}
	explicitSource := req
	explicitSource.SourceCheckout = sourceArg
	if !exactCoordinatorChildHostSmoke(explicitSource) {
		t.Fatal("explicit source checkout must remain the highest-priority authority")
	}
	activeChild := record
	activeChild.Execution.Lease = activeChildLease
	if _, err := writeIssueOps(IssueOpsStateRoot(), activeChild); err != nil {
		t.Fatal(err)
	}
	if exactCoordinatorChildHostSmoke(req) {
		t.Fatal("active child lease must keep coordinator smoke fail-closed")
	}
}

func TestCoordinatorChildHostSmokeRejectsNearMisses(t *testing.T) {
	useCanonicalSmokeTempDir(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, childRecord, childRoot := executionActiveLifecycleRecord(t)
	childRecord.Execution.Lease.Status = issueopscontract.LeaseStatusReleased
	childRecord.Execution.Lease.Holder = nil
	childRecord.Execution.Lease.ReleasedAt = "2026-08-02T00:00:00Z"
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
	childRecord.IssueURL = "https://github.com/acme/repo/issues/69"
	childRecord.Delegation = &issueopscontract.IssueOpsDelegationContract{
		ParentCycleID: coordinator.id, TaskScope: "exact child smoke", DelegatedAt: "2026-08-02T00:00:00Z",
	}
	coordinatorRecord.ChildCycles = []issueopscontract.IssueOpsChildCycleRef{{
		CycleID: childRecord.ID, Branch: childRecord.Branch, ChildIssueURL: childRecord.IssueURL,
	}}
	if _, err := writeIssueOps(IssueOpsStateRoot(), childRecord); err != nil {
		t.Fatal(err)
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
	output := filepath.Join(canonicalSmokeTestPath(t, outputDir), "receipt.json")
	exact := fmt.Sprintf(
		"scripts/verify-child-host-smoke.sh --issue 69 --source-root %s --child-root %s --head 0123456789012345678901234567890123456789 --remote-ref refs/heads/69-v1-observation --json-out %s --confirm-user-activation",
		sourceArg,
		childArg,
		output,
	)
	base := lifecyclecontract.HookToolUseLifecycleRequest{
		CWD:  sourceArg,
		Tool: "Bash", ToolInput: map[string]any{"command": exact},
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
		req.CWD = other
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

	t.Run("child repo is not coordinator authority", func(t *testing.T) {
		req := base
		req.Command = exact
		req.Repo = childArg
		if exactCoordinatorChildHostSmoke(req) {
			t.Fatal("child repo was classified as coordinator authority")
		}
	})

	t.Run("unrelated repo is not coordinator authority", func(t *testing.T) {
		req := base
		req.Command = exact
		req.Repo = canonicalSmokeTestPath(t, t.TempDir())
		if exactCoordinatorChildHostSmoke(req) {
			t.Fatal("unrelated repo was classified as coordinator authority")
		}
	})

	t.Run("missing delegation", func(t *testing.T) {
		req := base
		req.Command = exact
		withoutDelegation := childRecord
		withoutDelegation.Delegation = nil
		if _, err := writeIssueOps(IssueOpsStateRoot(), withoutDelegation); err != nil {
			t.Fatal(err)
		}
		if exactCoordinatorChildHostSmoke(req) {
			t.Fatal("child without durable delegation was classified")
		}
		if _, err := writeIssueOps(IssueOpsStateRoot(), childRecord); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing reverse child ref", func(t *testing.T) {
		req := base
		req.Command = exact
		withoutChild := coordinatorRecord
		withoutChild.ChildCycles = nil
		if _, err := writeIssueOps(IssueOpsStateRoot(), withoutChild); err != nil {
			t.Fatal(err)
		}
		if exactCoordinatorChildHostSmoke(req) {
			t.Fatal("parent without reverse child ref was classified")
		}
		if _, err := writeIssueOps(IssueOpsStateRoot(), coordinatorRecord); err != nil {
			t.Fatal(err)
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
