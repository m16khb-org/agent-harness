package issueops

import "testing"

func TestScoreIssueOpsBenchmarkArtifactAcceptsKoreanSectionLabels(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "korean-sections", CriticalFailures: []string{"works in source repo"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.IssueDraft = "## 문제\n\n캐시 미적용으로 동일 입력에 외부 LLM을 반복 호출한다.\n\n## 근거\n\n현재 호출 로그.\n선택 라벨: enhancement(score 0.90), 거절 라벨: documentation(score 0.20), threshold 0.70, 수동 override 없음.\n\n## 수용 기준\n\n동일 입력은 캐시 적중한다.\n\n## 비목표\n\n원격 이슈 자동 생성은 하지 않는다.\n\n## 검증\n\ngo test ./... -count=1\n\n## 피드백 로그\n\nsource/body/분류/후속.\n\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n"
	artifact.PRDraft = "## 의도\n\n이슈의 캐시 요구사항을 충족한다.\n\n## 변경\n\n캐시 저장소 추가.\n\n## 검증\n\ngo test ./... -count=1\n\n## 위험\n\nLLM 점수 변동.\n\n## 리뷰어 노트\n\n한국어 본문 기준.\n\nIssue: https://example.com/acme/agent-harness/issues/1\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n"

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if !score.Passed {
		t.Fatalf("Korean-only section labels should pass deterministic scoring: %+v", score)
	}
}

func completeBenchmarkArtifactForTest() IssueOpsBenchmarkArtifact {
	return IssueOpsBenchmarkArtifact{
		ProblemSummary:         "The request needs measurable IssueOps quality gates before prompt optimization.",
		IssueDraft:             "## Problem\n\n문제 요약\n\n## Current Evidence\n\n현재 근거\n선택 라벨: enhancement(score 0.90), 거절 라벨: documentation(score 0.20), threshold 0.70, 수동 override 없음.\n\n## Acceptance Criteria\n\n완료 기준\n\n## Non-goals\n\n비목표\n\n## Verification\n\n검증\n\n## Feedback Log\n\n피드백 기록\n\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
		Plan:                   "Run: go test ./... -count=1\n",
		TDDPlan:                "Write failing test before implementation.\n",
		TaskBreakdown:          "Worker A owns internal/core/issueops_benchmark.go. Worker B owns cmd/harness/issueops.go. Large issue uses provider-native child work items and records them with issueops link-child.",
		SubagentPrompts:        "You are not alone in the codebase. Do not revert others. Own internal/core only. Expected output: tests and implementation. Before work, report pwd, branch, HEAD, and worktree path; stop on mismatch. For narrow review, use verifier or direct bounded review. If code-reviewer is required, do not spawn subagents and use a 5 minute time budget.",
		PRDraft:                "Intent\n의도\nChanges\n변경사항\nVerification\n검증\nRisk\n위험\nReviewer Notes\n리뷰어 참고\nIssue: https://example.com/acme/agent-harness/issues/1\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
		GuidelineRef:           "docs/superpowers/specs/issueops-issue-pr-guidelines.md",
		PhaseChoices:           "Proceed to plan | revise current phase | jump to issue | pause",
		BranchName:             "feature/1-issueops-quality-benchmark",
		WorktreePath:           "/repo.worktrees/feature-1-issueops-quality-benchmark",
		ImplementationLocation: "/repo.worktrees/feature-1-issueops-quality-benchmark",
		WorktreeCleanup:        "clean worktree; cleanup choices offered after merge",
		DomainContractEvidence: "Invariant: preserve the user-visible contract. Exact mechanism: compare the documented mechanism with source file:line evidence. Equivalent behavior: record when another path enforces the same invariant. Source: internal/core/example.go:12.",
		APIDocGateEvidence:     "Changed endpoint list is reviewed. Public error responses are mapped. Static check: api_doc_static_check. Review: api_doc_review for OpenAPI/Swagger/API doc parity.",
		LiveEvidenceMatrix:     "Environment matrix covers dev, stg, and prod. Repo config evidence is compared with runtime evidence. Remediation order is recorded before edits.",
		ReviewFeedbackEvidence: "Classification: valid defect from Kodus or Gemini Code Assist review-agent feedback. Verification: command and file:line evidence. Thread reply: posted with verdict. Resolution: resolveReviewThread/resolved=true re-checked after fix.",
		CompletionHygiene:      "Draft issue completion record stored with final diff, evidence, labels, children, PR URL, and unresolved follow-ups. Final diff reviewed, target branch verified, remote artifact issue/PR/MR refreshed, single commit policy checked, cleanup status recorded.",
	}
}
