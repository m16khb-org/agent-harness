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
	if !score.Passed || score.AverageScore < 100 {
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
	artifact.PRDraft = "Intent\nChanges\nVerification\nRisk\nIssue: https://example.com/acme/agent-harness/issues/1\n"

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

func TestEvaluateIssueOpsAutoresearchGateKeepsPassingCandidate(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "A candidate with bounded files and no benchmark regression should be kept.",
		TargetDimensions: []string{"issue_quality", "plan_quality"},
		EditSurface:      []string{"skills/issueops/**", "internal/core/issueops_benchmark.go"},
		KeepCriteria:     "no regressions and no critical failures",
		DiscardCriteria:  "discard on benchmark regression or edit-surface violation",
	}
	baseline := issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0)
	next := issueOpsBenchmarkRunForGateTest("candidate", 100, 100, 0)

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  baseline,
		CandidateRun: next,
		ChangedPaths: []string{"skills/issueops/SKILL.md", "internal/core/issueops_benchmark.go"},
	})

	if !result.OK || !result.KeepCandidate || len(result.DiscardReasons) != 0 {
		t.Fatalf("expected gate to keep candidate: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsEditSurfaceViolation(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "A candidate cannot touch files outside the declared surface.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	}

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0),
		CandidateRun: issueOpsBenchmarkRunForGateTest("candidate", 100, 100, 0),
		ChangedPaths: []string{"cmd/harness/issueops.go"},
	})

	if result.KeepCandidate || len(result.EditSurfaceViolations) != 1 || !containsFold(strings.Join(result.DiscardReasons, "\n"), "edit surface") {
		t.Fatalf("expected edit surface discard: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsTargetRegression(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Target dimensions cannot regress.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	}
	baseline := issueOpsBenchmarkRunWithDimensionForGateTest("baseline", "issue_quality", 100)
	next := issueOpsBenchmarkRunWithDimensionForGateTest("candidate", "issue_quality", 90)

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  baseline,
		CandidateRun: next,
		ChangedPaths: []string{"skills/issueops/SKILL.md"},
	})

	if result.KeepCandidate || len(result.TargetDimensionRegressions) != 1 {
		t.Fatalf("expected target dimension regression discard: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsUnknownTargetDimension(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Target dimensions must be known benchmark dimensions.",
		TargetDimensions: []string{"issue_qualit"},
		EditSurface:      []string{"skills/issueops/**"},
	}

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0),
		CandidateRun: issueOpsBenchmarkRunForGateTest("candidate", 100, 100, 0),
		ChangedPaths: []string{"skills/issueops/SKILL.md"},
	})

	if result.KeepCandidate || !containsFold(strings.Join(result.DiscardReasons, "\n"), "invalid target dimension") {
		t.Fatalf("expected unknown target dimension discard: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsNonPassingCandidateRun(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Candidate benchmark must pass.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	}

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0),
		CandidateRun: issueOpsBenchmarkRunForGateTest("candidate", 90, 90, 1),
		ChangedPaths: []string{"skills/issueops/SKILL.md"},
	})

	if result.KeepCandidate || !containsFold(strings.Join(result.DiscardReasons, "\n"), "candidate benchmark did not pass") {
		t.Fatalf("expected non-passing benchmark discard: %+v", result)
	}
}

func issueOpsBenchmarkRunForGateTest(id string, average, minimum float64, criticalFailures int) IssueOpsBenchmarkRunResult {
	score := IssueOpsBenchmarkScore{
		OK:           criticalFailures == 0 && minimum >= 100,
		FixtureID:    "fixture",
		AverageScore: average,
		MinimumScore: minimum,
		DimensionScores: []IssueOpsDimensionScore{
			{Dimension: "issue_quality", Score: minimum, Evidence: "gate test"},
			{Dimension: "plan_quality", Score: minimum, Evidence: "gate test"},
		},
		Passed: criticalFailures == 0 && minimum >= 100,
	}
	for i := 0; i < criticalFailures; i++ {
		score.CriticalFailures = append(score.CriticalFailures, "critical failure")
	}
	return FinalizeIssueOpsBenchmarkRunResult(IssueOpsBenchmarkRunResult{ID: id, Scores: []IssueOpsBenchmarkScore{score}})
}

func issueOpsBenchmarkRunWithDimensionForGateTest(id, dimension string, scoreValue float64) IssueOpsBenchmarkRunResult {
	score := IssueOpsBenchmarkScore{
		OK:           scoreValue >= 100,
		FixtureID:    "fixture",
		AverageScore: scoreValue,
		MinimumScore: scoreValue,
		DimensionScores: []IssueOpsDimensionScore{
			{Dimension: dimension, Score: scoreValue, Evidence: "gate test"},
		},
		Passed: scoreValue >= 100,
	}
	return FinalizeIssueOpsBenchmarkRunResult(IssueOpsBenchmarkRunResult{ID: id, Scores: []IssueOpsBenchmarkScore{score}})
}

func TestScoreIssueOpsBenchmarkArtifactAcceptsKoreanSectionLabels(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "korean-sections", CriticalFailures: []string{"works in source repo"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.IssueDraft = "## 문제\n\n캐시 미적용으로 동일 입력에 외부 LLM을 반복 호출한다.\n\n## 근거\n\n현재 호출 로그.\n\n## 수용 기준\n\n동일 입력은 캐시 적중한다.\n\n## 비목표\n\n원격 이슈 자동 생성은 하지 않는다.\n\n## 검증\n\ngo test ./... -count=1\n\n## 피드백 로그\n\nsource/body/분류/후속.\n\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n"
	artifact.PRDraft = "## 의도\n\n이슈의 캐시 요구사항을 충족한다.\n\n## 변경\n\n캐시 저장소 추가.\n\n## 검증\n\ngo test ./... -count=1\n\n## 위험\n\nLLM 점수 변동.\n\n## 리뷰어 노트\n\n한국어 본문 기준.\n\nIssue: https://example.com/acme/agent-harness/issues/1\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n"

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if !score.Passed {
		t.Fatalf("Korean-only section labels should pass deterministic scoring: %+v", score)
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
		PRDraft:                "Intent\n의도\nChanges\n변경사항\nVerification\n검증\nRisk\n위험\nReviewer Notes\n리뷰어 참고\nIssue: https://example.com/acme/agent-harness/issues/1\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
		GuidelineRef:           "docs/superpowers/specs/issueops-issue-pr-guidelines.md",
		PhaseChoices:           "Proceed to plan | revise current phase | jump to issue | pause",
		BranchName:             "feature/1-issueops-quality-benchmark",
		WorktreePath:           "/repo.worktrees/feature-1-issueops-quality-benchmark",
		ImplementationLocation: "/repo.worktrees/feature-1-issueops-quality-benchmark",
		WorktreeCleanup:        "clean worktree; cleanup choices offered after merge",
	}
}
