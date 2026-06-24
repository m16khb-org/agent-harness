package benchmark

import (
	"strings"
	"testing"
)

func TestScoreIssueOpsBenchmarkArtifactAcceptsKoreanSectionLabels(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "korean-sections", CriticalFailures: []string{"works in source repo"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.IssueDraft = "## 문제\n\n캐시 미적용으로 동일 입력에 외부 LLM을 반복 호출한다.\n\n## 근거\n\n현재 호출 로그.\n\n## 관련 이슈/라벨 판단\n\n선택 라벨: enhancement(score 0.90), 거절 라벨: documentation(score 0.20), threshold 0.70, 수동 override 없음.\n\n## 수용 기준\n\n동일 입력은 캐시 적중한다.\n\n## 비목표\n\n원격 이슈 자동 생성은 하지 않는다.\n\n## 구현 범위\n\n캐시 저장과 wrapper 호출부만 바꾼다.\n\n## 검증\n\ngo test ./... -count=1\n\n## 위험과 트레이드오프\n\n캐시 키 drift 위험을 검증으로 관리한다.\n\n## 피드백 로그\n\nsource/body/분류/후속.\n\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n"
	artifact.PRDraft = "## 의도\n\n이슈의 캐시 요구사항을 충족한다.\n\n## 이슈\n\nIssue: https://example.com/acme/agent-harness/issues/1\n\n## 변경\n\n캐시 저장소 추가.\n\n## 검증\n\ngo test ./... -count=1\n\n## 리뷰어 초점\n\n한국어 본문 기준.\n\n## 위험/rollback\n\nLLM 점수 변동. rollback은 캐시 호출 제거.\n\n## 사용자 영향/릴리즈 노트\n\n동일 입력의 외부 호출 감소.\n\n## 문서/마이그레이션\n\n운영 문서 갱신.\n\n## 범위 관리\n\n캐시 경계만 수정.\n\n## 워크트리 정리\n\ncleanup status 확인.\n\n## 자동화/AI 개입 근거\n\nrenderer와 remote gate 결과를 기준으로 본문을 생성했다.\n\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n"

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if !score.Passed {
		t.Fatalf("Korean-only section labels should pass deterministic scoring: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresCanonicalRemoteIssueSections(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "canonical-issue", CriticalFailures: []string{"works in source repo"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.IssueDraft = strings.ReplaceAll(artifact.IssueDraft, "## 구현 범위\n\ncore renderer, CLI, MCP schema를 갱신한다.\n\n", "")

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if score.Passed {
		t.Fatalf("issue quality must fail when canonical implementation scope section is missing: %+v", score)
	}
	if row := adequacyRow(score, "issue_quality"); row.Score != 0 {
		t.Fatalf("issue_quality row must drop, got %+v", row)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresCanonicalPRSections(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "canonical-pr", CriticalFailures: []string{"works in source repo"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.PRDraft = strings.ReplaceAll(artifact.PRDraft, "## 자동화/AI 개입 근거\n\nrenderer와 remote gate 결과를 기준으로 본문을 생성했다.\n\n", "")

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if score.Passed {
		t.Fatalf("PR/MR quality must fail when canonical automation evidence section is missing: %+v", score)
	}
	if row := adequacyRow(score, "pr_mr_quality"); row.Score != 0 {
		t.Fatalf("pr_mr_quality row must drop, got %+v", row)
	}
}

func completeBenchmarkArtifactForTest() IssueOpsBenchmarkArtifact {
	return IssueOpsBenchmarkArtifact{
		ProblemSummary:         "The request needs measurable IssueOps quality gates before prompt optimization.",
		IssueDraft:             "## 문제\n\n문제 요약\n\n## 현재 근거\n\n현재 근거\n\n## 관련 이슈/라벨 판단\n\n선택 라벨: enhancement(score 0.90), 거절 라벨: documentation(score 0.20), threshold 0.70, 수동 override 없음.\n\n## 완료 기준\n\n완료 기준\n\n## 비목표\n\n비목표\n\n## 구현 범위\n\ncore renderer, CLI, MCP schema를 갱신한다.\n\n## 검증\n\n검증\n\n## 위험과 트레이드오프\n\ncontract drift 위험을 golden으로 관리한다.\n\n## 피드백 기록\n\n피드백 기록\n\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
		Plan:                   "Run: go test ./... -count=1\n",
		TDDPlan:                "Write failing test before implementation.\n",
		TaskBreakdown:          "Worker A owns internal/core/issueops_benchmark.go. Worker B owns cmd/harness/issueops.go. Large issue uses provider-native child work items and records them with issueops link-child.",
		SubagentPrompts:        "You are not alone in the codebase. Do not revert others. Own internal/core only. Expected output: tests and implementation. Before work, report pwd, branch, HEAD, and worktree path; stop on mismatch. For narrow review, use verifier or direct bounded review. If code-reviewer is required, do not spawn subagents and use a 5 minute time budget.",
		PRDraft:                "## 의도\n\n의도\n\n## 이슈\n\nIssue: https://example.com/acme/agent-harness/issues/1\n\n## 변경 사항\n\n변경사항\n\n## 검증\n\n검증\n\n## 리뷰어 초점\n\n리뷰어 참고\n\n## 위험/rollback\n\n위험과 rollback\n\n## 사용자 영향/릴리즈 노트\n\n사용자 영향 없음\n\n## 문서/마이그레이션\n\n문서 갱신\n\n## 범위 관리\n\n범위 제한\n\n## 워크트리 정리\n\ncleanup status\n\n## 자동화/AI 개입 근거\n\nrenderer와 remote gate 결과를 기준으로 본문을 생성했다.\n\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
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
		PioneerSkillEvidence:   "Durable state record: issueops state id and readiness gates recorded\nPhase routing: problem issue plan implement feedback pr cleanup\nFlow evidence: issue plan TDD subagent decision feedback PR linked\nHook boundary: hooks do not create issues edit files or run tests\nCleanup/readiness evidence: strict readiness and cleanup choices recorded",
	}
}
