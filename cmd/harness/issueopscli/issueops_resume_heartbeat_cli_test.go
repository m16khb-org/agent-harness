package issueopscli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestIssueOpsResumeByID(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "resume-by-id")
	parent := startIssueOpsCLIReadyDelegationParent(t, repo, "123-parent-resume")
	child, childWorktree := startIssueOpsCLIChildWithLinkedWorktree(t, parent, "124-child-resume")
	if err := core.UnbindScopedIssueOpsSessionForCycle(repo, child.ID); err != nil {
		t.Fatal(err)
	}

	unboundOut := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"resume", "--repo", repo, "--id", child.ID, "--json"})
	})
	var unbound core.IssueOpsResumeResult
	if err := json.Unmarshal([]byte(unboundOut), &unbound); err != nil {
		t.Fatalf("resume --id should return JSON: %v\n%s", err, unboundOut)
	}
	if !unbound.OK || unbound.CycleID != child.ID || unbound.WorktreePath != childWorktree {
		t.Fatalf("resume --id should resolve the requested child cycle, got %#v", unbound)
	}
	if !strings.Contains(unbound.Guidance, "HARNESS_EXPECTED_WORKTREE="+childWorktree) {
		t.Fatalf("resume --id should return child worktree guidance, got %q", unbound.Guidance)
	}
	if scoped, err := core.ReadScopedIssueOpsSession(repo, child.ID); err != nil {
		t.Fatal(err)
	} else if scoped.CycleID != "" {
		t.Fatalf("resume --id without --bind should not bind scoped session, got %+v", scoped)
	}

	boundOut := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"resume", "--repo", repo, "--id", child.ID, "--bind", "--json"})
	})
	var bound core.IssueOpsResumeResult
	if err := json.Unmarshal([]byte(boundOut), &bound); err != nil {
		t.Fatalf("resume --id --bind should return JSON: %v\n%s", err, boundOut)
	}
	if !bound.OK || bound.CycleID != child.ID || bound.WorktreePath != childWorktree {
		t.Fatalf("resume --id --bind should resolve the requested child cycle, got %#v", bound)
	}
	scoped, err := core.ReadScopedIssueOpsSession(repo, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if scoped.CycleID != child.ID || scoped.ExpectedWorktree != childWorktree {
		t.Fatalf("resume --id --bind should bind delegated child scoped session, got %+v", scoped)
	}
	primary, err := core.ReadIssueOpsSession(repo)
	if err != nil {
		t.Fatal(err)
	}
	if primary.CycleID != parent.ID {
		t.Fatalf("child resume bind must not clobber parent primary binding, got %+v", primary)
	}
}

func TestIssueOpsHeartbeatCLIUpdatesLastHeartbeat(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "heartbeat")
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: repo, Branch: "123-heartbeat"})
	if err != nil {
		t.Fatal(err)
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"heartbeat", "--id", record.ID, "--json"})
	})
	var updated core.IssueOpsRecord
	if err := json.Unmarshal([]byte(out), &updated); err != nil {
		t.Fatalf("heartbeat should return issueops record JSON: %v\n%s", err, out)
	}
	if updated.LastHeartbeatAt == "" {
		t.Fatalf("heartbeat should stamp last_heartbeat_at: %#v", updated)
	}
	read, err := core.ReadIssueOps(core.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.LastHeartbeatAt != updated.LastHeartbeatAt {
		t.Fatalf("heartbeat should persist timestamp, read=%q returned=%q", read.LastHeartbeatAt, updated.LastHeartbeatAt)
	}
}

func TestForceReleasedChildStillIncompleteThenResumable(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIGitRepoForRemoteVerifyTest(t)
	parent := startIssueOpsCLIReadyPRParentWithChild(t, repo, "123-parent-resume-force")
	startedOut := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"child", "start",
			"--parent", parent.ID,
			"--branch", "124-child-resume-force",
			"--title", "child resumable force release",
			"--scope", "restore child scoped binding after force-release",
			"--acceptance", "resume by id can bind child worktree",
			"--json",
		})
	})
	var started core.IssueOpsChildStartResult
	if err := json.Unmarshal([]byte(startedOut), &started); err != nil {
		t.Fatalf("child start should return JSON: %v\n%s", err, startedOut)
	}
	child, childWorktree := prepareIssueOpsCLIChildForPRFlow(t, parent, started.Child)
	if _, err := core.ForceReleaseIssueOps(core.IssueOpsStateRoot(), child.ID, "test child worker context lost"); err != nil {
		t.Fatal(err)
	}
	parent, err := core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), parent.ID, string(core.IssueOpsPhaseAISlopClean))
	if err != nil {
		t.Fatal(err)
	}

	blockedOut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"phase", "--id", parent.ID, "--to", "pr", "--json"})
	})
	if err == nil || !strings.Contains(blockedOut, "child_incomplete:"+child.ID) {
		t.Fatalf("force-released child should remain incomplete for parent gate, err=%v out=%s", err, blockedOut)
	}
	if err := core.UnbindScopedIssueOpsSessionForCycle(repo, child.ID); err != nil {
		t.Fatal(err)
	}
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"resume", "--repo", repo, "--id", child.ID, "--bind", "--json"})
	})
	scoped, err := core.ReadScopedIssueOpsSession(repo, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if scoped.CycleID != child.ID || scoped.ExpectedWorktree != childWorktree {
		t.Fatalf("resume --id --bind should restore child scoped binding after force release, got %+v", scoped)
	}

	child, err = core.ReadIssueOps(core.IssueOpsStateRoot(), child.ID)
	if err != nil {
		t.Fatal(err)
	}
	child.ForceReleasedAt = ""
	child.ForceReleaseReason = ""
	child.Phase = core.IssueOpsPhaseDone
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), child); err != nil {
		t.Fatal(err)
	}
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"child", "accept", "--parent", parent.ID, "--child", child.ID, "--evidence", "child completed after resume", "--json"})
	})
	prOut := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"phase", "--id", parent.ID, "--to", "pr", "--json"})
	})
	var prRecord core.IssueOpsRecord
	if err := json.Unmarshal([]byte(prOut), &prRecord); err != nil {
		t.Fatalf("phase pr should return JSON after child acceptance: %v\n%s", err, prOut)
	}
	if prRecord.Phase != core.IssueOpsPhasePR {
		t.Fatalf("accepted completed child should allow parent pr phase, got %s", prRecord.Phase)
	}
}

func startIssueOpsCLIChildWithLinkedWorktree(t *testing.T, parent core.IssueOpsRecord, branch string) (core.IssueOpsRecord, string) {
	t.Helper()
	started, err := core.StartIssueOpsChild(core.IssueOpsStateRoot(), core.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             branch,
		Title:              "child resume task",
		TaskScope:          "child scoped resume",
		AcceptanceCriteria: []string{"resume by id returns child guidance"},
		ParentPlanPath:     parent.PlanPath,
		ChildIssueURL:      "https://github.com/example/repo/issues/124",
	})
	if err != nil {
		t.Fatal(err)
	}
	return prepareIssueOpsCLIChildForPRFlow(t, parent, started.Child)
}

func prepareIssueOpsCLIChildForPRFlow(t *testing.T, parent core.IssueOpsRecord, child core.IssueOpsRecord) (core.IssueOpsRecord, string) {
	t.Helper()
	childIssueURL := "https://github.com/example/repo/issues/" + strings.SplitN(child.Branch, "-", 2)[0]
	child, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), child.ID, childIssueURL)
	if err != nil {
		t.Fatal(err)
	}
	child, err = core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), child.ID, core.IssueOpsBranchPrepareRequest{
		Provider: "github", IssueURL: child.IssueURL, Branch: child.Branch, BaseBranch: parent.Branch, LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	worktree := makeIssueOpsCLIWorktreeForTest(t, parent.Repo, child.Branch)
	writeIssueOpsCLIFileForTest(t, worktree, "plans/child.md", "child plan\n")
	child, err = core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), child.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	child.PlanPath = filepath.Join(worktree, "plans", "child.md")
	child.WorktreeTools = &core.IssueOpsWorktreeToolPreparation{OK: true, WorktreePath: worktree, CodeGraphProjectPath: worktree, CodeGraphChecked: true, CodeGraphReady: true}
	child.CompatibilityReview = &core.IssueOpsCompatibilityReview{BackwardCompatibility: []string{"child resume fixture"}, SideEffects: []string{"scoped binding"}, RollbackPlan: "reset fixture", Verification: []string{"go test ./cmd/harness/issueopscli"}, Approved: true}
	child.ExecutionDecision = &core.IssueOpsExecutionDecision{AutoProceed: []string{"complete child fixture"}, HookBlocked: []string{"hooks do not resume child"}, HumanGates: []string{"parent validates"}, SubagentUse: "none", SubagentRationale: "test fixture", RecordedAt: "2026-07-07T00:00:00Z"}
	child.Phase = core.IssueOpsPhaseImplement
	child, err = core.WriteIssueOps(core.IssueOpsStateRoot(), child)
	if err != nil {
		t.Fatal(err)
	}
	return child, worktree
}
