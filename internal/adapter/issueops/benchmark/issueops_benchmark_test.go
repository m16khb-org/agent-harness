package benchmark

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadIssueOpsBenchmarkFixtures(t *testing.T) {
	fixtures, err := LoadIssueOpsBenchmarkFixtures(filepath.Join("..", "..", "..", "..", "testdata", "issueops", "fixtures"))
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

func TestScoreIssueOpsBenchmarkArtifactIncludesEvidenceContractDimensions(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "evidence-contract", CriticalFailures: []string{"skips verification"}}
	artifact := completeBenchmarkArtifactForTest()

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	for _, dimension := range []string{
		"domain_contract_quality",
		"api_doc_gate_quality",
		"live_evidence_quality",
		"review_feedback_accountability",
		"completion_hygiene_quality",
	} {
		found := false
		for _, dimensionScore := range score.DimensionScores {
			if dimensionScore.Dimension == dimension {
				found = true
				if dimensionScore.Score < 100 {
					t.Fatalf("expected %s to pass for complete artifact: %+v", dimension, score)
				}
			}
		}
		if !found {
			t.Fatalf("expected score to include %s dimension: %+v", dimension, score)
		}
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresIssueOpsQualityUpgradeEvidence(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "quality-upgrade", CriticalFailures: []string{
		"skips intelligent label scoring",
		"flattens large issue hierarchy",
		"skips draft issue completion record",
		"reports review feedback cleared without resolving review-agent threads",
	}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.IssueDraft = strings.ReplaceAll(artifact.IssueDraft, "선택 라벨: enhancement(score 0.90), 거절 라벨: documentation(score 0.20), threshold 0.70, 수동 override 없음.\n", "")
	artifact.TaskBreakdown = "Worker A owns internal/core only."
	artifact.CompletionHygiene = strings.ReplaceAll(artifact.CompletionHygiene, "Draft issue completion record stored with final diff, evidence, labels, children, PR URL, and unresolved follow-ups. ", "")
	artifact.ReviewFeedbackEvidence = "Classification: valid defect. Verification: command and file:line evidence. Thread reply: posted with verdict."

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	for _, want := range fixture.CriticalFailures {
		if !containsString(score.CriticalFailures, want) {
			t.Fatalf("expected critical failure %q in %+v", want, score)
		}
	}
}

func TestScoreIssueOpsBenchmarkArtifactAcceptsSmallIssueNoSplitRationale(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "small-no-split", CriticalFailures: []string{"flattens large issue hierarchy"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.TaskBreakdown = "Single worker owns internal/core only. Task stays as one directly executable issue. 비분할 사유: acceptance criteria share one implementation boundary, no independent child tasks, and verification is one focused go test run."

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if containsString(score.CriticalFailures, "flattens large issue hierarchy") {
		t.Fatalf("small issue with explicit no-split rationale must satisfy hierarchy gate: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRejectsRoutineChildSplitWithoutLargeOrCollaborationRationale(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "routine-split", CriticalFailures: []string{"flattens large issue hierarchy"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.TaskBreakdown = "Worker A owns tests. Worker B owns implementation. Uses provider-native child work items and records them with issueops link-child."

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if !containsString(score.CriticalFailures, "flattens large issue hierarchy") {
		t.Fatalf("routine child split without large-risk or collaboration rationale must fail: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactAcceptsSplitForLargeUnsafeOrCollaborativeIssue(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "justified-split", CriticalFailures: []string{"flattens large issue hierarchy"}}

	for name, taskBreakdown := range map[string]string{
		"large unsafe":  "Worker A owns routing. Worker B owns migration. Large issue is unsafe as one work item because one issue would hide risky behavior changes. Uses provider-native child work items and records them with issueops link-child. Execution order: Wave 1 [p] routing is parallelizable with docs; Wave 2 [s] migration is sequential and depends on routing.",
		"collaboration": "Worker A owns API docs. Worker B owns runtime verification. Split explicitly requested for collaboration and parallel ownership. Uses provider-native child work items and records them with issueops link-child. Execution wave 1: [p] API docs and test inventory can run in parallel. Execution wave 2: [s] runtime verification is sequential and requires the inventory prerequisite.",
	} {
		t.Run(name, func(t *testing.T) {
			artifact := completeBenchmarkArtifactForTest()
			artifact.TaskBreakdown = taskBreakdown

			score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
			if containsString(score.CriticalFailures, "flattens large issue hierarchy") {
				t.Fatalf("justified split must satisfy hierarchy gate: %+v", score)
			}
		})
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresChildTaskExecutionDependencyClassification(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "dependency-classification", CriticalFailures: []string{"flattens large issue hierarchy"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.TaskBreakdown = "Worker A owns routing. Worker B owns migration. Large issue is unsafe as one work item because one issue would hide risky behavior changes. Uses provider-native child work items and records them with issueops link-child."

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if !containsString(score.CriticalFailures, "flattens large issue hierarchy") {
		t.Fatalf("split child tasks without parallel/sequential dependency classification must fail: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresChildTaskMarkers(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "dependency-markers", CriticalFailures: []string{"flattens large issue hierarchy"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.TaskBreakdown = "Worker A owns routing. Worker B owns migration. Large issue is unsafe as one work item because one issue would hide risky behavior changes. Uses provider-native child work items and records them with issueops link-child. Execution order: Wave 1 routing is parallelizable with docs; Wave 2 migration is sequential and depends on routing."

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if !containsString(score.CriticalFailures, "flattens large issue hierarchy") {
		t.Fatalf("split child tasks without [p]/[s] markers must fail: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactAcceptsAllParallelSplitWithoutSequentialMarker(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "all-parallel-split", CriticalFailures: []string{"flattens large issue hierarchy"}}
	artifact := completeBenchmarkArtifactForTest()
	// Parallelizable-by-default policy: a split with only [p] children and no
	// [s] marker is valid when every child can start and verify independently.
	artifact.TaskBreakdown = "Worker A owns docs. Worker B owns tests. Worker C owns prompt evaluation. Large issue is unsafe as one work item because one issue would hide risky behavior changes. Uses provider-native child work items and records them with issueops link-child. Execution wave 1: [p] docs is parallelizable, prerequisite none; [p] tests is parallelizable, prerequisite none; [p] prompt evaluation is parallelizable, prerequisite none. No child depends on another child's output. Execution order: all children run in parallel in wave 1."

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if containsString(score.CriticalFailures, "flattens large issue hierarchy") {
		t.Fatalf("all-parallel split without [s] marker must satisfy hierarchy gate under parallel-by-default policy: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactDetectsEvidenceCriticalFailures(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "evidence-critical", CriticalFailures: []string{
		"skips domain contract evidence",
		"skips api doc gate",
		"skips live evidence matrix",
		"skips review feedback accountability",
		"skips completion hygiene",
	}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.DomainContractEvidence = ""
	artifact.APIDocGateEvidence = ""
	artifact.LiveEvidenceMatrix = ""
	artifact.ReviewFeedbackEvidence = ""
	artifact.CompletionHygiene = ""

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	for _, want := range fixture.CriticalFailures {
		if !containsString(score.CriticalFailures, want) {
			t.Fatalf("expected critical failure %q in %+v", want, score)
		}
	}
}
