package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadIssueOpsBenchmarkFixtures(t *testing.T) {
	fixtures, err := LoadIssueOpsBenchmarkFixtures(filepath.Join("..", "..", "testdata", "issueops", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) < 2 {
		t.Fatalf("expected at least two fixtures, got %d", len(fixtures))
	}
	for _, fixture := range fixtures {
		if fixture.ID == "" || fixture.Title == "" || fixture.UserPrompt == "" {
			t.Fatalf("fixture missing required fields: %+v", fixture)
		}
		if len(fixture.CriticalFailures) == 0 {
			t.Fatalf("fixture %s should define critical failures", fixture.ID)
		}
	}
}

func TestScoreIssueOpsBenchmarkArtifactDeterministic(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "worktree", CriticalFailures: []string{"works in source repo"}}
	artifact := completeBenchmarkArtifactForTest()

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if !score.Passed || score.AverageScore < 5 {
		t.Fatalf("expected complete artifact to pass: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactDetectsCriticalFailures(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "worktree", CriticalFailures: []string{"works in source repo"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.ImplementationLocation = "/repo"

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if score.Passed {
		t.Fatalf("expected source repo implementation to fail: %+v", score)
	}
	if len(score.CriticalFailures) == 0 {
		t.Fatalf("expected critical failures: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresKoreanIssueAndPR(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "korean", CriticalFailures: []string{"issue or pr/mr not written in Korean"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.IssueDraft = "## Problem\n\n## Current Evidence\n\n## Acceptance Criteria\n\n## Non-goals\n\n## Verification\n\n## Feedback Log\n"
	artifact.PRDraft = "Intent\nChanges\nVerification\nRisk\nIssue: https://github.com/m16khb/agent-harness/issues/1\n"

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if score.Passed {
		t.Fatalf("expected English-only issue/PR output to fail: %+v", score)
	}
	if len(score.CriticalFailures) == 0 {
		t.Fatalf("expected Korean critical failure: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresGuidelineReference(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "guideline", CriticalFailures: []string{"missing issue/pr guideline reference"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.GuidelineRef = ""
	artifact.IssueDraft = strings.ReplaceAll(artifact.IssueDraft, "Guideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n", "")
	artifact.PRDraft = strings.ReplaceAll(artifact.PRDraft, "Guideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n", "")

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if score.Passed {
		t.Fatalf("expected missing guideline reference to fail: %+v", score)
	}
	if len(score.CriticalFailures) == 0 {
		t.Fatalf("expected guideline critical failure: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRejectsExcessiveEmoji(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "emoji", CriticalFailures: []string{"excessive emoji in issue or pr/mr"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.PRDraft += "😀😀😀😀"

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if score.Passed {
		t.Fatalf("expected excessive emoji to fail: %+v", score)
	}
	if len(score.CriticalFailures) == 0 {
		t.Fatalf("expected emoji critical failure: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresWorkerContextGate(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "worker-context", CriticalFailures: []string{"worker starts without context check"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.SubagentPrompts = "You are not alone in the codebase. Do not revert others. Own internal/core only. Expected output: tests and implementation."

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if score.Passed {
		t.Fatalf("expected missing worker context gate to fail: %+v", score)
	}
	if len(score.CriticalFailures) == 0 {
		t.Fatalf("expected worker context critical failure: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresBoundedReviewPrompt(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "review", CriticalFailures: []string{"unbounded code-reviewer review"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.SubagentPrompts = "You are not alone in the codebase. Do not revert others. Own internal/core only. Expected output: review report. Before work, report pwd, branch, HEAD, worktree, and stop on mismatch. Use code-reviewer for this review."

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if score.Passed {
		t.Fatalf("expected unbounded code-reviewer prompt to fail: %+v", score)
	}
	if len(score.CriticalFailures) == 0 {
		t.Fatalf("expected bounded review critical failure: %+v", score)
	}
}

func TestRunAndCompareIssueOpsBenchmark(t *testing.T) {
	dir := t.TempDir()
	fixtures := []IssueOpsBenchmarkFixture{
		{ID: "fixture", Title: "Fixture", UserPrompt: "prompt", RepoContext: "ctx", CriticalFailures: []string{"works in source repo"}},
	}

	baseline, err := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{
		StateRoot: dir,
		Fixtures:  fixtures,
		Artifacts: map[string]IssueOpsBenchmarkArtifact{
			"fixture": {},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{
		StateRoot: dir,
		Fixtures:  fixtures,
		Artifacts: map[string]IssueOpsBenchmarkArtifact{
			"fixture": completeBenchmarkArtifactForTest(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	compare := CompareIssueOpsBenchmarkRuns(baseline, candidate)
	if !compare.Improved || compare.AverageScoreDelta <= 0 {
		t.Fatalf("expected candidate improvement: %+v", compare)
	}
}

func completeBenchmarkArtifactForTest() IssueOpsBenchmarkArtifact {
	return IssueOpsBenchmarkArtifact{
		ProblemSummary:         "The request needs measurable IssueOps quality gates before prompt optimization.",
		IssueDraft:             "## Problem\n\n문제 요약\n\n## Current Evidence\n\n현재 근거\n\n## Acceptance Criteria\n\n완료 기준\n\n## Non-goals\n\n비목표\n\n## Verification\n\n검증\n\n## Feedback Log\n\n피드백 기록\n\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
		Plan:                   "Run: go test ./... -count=1\n",
		TDDPlan:                "Write failing test before implementation.\n",
		TaskBreakdown:          "Worker A owns internal/core/issueops_benchmark.go. Worker B owns cmd/harness/issueops.go.",
		SubagentPrompts:        "You are not alone in the codebase. Do not revert others. Own internal/core only. Expected output: tests and implementation. Before work, report pwd, branch, HEAD, and worktree path; stop on mismatch. For narrow review, use verifier or direct bounded review. If code-reviewer is required, do not spawn subagents and use a 5 minute time budget.",
		PRDraft:                "Intent\n의도\nChanges\n변경사항\nVerification\n검증\nRisk\n위험\nReviewer Notes\n리뷰어 참고\nIssue: https://github.com/m16khb/agent-harness/issues/1\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
		GuidelineRef:           "docs/superpowers/specs/issueops-issue-pr-guidelines.md",
		PhaseChoices:           "Proceed to plan | revise current phase | jump to issue | pause",
		BranchName:             "feature/1-issueops-quality-benchmark",
		WorktreePath:           "/repo.worktrees/feature-1-issueops-quality-benchmark",
		ImplementationLocation: "/repo.worktrees/feature-1-issueops-quality-benchmark",
		WorktreeCleanup:        "clean worktree; cleanup choices offered after merge",
	}
}
