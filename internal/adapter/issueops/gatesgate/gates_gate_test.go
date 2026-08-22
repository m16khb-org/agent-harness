package gatesgate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/issueops/implementation"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func initGatesGateRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	remote := filepath.Join(root, "remote.git")
	runGatesGateGit(t, root, "git", "init", "--bare", remote)
	runGatesGateGit(t, root, "git", "init", repo)
	runGatesGateGit(t, repo, "git", "config", "user.email", "test@example.com")
	runGatesGateGit(t, repo, "git", "config", "user.name", "Test User")
	runGatesGateGit(t, repo, "git", "checkout", "-b", "main")
	if err := os.MkdirAll(filepath.Join(repo, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "plans", "demo.md"), []byte("plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGatesGateGit(t, repo, "git", "add", "plans/demo.md")
	runGatesGateGit(t, repo, "git", "commit", "-m", "seed")
	runGatesGateGit(t, repo, "git", "remote", "add", "origin", remote)
	runGatesGateGit(t, repo, "git", "push", "-u", "origin", "main")
	return repo
}

func runGatesGateGit(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
}

func readyGatesGateRecord(t *testing.T) issueopscontract.IssueOpsRecord {
	t.Helper()
	repo := initGatesGateRepo(t)
	record := issueopscontract.IssueOpsRecord{
		OK:            true,
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            issueops.NewIssueOpsID(repo, "main"),
		Repo:          repo,
		Branch:        "main",
		Phase:         issueopscontract.IssueOpsPhasePR,
		IssueURL:      "https://github.com/acme/repo/issues/21",
		PlanPath:      "plans/demo.md",
		WorktreePath:  repo,
		Intent: &issueopscontract.IssueOpsIntentContract{
			RawRequest:        "ship gates gate",
			InterpretedIntent: "ship gates gate",
			SuccessCriteria:   []string{"gates gate works"},
			RecordedAt:        "2026-07-07T00:00:00Z",
		},
		DesignReview: &issueopscontract.IssueOpsDesignReview{
			ProblemSummary: "gates ledger readiness",
			ProposedDesign: "block PR while gates unmet",
			RefactorPlan:   "apply the gates gate",
			Alternatives:   []string{"trust self-reports"},
			Risks:          []string{"stale readiness"},
			Verification:   []string{"design review checked alternatives and risks"},
			Approved:       true,
			ReviewedAt:     "2026-07-07T00:00:00Z",
		},
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{
			Provider:     "github",
			IssueURL:     "https://github.com/acme/repo/issues/21",
			Branch:       "main",
			BaseBranch:   "main",
			LinkVerified: true,
			CreatedAt:    "2026-07-07T00:00:00Z",
		},
		AISlopCleanAt: "2026-07-07T00:00:00Z",
	}
	record.AISlopCleanFingerprint = implementation.ChangeFingerprint(record)
	if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), record); err != nil {
		t.Fatalf("WriteIssueOps: %v", err)
	}
	return record
}

func writeGatesLedger(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "GATES.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// 게이트 ledger는 worktree 콘텐츠의 일부다. 커밋하지 않으면 strict
	// readiness의 worktree_clean/fingerprint 검사에 걸린다 — 실제 사이클에서도
	// ledger는 PR 전에 커밋된다.
	runGatesGateGit(t, root, "git", "add", "GATES.md")
	runGatesGateGit(t, root, "git", "commit", "-m", "add gates ledger")
	runGatesGateGit(t, root, "git", "push", "origin", "main")
}

func TestStrictPRReadinessWithoutLedgerStaysReady(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyGatesGateRecord(t)
	ready := StrictPRReadinessWithState(issueops.IssueOpsStateRoot(), record)
	if !ready.Ready {
		t.Fatalf("no ledger must not add gates missing: %+v", ready.Missing)
	}
}

func TestStrictPRReadinessBlocksOnUnmetGates(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyGatesGateRecord(t)
	writeGatesLedger(t, record.Repo, "# Gates: cycle\n\n- [ ] G1: tests pass\n  CHECK: go test ./...\n  EVIDENCE: pending\n\n- [x] G2: stale claim\n  EVIDENCE: pending\n")
	ready := StrictPRReadinessWithState(issueops.IssueOpsStateRoot(), record)
	if ready.Ready {
		t.Fatalf("unmet gates must block readiness: %+v", ready)
	}
	if !containsMissing(ready.Missing, "gates_incomplete:GATES.md") {
		t.Fatalf("gates_incomplete missing entry absent: %+v", ready.Missing)
	}
	if !hasWarning(ready.Warnings, "G1") || !hasWarning(ready.Warnings, "G2") {
		t.Fatalf("per-gate warnings absent: %+v", ready.Warnings)
	}
}

func TestStrictPRReadinessPassesOnMetAndAbandonedGates(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyGatesGateRecord(t)
	writeGatesLedger(t, record.Repo, "# Gates: cycle\n\n- [x] G1: tests pass\n  EVIDENCE: go test ./... — all packages ok\n\n- [ ] G2: manual doc polish\n  EVIDENCE: pending\n\nABANDON: G2 outcome verified by review\n")
	ready := StrictPRReadinessWithState(issueops.IssueOpsStateRoot(), record)
	if !ready.Ready {
		t.Fatalf("met+abandoned gates must stay ready: %+v %+v", ready.Missing, ready.Warnings)
	}
}

func TestStrictPRReadinessBlocksOnEvidencePendingOnly(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyGatesGateRecord(t)
	writeGatesLedger(t, record.Repo, "- [x] G1: claimed done\n  EVIDENCE: pending\n")
	ready := StrictPRReadinessWithState(issueops.IssueOpsStateRoot(), record)
	if ready.Ready || !containsMissing(ready.Missing, "gates_incomplete:GATES.md") {
		t.Fatalf("checked-but-pending must block readiness: %+v", ready)
	}
}

func TestAdvancePhaseGuardsPRWithGates(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyGatesGateRecord(t)
	if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), func() issueopscontract.IssueOpsRecord {
		regressed := record
		regressed.Phase = issueopscontract.IssueOpsPhaseImplement
		return regressed
	}()); err != nil {
		t.Fatal(err)
	}
	stateRoot := issueops.IssueOpsStateRoot()

	// 게이트 없으면 loopgate와 동일하게 pr 진입 가능(다른 readiness는 이미 충족).
	writeGatesLedger(t, record.Repo, "- [x] G1: done\n  EVIDENCE: measured\n")
	if _, err := AdvancePhaseWithActor(stateRoot, record.ID, "pr", issueops.IssueOpsActor{Host: "codex"}); err != nil {
		t.Fatalf("pr entry with met gates must pass: %v", err)
	}

	// 미충족 게이트면 pr 진입이 거부된다.
	blockRecord := func() issueopscontract.IssueOpsRecord {
		regressed := record
		regressed.Phase = issueopscontract.IssueOpsPhaseImplement
		return regressed
	}()
	if _, err := issueops.WriteIssueOps(stateRoot, blockRecord); err != nil {
		t.Fatal(err)
	}
	writeGatesLedger(t, record.Repo, "- [ ] G1: blocked\n  EVIDENCE: pending\n")
	_, err := AdvancePhaseWithActor(stateRoot, record.ID, "pr", issueops.IssueOpsActor{Host: "codex"})
	if err == nil || !strings.Contains(err.Error(), "gates_incomplete") {
		t.Fatalf("pr entry with unmet gates must fail with gates_incomplete: %v", err)
	}

	// pr 외 전환은 게이트와 무관하게 통과.
	if _, err := AdvancePhaseWithActor(stateRoot, record.ID, "feedback", issueops.IssueOpsActor{Host: "codex"}); err != nil {
		t.Fatalf("non-pr transition must pass: %v", err)
	}
}

func containsMissing(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasWarning(warnings []string, gateID string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, "gate "+gateID+" ") {
			return true
		}
	}
	return false
}
