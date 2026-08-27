package issueopscli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	issueopscore "agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/issueops/loopgate"
	preflight "agent-harness/internal/adapter/preflight"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestRunIssueOpsChildLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	parent, actor := startIssueOpsCLIReadyDelegationParent(t, makeIssueOpsCLIRepoForTest(t, "delegation-cli"), "123-parent-child-cli")

	startOut := captureStdoutForContract(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{
			"child", "start",
			"--parent", parent.ID,
			"--branch", "123-child-cli",
			"--title", "child cli task",
			"--scope", "implement the delegated child cli fixture",
			"--acceptance", "child cli start returns JSON",
			"--acceptance", "parent can validate child result",
			"--json",
		}, actor))
	})
	var started issueopscontract.IssueOpsChildStartResult
	if err := json.Unmarshal([]byte(startOut), &started); err != nil {
		t.Fatalf("child start should return JSON: %v\n%s", err, startOut)
	}
	if !started.OK || started.ParentID != parent.ID || started.Child.ID == "" || started.ParentRef.CycleID != started.Child.ID {
		t.Fatalf("unexpected child start result: %#v", started)
	}
	if !strings.Contains(started.Guidance, parent.Branch) || !strings.Contains(started.Guidance, "HARNESS_EXPECTED_WORKTREE") {
		t.Fatalf("child start should surface core delegation guidance, got %q", started.Guidance)
	}

	statusOut := captureStdoutForContract(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{"child", "status", "--parent", parent.ID, "--json"}, actor))
	})
	var status issueopscontract.IssueOpsChildStatusResult
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("child status should return JSON: %v\n%s", err, statusOut)
	}
	if !status.OK || len(status.Children) != 1 || status.Children[0].CycleID != started.Child.ID {
		t.Fatalf("child status should include started child: %#v", status)
	}

	listOut := captureStdoutForContract(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{"child", "list", "--parent", parent.ID, "--json"}, actor))
	})
	var listed issueopscontract.IssueOpsChildStatusResult
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("child list should return status JSON: %v\n%s", err, listOut)
	}
	if len(listed.Children) != 1 || listed.Children[0].CycleID != started.Child.ID {
		t.Fatalf("child list should mirror status children, got %#v", listed.Children)
	}

	child := started.Child
	child.Phase = issueopscore.IssueOpsPhaseDone
	if _, err := issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), child); err != nil {
		t.Fatal(err)
	}
	acceptOut := captureStdoutForContract(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{"child", "accept", "--parent", parent.ID, "--child", child.ID, "--evidence", "parent verified child output", "--json"}, actor))
	})
	var accepted issueopscontract.IssueOpsChildValidationResult
	if err := json.Unmarshal([]byte(acceptOut), &accepted); err != nil {
		t.Fatalf("child accept should return JSON: %v\n%s", err, acceptOut)
	}
	if !accepted.OK || accepted.ParentRef.ValidationVerdict != "accepted" || len(accepted.ParentRef.ValidationEvidence) != 1 {
		t.Fatalf("child accept should record accepted verdict and evidence: %#v", accepted)
	}

	if _, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{"child", "status", "--json"}, actor))
	}); err == nil || !strings.Contains(err.Error(), "parent_id is required") {
		t.Fatalf("missing --parent should fail through core validation, got %v", err)
	}
	if _, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{"child", "reject", "--parent", parent.ID, "--child", child.ID, "--reason", "short", "--json"}, actor))
	}); err == nil || !strings.Contains(err.Error(), "reason must be at least 10 characters") {
		t.Fatalf("short reject reason should fail through core validation, got %v", err)
	}
}

func TestCLIIssueOpsPhaseAdvanceToPRBlockedByChildren(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIGitRepoForRemoteVerifyTest(t)
	parent, actor := startIssueOpsCLIReadyPRParentWithChild(t, repo, "123-parent-pr-child-gate")

	startOut := captureStdoutForContract(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{
			"child", "start",
			"--parent", parent.ID,
			"--branch", "123-child-pr-gate",
			"--title", "child pr gate task",
			"--scope", "prove parent pr gate blocks incomplete child",
			"--acceptance", "parent pr gate sees child status",
			"--json",
		}, actor))
	})
	var started issueopscontract.IssueOpsChildStartResult
	if err := json.Unmarshal([]byte(startOut), &started); err != nil {
		t.Fatalf("child start should return JSON: %v\n%s", err, startOut)
	}
	if _, err := loopgate.AdvancePhaseWithActor(issueopscore.IssueOpsStateRoot(), parent.ID, string(issueopscore.IssueOpsPhaseAISlopClean), actor); err != nil {
		t.Fatal(err)
	}

	blockedOut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{"phase", "--id", parent.ID, "--to", "pr", "--json"}, actor))
	})
	assertIssueOpsJSONErrorContains(t, blockedOut, err, "child_incomplete:"+started.Child.ID)

	child := started.Child
	child.Phase = issueopscore.IssueOpsPhaseDone
	if _, err := issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), child); err != nil {
		t.Fatal(err)
	}
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{"child", "accept", "--parent", parent.ID, "--child", child.ID, "--evidence", "merged child verification", "--json"}, actor))
	})
	prOut := captureStdoutForContract(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{"phase", "--id", parent.ID, "--to", "pr", "--json"}, actor))
	})
	var prRecord issueopscontract.IssueOpsRecord
	if err := json.Unmarshal([]byte(prOut), &prRecord); err != nil {
		t.Fatalf("phase pr should return JSON after child acceptance: %v\n%s", err, prOut)
	}
	if prRecord.Phase != issueopscore.IssueOpsPhasePR {
		t.Fatalf("accepted child should allow parent pr phase, got %s", prRecord.Phase)
	}
}

func TestCLIIssueOpsStrictPRReadinessReportsIncompleteChildren(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIGitRepoForRemoteVerifyTest(t)
	parent, actor := startIssueOpsCLIReadyPRParentWithChild(t, repo, "123-parent-pr-readiness-child-gate")

	startOut := captureStdoutForContract(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{
			"child", "start",
			"--parent", parent.ID,
			"--branch", "123-child-pr-readiness-gate",
			"--title", "child readiness gate task",
			"--scope", "prove parent strict pr-readiness reports incomplete child",
			"--acceptance", "parent pr-readiness sees child status",
			"--json",
		}, actor))
	})
	var started issueopscontract.IssueOpsChildStartResult
	if err := json.Unmarshal([]byte(startOut), &started); err != nil {
		t.Fatalf("child start should return JSON: %v\n%s", err, startOut)
	}
	if _, err := loopgate.AdvancePhaseWithActor(issueopscore.IssueOpsStateRoot(), parent.ID, string(issueopscore.IssueOpsPhaseAISlopClean), actor); err != nil {
		t.Fatal(err)
	}

	readinessOut := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"pr-readiness", "--id", parent.ID, "--strict", "--json"})
	})
	var readiness issueopscontract.IssueOpsReadiness
	if err := json.Unmarshal([]byte(readinessOut), &readiness); err != nil {
		t.Fatalf("strict pr-readiness should return JSON: %v\n%s", err, readinessOut)
	}
	if !containsString(readiness.Missing, "child_incomplete:"+started.Child.ID) {
		t.Fatalf("strict pr-readiness should include incomplete child gate, got %#v", readiness.Missing)
	}
}

func startIssueOpsCLIReadyDelegationParent(t *testing.T, repo, branch string) (issueopscontract.IssueOpsRecord, issueopscore.IssueOpsActor) {
	t.Helper()
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", branch, "--json"})
	})
	var record issueopscontract.IssueOpsRecord
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, out)
	}
	worktree := makeIssueOpsCLIWorktreeForTest(t, repo, branch)
	planPath := filepath.Join(worktree, "plans", "parent.md")
	writeIssueOpsCLIFileForTest(t, worktree, "plans/parent.md", "parent plan\n")
	prepareIssueOpsCLIParentImplementationSurface(t, record.ID, branch, worktree)
	recordIssueOpsCLIParentDelegationPrereqs(t, record.ID, planPath)
	record, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record, actor := seedIssueOpsCLIExecution(t, record)
	record, err = loopgate.AdvancePhaseWithActor(issueopscore.IssueOpsStateRoot(), record.ID, string(issueopscore.IssueOpsPhaseImplement), actor)
	if err != nil {
		t.Fatal(err)
	}
	return record, actor
}

func startIssueOpsCLIReadyPRParentWithChild(t *testing.T, repo, branch string) (issueopscontract.IssueOpsRecord, issueopscore.IssueOpsActor) {
	t.Helper()
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout parent branch failed: %s", stderr)
	}
	writeIssueOpsCLIFileForTest(t, repo, "plans/parent-pr.md", "parent plan\n")
	writeIssueOpsCLIFileForTest(t, repo, "internal/parent.go", "package parent\n")
	if code, _, stderr := preflight.GitCmd(repo, "add", "plans/parent-pr.md", "internal/parent.go"); code != 0 {
		t.Fatalf("git add parent files failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "-m", "feat: parent pr fixture"); code != 0 {
		t.Fatalf("git commit parent fixture failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push parent branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", branch)
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add parent branch failed: %s", stderr)
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", branch, "--json"})
	})
	var parent issueopscontract.IssueOpsRecord
	if err := json.Unmarshal([]byte(out), &parent); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, out)
	}
	prepareIssueOpsCLIParentImplementationSurface(t, parent.ID, branch, worktree)
	recordIssueOpsCLIParentDelegationPrereqs(t, parent.ID, filepath.Join(worktree, "plans", "parent-pr.md"))
	parent, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	parent, actor := seedIssueOpsCLIExecution(t, parent)
	parent, err = loopgate.AdvancePhaseWithActor(issueopscore.IssueOpsStateRoot(), parent.ID, string(issueopscore.IssueOpsPhaseImplement), actor)
	if err != nil {
		t.Fatal(err)
	}
	return parent, actor
}

func prepareIssueOpsCLIParentImplementationSurface(t *testing.T, id, branch, worktree string) {
	t.Helper()
	if _, err := issueopscore.LinkIssueOpsIssue(issueopscore.IssueOpsStateRoot(), id, "https://github.com/example/repo/issues/123"); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.PrepareIssueOpsBranch(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/123",
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.LinkIssueOpsWorktree(issueopscore.IssueOpsStateRoot(), id, worktree); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsCLIParentDelegationPrereqs(t *testing.T, id, planPath string) {
	t.Helper()
	recordIssueOpsCoreIntentForCLITest(t, id)
	recordIssueOpsCLIPlanPrepForTest(t, id)
	recordIssueOpsCoreDesignForCLITest(t, id)
	if _, err := issueopscore.LinkIssueOpsPlan(issueopscore.IssueOpsStateRoot(), id, planPath); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.RecordIssueOpsDomainReview(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsDomainReviewRequest{
		ModelFit:    "delegation cli fixture follows IssueOps domain model",
		Terminology: []string{"parent cycle", "child cycle"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.RecordIssueOpsCompatibilityReview(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsCompatibilityReviewRequest{
		BackwardCompatibility: []string{"existing IssueOps records remain readable"},
		SideEffects:           []string{"child CLI writes only IssueOps state"},
		RollbackPlan:          "Revert child CLI dispatch.",
		Verification:          []string{"go test ./cmd/harness/issueopscli"},
		Approved:              true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.RecordIssueOpsDevilsAdvocateReview(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "subagent", Findings: []string{"attacked gate 3"}}); err != nil {
		t.Fatal(err)
	}
}
