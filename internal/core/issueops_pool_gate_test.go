package core

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/workpool"
)

func TestIssueOpsStrictPRReadinessBlocksOpenPool(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForPoolGateTest(t)

	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Ready || containsCorePoolGateString(ready.Missing, "pool_incomplete:") {
		t.Fatalf("parent with no linked pool should be strict-ready, got %+v", ready)
	}

	pool := writeCorePoolGatePool(t, record, "open-pool", "active")
	ready = IssueOpsStrictPRReadiness(record)
	if ready.Ready || !containsCorePoolGateString(ready.Missing, "pool_incomplete:"+pool.ID) {
		t.Fatalf("open linked pool should block strict readiness, got %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessClearsAfterPoolClose(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := readyIssueOpsRecordForPoolGateTest(t)
	pool := writeCorePoolGatePool(t, record, "closable-pool", "active")
	task, err := workpool.AddTask(pool.ID, workpool.AddTaskRequest{
		Title:        "mechanical update",
		Instructions: "update one scoped file",
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	ready := IssueOpsStrictPRReadiness(record)
	if ready.Ready || !containsCorePoolGateString(ready.Missing, "pool_incomplete:"+pool.ID) {
		t.Fatalf("linked pool with pending task should block strict readiness, got %+v", ready)
	}

	claimed, err := workpool.Claim(pool.ID, "worker-a")
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if claimed.Task.ID != task.ID {
		t.Fatalf("claimed task %s, want %s", claimed.Task.ID, task.ID)
	}
	if _, err := workpool.Submit(pool.ID, task.ID, "worker-a", []string{"go test ./pkg -count=1"}, "pool/branch", filepath.Join(record.Repo, "pool-worktree")); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := workpool.Accept(pool.ID, task.ID, []string{"reviewed submitted diff"}); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if _, err := workpool.Close(pool.ID, false, ""); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ready = IssueOpsStrictPRReadiness(record)
	if !ready.Ready || containsCorePoolGateString(ready.Missing, "pool_incomplete:"+pool.ID) {
		t.Fatalf("closed linked pool with terminal tasks should clear strict readiness, got %+v", ready)
	}
}

func readyIssueOpsRecordForPoolGateTest(t *testing.T) IssueOpsRecord {
	t.Helper()
	repo := initCorePoolGateRepo(t)
	record := IssueOpsRecord{
		OK:            true,
		SchemaVersion: IssueOpsCurrentSchemaVersion,
		ID:            NewIssueOpsID(repo, "main"),
		Repo:          repo,
		Branch:        "main",
		IssueURL:      "https://github.com/acme/repo/issues/11",
		PlanPath:      "plans/demo.md",
		WorktreePath:  repo,
		Intent: &IssueOpsIntentContract{
			RawRequest:        "ship task 11",
			InterpretedIntent: "ship task 11",
			SuccessCriteria:   []string{"pool gate works"},
			RecordedAt:        "2026-07-07T00:00:00Z",
		},
		DesignReview: &IssueOpsDesignReview{
			ProblemSummary: "pool parent gate",
			ProposedDesign: "scan linked work pools",
			RefactorPlan:   "add facade gate",
			Alternatives:   []string{"ignore pools"},
			Risks:          []string{"stale parent readiness"},
			Verification:   []string{"design review checked alternatives and risks"},
			Approved:       true,
			ReviewedAt:     "2026-07-07T00:00:00Z",
		},
		BranchPrepare: &IssueOpsBranchPrepare{
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

func writeCorePoolGatePool(t *testing.T, record IssueOpsRecord, name, status string) workpool.WorkPool {
	t.Helper()
	pool := workpool.WorkPool{
		OK:            true,
		SchemaVersion: workpool.WorkPoolCurrentSchemaVersion,
		ID:            "wp-" + strings.Repeat("a", 12),
		Repo:          record.Repo,
		Name:          name,
		ParentCycleID: record.ID,
		Size:          4,
		LeaseTTL:      "15m",
		MaxAttempts:   2,
		Status:        status,
		CreatedAt:     "2026-07-07T00:00:00Z",
		UpdatedAt:     "2026-07-07T00:00:00Z",
	}
	if err := os.MkdirAll(workpool.StateRoot(), 0o700); err != nil {
		t.Fatal(err)
	}
	writeCorePoolGateJSON(t, filepath.Join(workpool.StateRoot(), pool.ID+".json"), pool)
	return pool
}

func writeCorePoolGateJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func initCorePoolGateRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	remote := filepath.Join(root, "remote.git")
	runCorePoolGateGit(t, root, "git", "init", "--bare", remote)
	runCorePoolGateGit(t, root, "git", "init", repo)
	runCorePoolGateGit(t, repo, "git", "config", "user.email", "test@example.com")
	runCorePoolGateGit(t, repo, "git", "config", "user.name", "Test User")
	runCorePoolGateGit(t, repo, "git", "checkout", "-b", "main")
	if err := os.MkdirAll(filepath.Join(repo, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "plans", "demo.md"), []byte("plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCorePoolGateGit(t, repo, "git", "add", "plans/demo.md")
	runCorePoolGateGit(t, repo, "git", "commit", "-m", "seed")
	runCorePoolGateGit(t, repo, "git", "remote", "add", "origin", remote)
	runCorePoolGateGit(t, repo, "git", "push", "-u", "origin", "main")
	return repo
}

func runCorePoolGateGit(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
}

func containsCorePoolGateString(values []string, target string) bool {
	for _, value := range values {
		if value == target || strings.HasSuffix(target, ":") && strings.HasPrefix(value, target) {
			return true
		}
	}
	return false
}
