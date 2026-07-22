package workpool

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops"
)

func TestCreatePoolSizeClamp(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	tests := []struct {
		name     string
		size     int
		wantSize int
		wantErr  string
	}{
		{name: "default", size: 0, wantSize: 4},
		{name: "minimum", size: 1, wantSize: 1},
		{name: "maximum", size: 16, wantSize: 16},
		{name: "too large", size: 17, wantErr: "pool_size_out_of_range"},
		{name: "negative", size: -1, wantErr: "pool_size_out_of_range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := CreatePool(CreatePoolRequest{
				Repo: repo,
				Name: "pool-" + strings.ReplaceAll(tt.name, " ", "-"),
				Size: tt.size,
			})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("CreatePool size=%d err=%v, want %s", tt.size, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreatePool size=%d: %v", tt.size, err)
			}
			if !pool.OK || pool.Size != tt.wantSize {
				t.Fatalf("pool size=%d, want %d: %#v", pool.Size, tt.wantSize, pool)
			}
		})
	}
}

func TestCreatePoolLeaseTTLEdgeCases(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	tests := []struct {
		name     string
		leaseTTL string
		wantTTL  string
		wantErr  string
	}{
		{name: "default", leaseTTL: "", wantTTL: "15m"},
		{name: "zero", leaseTTL: "0s", wantErr: "lease_ttl_invalid"},
		{name: "negative", leaseTTL: "-5m", wantErr: "lease_ttl_invalid"},
		{name: "accepted", leaseTTL: "15m", wantTTL: "15m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := CreatePool(CreatePoolRequest{
				Repo:     repo,
				Name:     "ttl-" + tt.name,
				LeaseTTL: tt.leaseTTL,
			})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("CreatePool lease=%q err=%v, want %s", tt.leaseTTL, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreatePool lease=%q: %v", tt.leaseTTL, err)
			}
			if pool.LeaseTTL != tt.wantTTL {
				t.Fatalf("LeaseTTL=%q, want %q", pool.LeaseTTL, tt.wantTTL)
			}
		})
	}
}

func TestAddTaskCapsPoolAt4096Tasks(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool, err := CreatePool(CreatePoolRequest{Repo: t.TempDir(), Name: "task-cap"})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4096; i++ {
		if _, err := AddTask(pool.ID, AddTaskRequest{
			Title:        fmt.Sprintf("task %04d", i),
			Instructions: "change one file",
		}); err != nil {
			t.Fatalf("AddTask %d: %v", i, err)
		}
	}
	if _, err := AddTask(pool.ID, AddTaskRequest{Title: "overflow", Instructions: "must fail"}); err == nil || !strings.Contains(err.Error(), "pool_task_cap") {
		t.Fatalf("4097th AddTask err=%v, want pool_task_cap", err)
	}
}

func TestCreatePoolRejectsParentWithoutChildStartPreconditions(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	parent, err := issueops.StartIssueOps(issueops.IssueOpsStateRoot(), issueops.IssueOpsStartRequest{
		Repo:   t.TempDir(),
		Branch: "123-parent-not-ready",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = CreatePool(CreatePoolRequest{
		Repo:          parent.Repo,
		Name:          "blocked-parent",
		ParentCycleID: parent.ID,
	})
	if err == nil || !strings.Contains(err.Error(), "parent_phase_not_implement") || !strings.Contains(err.Error(), "parent_design_review_unapproved") {
		t.Fatalf("CreatePool should reuse child-start parent preconditions, got %v", err)
	}
}

func TestCreatePoolWithReadyParent(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	parent := readyWorkPoolParentCycleForTest(t)

	pool, err := CreatePool(CreatePoolRequest{
		Repo:          parent.Repo,
		Name:          "ready-parent",
		ParentCycleID: parent.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pool.ParentCycleID != parent.ID || pool.Status != "active" {
		t.Fatalf("pool parent/status mismatch: %#v", pool)
	}
}

func readyWorkPoolParentCycleForTest(t *testing.T) issueops.IssueOpsRecord {
	t.Helper()
	repo := t.TempDir()
	parent, err := issueops.StartIssueOps(issueops.IssueOpsStateRoot(), issueops.IssueOpsStartRequest{
		Repo:   repo,
		Branch: "123-parent-ready",
	})
	if err != nil {
		t.Fatal(err)
	}
	parent.Phase = issueops.IssueOpsPhaseImplement
	parent.IssueURL = "https://github.com/example/repo/issues/123"
	parent.PlanPath = filepath.Join(repo, "plans", "parent.md")
	parent.Intent = &issueops.IssueOpsIntentContract{
		RawRequest:        "coordinate workers",
		InterpretedIntent: "coordinate workers",
		SuccessCriteria:   []string{"pool can be created"},
		RecordedAt:        "2026-07-07T00:00:00Z",
	}
	parent.DesignReview = &issueops.IssueOpsDesignReview{
		ProblemSummary: "fan out",
		ProposedDesign: "bounded pool",
		RefactorPlan:   "create tasks",
		Alternatives:   []string{"manual serial work"},
		Risks:          []string{"coordination drift"},
		Verification:   []string{"go test"},
		Approved:       true,
		ReviewedAt:     "2026-07-07T00:00:00Z",
	}
	parent.CompatibilityReview = &issueops.IssueOpsCompatibilityReview{
		BackwardCompatibility: []string{"additive"},
		SideEffects:           []string{"state files"},
		RollbackPlan:          "close pool",
		Verification:          []string{"go test"},
		Approved:              true,
		ReviewedAt:            "2026-07-07T00:00:00Z",
	}
	parent.DevilsAdvocateReview = &issueops.IssueOpsDevilsAdvocateReview{
		Verdict:    "pass",
		RecordedAt: "2026-07-07T00:00:00Z",
	}
	parent, err = issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), parent)
	if err != nil {
		t.Fatal(err)
	}
	return parent
}
