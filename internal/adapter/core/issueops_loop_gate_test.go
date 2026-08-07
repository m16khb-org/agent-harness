package core

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"

	"agent-harness/internal/adapter/looprun"
	"agent-harness/internal/adapter/outbound/sqlstore"
)

func TestIssueOpsStrictPRReadinessBlocksActiveLoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForLoopGateTest(t)

	loop := startCoreLoopGateLoop(t, record.Repo, "active-loop", 3)
	ready := IssueOpsStrictPRReadiness(record)
	if ready.Ready || !containsLoopGateString(ready.Missing, "loop_incomplete:"+loop.ID) {
		t.Fatalf("active loop should block strict readiness, got %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessIgnoresOtherRepoLoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForLoopGateTest(t)

	startCoreLoopGateLoop(t, filepath.Join(t.TempDir(), "other-repo"), "other-loop", 3)
	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Ready || containsLoopGateString(ready.Missing, "loop_incomplete:") {
		t.Fatalf("other repo loop should not block strict readiness, got %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessIgnoresStaleRetiredPoolState(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForLoopGateTest(t)

	root := filepath.Join(os.Getenv("HARNESS_STATE_DIR"), strings.Join([]string{"work", "pool"}, ""))
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := json.Marshal(map[string]any{
		"id": "wp-stale", "repo": record.Repo, "parent_cycle_id": record.ID, "status": "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("pool", "wp-stale", entry); err != nil {
		t.Fatal(err)
	}

	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Ready || containsLoopGateString(ready.Missing, "pool_incomplete:") {
		t.Fatalf("stale retired state must not block strict readiness, got %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessBlocksExhaustedLoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForLoopGateTest(t)

	loop := startCoreLoopGateLoop(t, record.Repo, "exhausted-loop", 1)
	if _, err := looprun.RecordAttempt(loop.ID, looprun.RecordAttemptRequest{
		Verdict:  "fail",
		Evidence: []string{"focused verification failed"},
	}); err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}
	ready := IssueOpsStrictPRReadiness(record)
	if ready.Ready || !containsLoopGateString(ready.Missing, "loop_incomplete:"+loop.ID) {
		t.Fatalf("exhausted loop should block strict readiness, got %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessClearsAfterLoopStop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForLoopGateTest(t)

	loop := startCoreLoopGateLoop(t, record.Repo, "stopped-loop", 3)
	if _, err := looprun.Stop(loop.ID, false, "operator stopped loop after explicit handoff"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Ready || containsLoopGateString(ready.Missing, "loop_incomplete:"+loop.ID) {
		t.Fatalf("stopped loop should clear strict readiness, got %+v", ready)
	}

	successLoop := startCoreLoopGateLoop(t, record.Repo, "succeeded-loop", 3)
	if _, err := looprun.RecordAttempt(successLoop.ID, looprun.RecordAttemptRequest{
		Verdict:  "pass",
		Evidence: []string{"focused verification passed"},
	}); err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}
	if _, err := looprun.Stop(successLoop.ID, true, ""); err != nil {
		t.Fatalf("Stop success: %v", err)
	}
	ready = IssueOpsStrictPRReadiness(record)
	if !ready.Ready || containsLoopGateString(ready.Missing, "loop_incomplete:"+successLoop.ID) {
		t.Fatalf("succeeded loop should clear strict readiness, got %+v", ready)
	}
}

func startCoreLoopGateLoop(t *testing.T, repo, name string, maxAttempts int) looprun.LoopRun {
	t.Helper()
	loop, err := looprun.Start(looprun.StartLoopRequest{
		Repo:        repo,
		Name:        name,
		Goal:        "verify strict loop gate behavior",
		MaxAttempts: maxAttempts,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return loop
}

func readyIssueOpsRecordForLoopGateTest(t *testing.T) issueopscontract.IssueOpsRecord {
	t.Helper()
	repo := initCoreLoopGateRepo(t)
	record := issueopscontract.IssueOpsRecord{
		OK:            true,
		SchemaVersion: IssueOpsCurrentSchemaVersion,
		ID:            NewIssueOpsID(repo, "main"),
		Repo:          repo,
		Branch:        "main",
		IssueURL:      "https://github.com/acme/repo/issues/11",
		PlanPath:      "plans/demo.md",
		WorktreePath:  repo,
		Intent: &issueopscontract.IssueOpsIntentContract{
			RawRequest:        "ship task 11",
			InterpretedIntent: "ship task 11",
			SuccessCriteria:   []string{"loop gate works"},
			RecordedAt:        "2026-07-07T00:00:00Z",
		},
		DesignReview: &issueopscontract.IssueOpsDesignReview{
			ProblemSummary: "durable loop readiness",
			ProposedDesign: "check loops for the target repository",
			RefactorPlan:   "apply the loop gate",
			Alternatives:   []string{"ignore active loops"},
			Risks:          []string{"stale readiness"},
			Verification:   []string{"design review checked alternatives and risks"},
			Approved:       true,
			ReviewedAt:     "2026-07-07T00:00:00Z",
		},
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{
			Provider:     "github",
			IssueURL:     "https://github.com/acme/repo/issues/11",
			Branch:       "main",
			BaseBranch:   "main",
			LinkVerified: true,
			CreatedAt:    "2026-07-07T00:00:00Z",
		},
		AISlopCleanAt: "2026-07-07T00:00:00Z",
	}
	if _, err := WriteIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatalf("WriteIssueOps: %v", err)
	}
	return record
}

func initCoreLoopGateRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	remote := filepath.Join(root, "remote.git")
	runCoreLoopGateGit(t, root, "git", "init", "--bare", remote)
	runCoreLoopGateGit(t, root, "git", "init", repo)
	runCoreLoopGateGit(t, repo, "git", "config", "user.email", "test@example.com")
	runCoreLoopGateGit(t, repo, "git", "config", "user.name", "Test User")
	runCoreLoopGateGit(t, repo, "git", "checkout", "-b", "main")
	if err := os.MkdirAll(filepath.Join(repo, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "plans", "demo.md"), []byte("plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCoreLoopGateGit(t, repo, "git", "add", "plans/demo.md")
	runCoreLoopGateGit(t, repo, "git", "commit", "-m", "seed")
	runCoreLoopGateGit(t, repo, "git", "remote", "add", "origin", remote)
	runCoreLoopGateGit(t, repo, "git", "push", "-u", "origin", "main")
	return repo
}

func runCoreLoopGateGit(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
}

func containsLoopGateString(values []string, target string) bool {
	for _, value := range values {
		if value == target || strings.HasSuffix(target, ":") && strings.HasPrefix(value, target) {
			return true
		}
	}
	return false
}
