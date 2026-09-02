package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

// gitRepoWithProjectDocsForTest는 committed 문서 하나(변경 집합 밖)와
// untracked 변경 둘(구현 파일, 운영 문서)을 가진 최소 repo를 만든다.
func gitRepoWithProjectDocsForTest(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		if code, _, stderr := preflightGitForReviewTest(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeRepoFileForTest(t, repo, ".agent-harness/ADR.md", "# adr\n")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "base"}} {
		if code, _, stderr := preflightGitForReviewTest(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeRepoFileForTest(t, repo, "change.go", "package x\n")
	writeRepoFileForTest(t, repo, ".agent-harness/CAUTIONS.md", "# cautions\n")
	return repo
}

func writeRepoFileForTest(t *testing.T, repo, rel, body string) {
	t.Helper()
	abs := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRecordIssueOpsProjectDocsReviewValidation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := gitRepoWithProjectDocsForTest(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "90-docs"})
	if err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) { rec.Phase = IssueOpsPhaseImplement })

	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{Verdict: "done"}); err == nil {
		t.Fatal("unknown verdict must be rejected")
	}
	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{Verdict: "no-change"}); err == nil {
		t.Fatal("verdict without evidence must be rejected")
	}
	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "updated", Evidence: []string{"CAUTIONS 확인"},
	}); err == nil {
		t.Fatal("updated verdict without a doc path must be rejected")
	}
	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "no-change", Docs: []string{".agent-harness/CAUTIONS.md"}, Evidence: []string{"확인함"},
	}); err == nil {
		t.Fatal("no-change verdict must not carry updated docs")
	}
	// 연극 방지: 변경 집합에 없는 문서를 갱신했다고 주장하면 거부한다.
	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "updated", Docs: []string{".agent-harness/ADR.md"}, Evidence: []string{"ADR 갱신"},
	}); err == nil || !strings.Contains(err.Error(), "change set") {
		t.Fatalf("doc outside the change set must be rejected: %v", err)
	}
	got, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "updated", Docs: []string{".agent-harness/CAUTIONS.md"}, Evidence: []string{"재발 함정 기록"},
	})
	if err != nil {
		t.Fatal(err)
	}
	review := got.ProjectDocsReview
	if review == nil || review.Verdict != "updated" || len(review.Docs) != 1 {
		t.Fatalf("review must round-trip: %+v", review)
	}
	if review.ReviewedFingerprint == "" {
		t.Fatalf("review must bind the reviewed change fingerprint: %+v", review)
	}
}

func TestRecordIssueOpsProjectDocsReviewRejectsPreImplementPhase(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := gitRepoWithProjectDocsForTest(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "91-docs"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "no-change", Evidence: []string{"문서 영향 없음"},
	})
	if err == nil || !strings.Contains(err.Error(), "implement phase") {
		t.Fatalf("pre-implement recording must be rejected: %v", err)
	}
}

// project-docs 게이트는 implementation review와 달리 direct/orca 양쪽에 걸린다.
func TestProjectDocsReviewMissingAppliesToBothModes(t *testing.T) {
	record := issueops.IssueOpsRecord{Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect}}
	if got := projectDocsReviewMissing(record, ""); got != "project_docs_review" {
		t.Fatalf("direct mode must also be gated: %q", got)
	}
	record.Execution.Mode = issueops.ExecutionModeOrca
	if got := projectDocsReviewMissing(record, ""); got != "project_docs_review" {
		t.Fatalf("orca mode must be gated: %q", got)
	}
	record.ProjectDocsReview = &issueops.IssueOpsProjectDocsReview{Verdict: "no-change"}
	if got := projectDocsReviewMissing(record, ""); got != "" {
		t.Fatalf("recorded review must clear the gate: %q", got)
	}
	record.ProjectDocsReview.ReviewedFingerprint = "old"
	if got := projectDocsReviewMissing(record, "new"); got != "project_docs_review_stale" {
		t.Fatalf("drifted fingerprint must be stale: %q", got)
	}
	record.ProjectDocsReview.ReviewedFingerprint = "new"
	if got := projectDocsReviewMissing(record, "new"); got != "" {
		t.Fatalf("matching fingerprint must clear the gate: %q", got)
	}
}

func TestPRReadinessSurfacesProjectDocsReview(t *testing.T) {
	record := issueops.IssueOpsRecord{Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect}}
	if ready := IssueOpsPRReadiness(record); !containsString(ready.Missing, "project_docs_review") {
		t.Fatalf("PR readiness must surface the project docs gate: %+v", ready.Missing)
	}
}
