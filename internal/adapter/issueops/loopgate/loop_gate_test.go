package loopgate

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"

	"agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/looprun"
	"agent-harness/internal/adapter/outbound/sqlstore"
	loopruncontract "agent-harness/internal/contract/looprun"
)

func TestIssueOpsStrictPRReadinessBlocksActiveLoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForLoopGateTest(t)

	loop := startCoreLoopGateLoop(t, record.Repo, "active-loop", 3)
	ready := StrictPRReadiness(record)
	if ready.Ready || !containsLoopGateString(ready.Missing, "loop_incomplete:"+loop.ID) {
		t.Fatalf("active loop should block strict readiness, got %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessIgnoresOtherRepoLoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForLoopGateTest(t)

	startCoreLoopGateLoop(t, filepath.Join(t.TempDir(), "other-repo"), "other-loop", 3)
	ready := StrictPRReadiness(record)
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

	ready := StrictPRReadiness(record)
	if !ready.Ready || containsLoopGateString(ready.Missing, "pool_incomplete:") {
		t.Fatalf("stale retired state must not block strict readiness, got %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessBlocksExhaustedLoop(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForLoopGateTest(t)

	loop := startCoreLoopGateLoop(t, record.Repo, "exhausted-loop", 1)
	if _, err := looprun.RecordAttempt(loop.ID, loopruncontract.RecordAttemptRequest{
		Verdict:  "fail",
		Evidence: []string{"focused verification failed"},
	}); err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}
	ready := StrictPRReadiness(record)
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
	ready := StrictPRReadiness(record)
	if !ready.Ready || containsLoopGateString(ready.Missing, "loop_incomplete:"+loop.ID) {
		t.Fatalf("stopped loop should clear strict readiness, got %+v", ready)
	}

	successLoop := startCoreLoopGateLoop(t, record.Repo, "succeeded-loop", 3)
	if _, err := looprun.RecordAttempt(successLoop.ID, loopruncontract.RecordAttemptRequest{
		Verdict:  "pass",
		Evidence: []string{"focused verification passed"},
	}); err != nil {
		t.Fatalf("RecordAttempt: %v", err)
	}
	if _, err := looprun.Stop(successLoop.ID, true, ""); err != nil {
		t.Fatalf("Stop success: %v", err)
	}
	ready = StrictPRReadiness(record)
	if !ready.Ready || containsLoopGateString(ready.Missing, "loop_incomplete:"+successLoop.ID) {
		t.Fatalf("succeeded loop should clear strict readiness, got %+v", ready)
	}
}

func startCoreLoopGateLoop(t *testing.T, repo, name string, maxAttempts int) loopruncontract.LoopRun {
	t.Helper()
	loop, err := looprun.Start(loopruncontract.StartLoopRequest{
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
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            issueops.NewIssueOpsID(repo, "main"),
		Repo:          repo,
		Branch:        "main",
		Phase:         issueopscontract.IssueOpsPhasePR,
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
	if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), record); err != nil {
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

// AdvancePhase는 pr 진입 시 strict readiness를 강제하고, pr 재진입(이미 pr)과
// pr 외 전환은 통과시킨다. dogfooding에서 확인한 그 gate를 잠근다.
func TestAdvancePhaseGuardsPRTransition(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForLoopGateTest(t)
	stateRoot := issueops.IssueOpsStateRoot()
	written, err := issueops.WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	written.Phase = issueopscontract.IssueOpsPhaseFeedback
	written, err = issueops.WriteIssueOps(stateRoot, written)
	if err != nil {
		t.Fatal(err)
	}

	// pr이 아닌 전환은 loop gate 없이 core 위임이다(feedback -> pr는 아래
	// gate 케이스가 담당하므로 여기선 하위 위상으로 되돌리지 않는다).
	// active loop이 있으면 pr 진입이 loop_incomplete로 거부된다.
	loop := startCoreLoopGateLoop(t, record.Repo, "advance-loop", 3)
	_, err = AdvancePhase(stateRoot, written.ID, "pr")
	if err == nil || !strings.Contains(err.Error(), "loop_incomplete:"+loop.ID) {
		t.Fatalf("pr entry must be gated by the active loop: %v", err)
	}

	// loop를 성공으로 끝내면 pr 재진입이 통과한다(복구 경로 보존).
	if _, err := looprun.RecordAttempt(loop.ID, loopruncontract.RecordAttemptRequest{Verdict: "pass", Evidence: []string{"gate cleared"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := looprun.Stop(loop.ID, true, "goal met"); err != nil {
		t.Fatal(err)
	}
	inPR, err := AdvancePhase(stateRoot, written.ID, "pr")
	if err != nil || inPR.Phase != issueopscontract.IssueOpsPhasePR {
		t.Fatalf("pr entry after successful loop = %#v err=%v", inPR, err)
	}
	// 이미 pr인 레코드의 pr 재진입도 통과한다.
	again, err := AdvancePhase(stateRoot, written.ID, "pr")
	if err != nil || again.Phase != issueopscontract.IssueOpsPhasePR {
		t.Fatalf("pr re-entry must pass for recovery: %#v err=%v", again, err)
	}
}

func TestAdvancePhaseRejectsUnknownRecord(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := AdvancePhase(issueops.IssueOpsStateRoot(), "io-missing", "pr"); err == nil {
		t.Fatal("unknown record must fail before any transition")
	}
}

func TestStrictPRReadinessWithStateAppliesLoopGate(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForLoopGateTest(t)
	startCoreLoopGateLoop(t, record.Repo, "state-loop", 3)
	ready := StrictPRReadinessWithState(issueops.IssueOpsStateRoot(), record)
	if ready.Ready {
		t.Fatalf("with-state readiness must honor the loop gate: %+v", ready)
	}
}
