package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
)

func preflightGitForReviewTest(dir string, args ...string) (int, string, string) {
	return preflight.GitCmd(dir, args...)
}

func TestRecordIssueOpsImplementationReviewValidation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := gitInitedRepoForReviewTest(t)
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "83-review"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := RecordIssueOpsImplementationReview(stateRoot, record.ID, IssueOpsImplementationReviewRequest{Verdict: "approve"}); err == nil {
		t.Fatal("unknown verdict must be rejected")
	}
	if _, err := RecordIssueOpsImplementationReview(stateRoot, record.ID, IssueOpsImplementationReviewRequest{Verdict: "pass"}); err == nil {
		t.Fatal("pass without findings/evidence must be rejected")
	}
	valid := IssueOpsImplementationReviewRequest{
		Verdict: "pass", Findings: []string{"경계 조건 검토 완료"}, Evidence: []string{"go test ./... ok"},
		ReviewerHost: "codex", ReviewerModel: "gpt-5.6-sol", ReviewerEffort: "xhigh",
	}
	// C4b-F2: implement 이전 phase에서는 기록을 거부한다.
	if _, err := RecordIssueOpsImplementationReview(stateRoot, record.ID, valid); err == nil || !strings.Contains(err.Error(), "implement phase") {
		t.Fatalf("pre-implement recording must be rejected: %v", err)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) { rec.Phase = IssueOpsPhaseImplement })
	got, err := RecordIssueOpsImplementationReview(stateRoot, record.ID, valid)
	if err != nil {
		t.Fatal(err)
	}
	review := got.ImplementationReview
	if review == nil || review.Verdict != "pass" || review.ReviewerModel != "gpt-5.6-sol" {
		t.Fatalf("review must round-trip with audit fields: %+v", review)
	}
	// C4b-F1: 리뷰가 변경 집합 fingerprint를 봉인한다.
	if review.ReviewedFingerprint == "" {
		t.Fatalf("review must bind the reviewed change fingerprint: %+v", review)
	}
}

// gitInitedRepoForReviewTest는 변경 fingerprint가 계산 가능한 최소 git repo를
// 만든다(미추적 파일 1개 = 변경 집합).
func gitInitedRepoForReviewTest(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		if code, _, stderr := preflightGitForReviewTest(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "change.go"), []byte("package x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

// AC-05: orca 모드에만 fail-closed로 적용되고 direct 모드는 게이트 대상이 아니다.
func TestImplementationReviewMissingScopedToOrcaMode(t *testing.T) {
	record := IssueOpsRecord{}
	if got := implementationReviewMissing(record, ""); got != "" {
		t.Fatalf("record without execution must not be gated: %q", got)
	}
	record.Execution = &Execution{Mode: model.ExecutionModeDirect}
	if got := implementationReviewMissing(record, ""); got != "" {
		t.Fatalf("direct mode must not be gated: %q", got)
	}
	record.Execution.Mode = model.ExecutionModeOrca
	if got := implementationReviewMissing(record, ""); got != "implementation_review" {
		t.Fatalf("orca mode without review must be gated: %q", got)
	}
	record.ImplementationReview = &model.IssueOpsImplementationReview{Verdict: "revise"}
	if got := implementationReviewMissing(record, ""); got != "implementation_review_verdict_revise" {
		t.Fatalf("non-pass verdict must be gated with its verdict: %q", got)
	}
	record.ImplementationReview.Verdict = "pass"
	if got := implementationReviewMissing(record, ""); got != "" {
		t.Fatalf("pass verdict must clear the gate: %q", got)
	}
	// C4b-F1: fingerprint가 현재 변경 집합과 다르면 stale로 거부.
	record.ImplementationReview.ReviewedFingerprint = "old"
	if got := implementationReviewMissing(record, "new"); got != "implementation_review_stale" {
		t.Fatalf("drifted fingerprint must be stale: %q", got)
	}
	record.ImplementationReview.ReviewedFingerprint = "new"
	if got := implementationReviewMissing(record, "new"); got != "" {
		t.Fatalf("matching fingerprint must clear the gate: %q", got)
	}
}

func TestStrictPRReadinessSurfacesImplementationReview(t *testing.T) {
	record := IssueOpsRecord{Execution: &Execution{Mode: model.ExecutionModeOrca}}
	ready := IssueOpsPRReadiness(record)
	if !containsString(ready.Missing, "implementation_review") {
		t.Fatalf("strict readiness must surface the implementation review gate: %+v", ready.Missing)
	}
}

func TestOwnerCommandsIncludeImplementationReviewWithPlannerModel(t *testing.T) {
	repo := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "83-review")
	record := IssueOpsRecord{
		ID: "io-000000000083", Repo: repo,
		BranchPrepare: &IssueOpsBranchPrepare{Provider: "github", BaseBranch: "main"},
		Execution: &Execution{
			Mode:      model.ExecutionModeOrca,
			Workspace: Workspace{SourceRoot: repo, Root: worktree, Branch: "83-review", BaseHead: "deadbeef"},
			Lease:     WriteLease{Generation: 1},
		},
	}
	commands := executionOwnerCommandsFor(record, ExecutionPrepareRequest{OwnerHost: "codex"}, strings.Repeat("a", 64))
	if !strings.Contains(commands.ImplementationReview, "implementation-review record") ||
		!strings.Contains(commands.ImplementationReview, "gpt-5.6-sol") ||
		!strings.Contains(commands.ImplementationReview, "--reviewer-effort 'xhigh'") {
		t.Fatalf("owner command must pin the planner reviewer model: %s", commands.ImplementationReview)
	}
	if err := validateExecutionOwnerCatalog(commands); err != nil {
		t.Fatalf("implementation review command must match the catalog: %v", err)
	}
}
