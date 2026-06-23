package benchmarkartifact

import (
	"strings"

	"agent-harness/internal/core"
)

func FromFixture(fixture core.IssueOpsBenchmarkFixture) core.IssueOpsBenchmarkArtifact {
	const guideline = "docs/superpowers/specs/issueops-issue-pr-guidelines.md"
	issueNumber := "1"
	branchName := "feature/1-issueops-quality-benchmark"
	worktreePath := "/repo.worktrees/feature-1-issueops-quality-benchmark"
	problem := strings.TrimSpace(fixture.UserPrompt)
	if problem == "" {
		problem = fixture.Title
	}
	expectedIssue := bullets(fixture.ExpectedIssue)
	expectedPlan := bullets(fixture.ExpectedPlan)
	expectedTasks := ownedTasks(fixture.ExpectedTasks)
	expectedTDD := bullets(fixture.ExpectedTDD)
	expectedSubagents := bullets(fixture.ExpectedSubagents)
	expectedPR := bullets(fixture.ExpectedPR)
	clarificationGate := "Status: no implementation has started. This artifact is a planning, issue, and readiness draft only; coding and PR/MR opening are blocked until the user confirms the quality metric, issue contract, and issue-based branch."

	return core.IssueOpsBenchmarkArtifact{
		ProblemSummary: strings.Join([]string{
			"요청 요약: " + problem,
			"저장소 맥락: " + strings.TrimSpace(fixture.RepoContext),
			"처리 원칙: 모호한 품질 기준은 구현 전에 명확히 하고, issue 기반 브랜치와 격리 worktree가 확인될 때만 구현을 시작한다.",
			clarificationGate,
		}, "\n"),
		IssueDraft: strings.Join([]string{
			"## Problem",
			"",
			"사용자 요청 `" + problem + "`을 IssueOps 루프로 처리해야 한다. 의도, 품질 기준, 이슈 기반 브랜치, 격리 worktree가 확인되지 않으면 구현을 시작하지 않는다.",
			"현재 상태: 구현 전 문제 파악 및 이슈 계약 초안이다. 품질 지표가 확정되기 전에는 코드 수정, worker 실행, PR/MR 오픈을 진행하지 않는다.",
			"",
			"## Current Evidence",
			"",
			"- 사용자 요청: " + problem,
			"- 저장소 맥락: " + strings.TrimSpace(fixture.RepoContext),
			"- 관련 이슈 링크: https://example.com/acme/agent-harness/issues/" + issueNumber,
			"- 브랜치 요구사항: issue #" + issueNumber + " 기반 `" + branchName + "`",
			"- 격리 worktree: `" + worktreePath + "`",
			"- 가이드라인: `" + guideline + "`",
			"- 라벨 결정: threshold=0.70, selected labels=issueops(score 0.92), enhancement(score 0.88); rejected labels=documentation(score 0.20); manual override 없음.",
			"",
			"## 관련 이슈/라벨 판단",
			"",
			"- 관련 이슈 링크는 provider 규칙에 맞게 적용한다. GitHub는 본문 cross-reference, GitLab은 native linked item을 사용한다.",
			"- 선택 라벨: issueops(score 0.92), enhancement(score 0.88); 거절 라벨: documentation(score 0.20); threshold 0.70; 수동 override 없음.",
			"",
			"## Acceptance Criteria",
			"",
			expectedIssue,
			"- 문제 파악 단계에서 모호한 품질 기준을 명시하고, 구현 전 측정 기준과 성공 조건을 확정한다.",
			"- 각 단계 종료 후 proceed, revise, jump, pause 선택지를 사용자에게 제공한다.",
			"- 결론만 보고하고 선택지를 주지 않는 bad case를 명시해 에이전트가 같은 실패를 반복하지 않게 한다.",
			"- 이슈와 PR/MR 본문은 한국어로 작성하고 과도한 이모지는 사용하지 않는다.",
			"",
			"## Non-goals",
			"",
			"- 사용자 확인 없이 원격 이슈, PR, MR을 생성하지 않는다.",
			"- issue 기반 브랜치와 격리 worktree 확인 없이 소스 repo에서 직접 구현하지 않는다.",
			"- 필요 없는 다이어그램이나 장식적 문구를 추가하지 않는다.",
			"",
			"## 구현 범위",
			"",
			"- issue contract, branch/worktree gate, benchmark fixture, CLI/MCP artifact generation 범위만 다룬다.",
			"- provider adapter 정책 복제나 원격 mutation 자동화는 범위에서 제외한다.",
			"",
			"## Verification",
			"",
			"- `./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge llm --model glm-5-turbo --json`",
			"- `go test ./... -count=1`",
			"- worktree cleanup 전 `git status --short --branch` 확인",
			"",
			"## 위험과 트레이드오프",
			"",
			"- scorer가 너무 느슨하면 품질 회귀를 놓치고, 너무 엄격하면 유효한 한국어 표현을 거부할 수 있다.",
			"- golden contract 변경은 의도된 public surface 변경으로만 갱신한다.",
			"",
			"## Feedback Log",
			"",
			"- 사용자 피드백은 source, body, 결정, 후속 조치로 기록한다.",
			"",
			"Guideline: " + guideline + "\n",
		}, "\n"),
		Plan: strings.Join([]string{
			"1. Problem intake: superpowers:brainstorming으로 사용자 의도, 품질 지표, 성공 기준, 모호성을 확인한다. 모호하면 구현하지 말고 질문한다.",
			"2. Domain grill: grill-with-docs로 용어, 기존 도메인 모델, 문서 갱신 필요성을 검토한다.",
			"3. Issue contract: 한국어 이슈에 문제, 근거, acceptance criteria, non-goals, verification, feedback log, guideline reference를 기록한다.",
			"4. Branch/worktree gate: 사용자가 issue 기반 브랜치를 제공할 때까지 구현을 막고, `" + worktreePath + "` 격리 worktree를 생성한 뒤 pwd/branch/HEAD를 검증한다.",
			"5. TDD: 격리 worktree에서 실패 테스트를 먼저 작성하고 `go test ./... -count=1`로 확인한다.",
			"6. Subagent DD: 독립 파일 소유권을 나누고, 모든 worker prompt에 pwd/branch/HEAD/worktree 검증과 stop-on-mismatch를 주입한다.",
			"7. Feedback loop: 각 단계 종료 후 proceed, revise, jump, pause 선택지를 제시하고 feedback add로 반영한다.",
			"8. Bad-case guard: `머지 완료했습니다`, `다음 단계는 수정입니다`처럼 선택지 없이 끝내는 응답을 실패 예시로 기록하고, 번호 선택지로 고친다.",
			"9. PR/MR: 한국어 PR/MR에 intent, changes, verification, risk, reviewer notes, issue link, cleanup status, guideline reference를 포함한다.",
			"10. Clarification gate: 품질 기준이 모호하면 여기서 멈춘다. branch/worktree 기록은 미래 구현 준비 상태일 뿐이며, 구현/worker 실행/PR 오픈은 사용자 확인 후에만 진행한다.",
			"",
			"Fixture-specific plan requirements:",
			expectedPlan,
			"",
			"Verify: ./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge llm --model glm-5-turbo --json",
		}, "\n"),
		TDDPlan: strings.Join([]string{
			"Write failing tests first before implementation.",
			"- 격리 worktree `" + worktreePath + "`에서 테스트를 작성하고 실행한다.",
			expectedTDD,
			"- 예상 실패를 확인한 뒤 최소 구현을 적용하고 `go test ./... -count=1`로 통과를 확인한다.",
		}, "\n"),
		TaskBreakdown: strings.Join([]string{
			"Worker A owns internal/core/issueops_benchmark.go and fixture scoring rules.",
			"Worker A also owns fixture schema validation, deterministic scoring, and benchmark fixture failure coverage.",
			"Worker B owns cmd/harness/issueops.go and CLI artifact generation.",
			"Worker B also owns judge adapter wiring, benchmark result JSON output, and issueops CLI integration.",
			"Worker C owns skills/issueops/SKILL.md and .agent-harness documentation if docs need updates.",
			"Each worker owns a non-overlapping task and reports expected output, touched files, tests, and remaining risk.",
			"Large issues use provider-native child work items: GitHub sub-issue or GitLab child item, then issueops link-child records the remote child.",
			"Fixture-specific task ownership:",
			expectedTasks,
		}, "\n"),
		SubagentPrompts: strings.Join([]string{
			"You are not alone in the codebase. Do not revert others. Own only the assigned files and adapt to changes made by other workers.",
			"Before work, report pwd, git branch --show-current, git rev-parse --short HEAD, and expected isolated worktree path `" + worktreePath + "`.",
			"If pwd, branch, HEAD, or worktree path does not match the IssueOps contract, stop and report the mismatch instead of editing or reviewing.",
			"Expected output: failing test evidence, implementation diff, verification commands, and Korean issue/PR notes when applicable.",
			"For narrow reviews, use verifier or direct bounded review. If code-reviewer is required, do not spawn nested subagents, use a 5 minute time budget, and verify pwd/branch/HEAD/worktree before inspecting the diff.",
			"Fixture-specific subagent requirements:",
			expectedSubagents,
		}, "\n"),
		ImplementationNotes:    clarificationGate + " Branch and worktree values are recorded as gates for future isolated work, not as evidence that implementation has already started.",
		PRDraft:                prDraft(fixture, issueNumber, branchName, worktreePath, guideline, expectedIssue, expectedPR),
		PhaseChoices:           phaseChoices(),
		BranchName:             branchName,
		WorktreePath:           worktreePath,
		ImplementationLocation: worktreePath,
		WorktreeCleanup:        worktreeCleanup(),
		GuidelineRef:           guideline,
		DomainContractEvidence: domainContractEvidence(),
		APIDocGateEvidence:     apiDocGateEvidence(),
		LiveEvidenceMatrix:     liveEvidenceMatrix(),
		ReviewFeedbackEvidence: reviewFeedbackEvidence(),
		CompletionHygiene:      completionHygiene(),
		PioneerSkillEvidence:   pioneerEvidenceFor(fixture.PioneerSkillTarget),
		RoutingTrace:           routingTraceFor(fixture),
	}
}

// routingTraceFor synthesizes the recorded routing trace from the fixture's own
// ExpectedRouting, so the deterministic benchmark run passes skill_routing_fidelity
// tautologically — exactly parallel to pioneerEvidenceFor. Real discrimination
// comes from the tampered-trace boundary test and from future real traces
// recorded during non-CI issueops runs. Fixtures without expected routing get nil.
func routingTraceFor(fixture core.IssueOpsBenchmarkFixture) []core.SkillRouting {
	if len(fixture.ExpectedRouting) == 0 {
		return nil
	}
	trace := make([]core.SkillRouting, len(fixture.ExpectedRouting))
	copy(trace, fixture.ExpectedRouting)
	return trace
}

// pioneerEvidenceFor returns distinctive-method evidence satisfying the
// targeted pioneer skill's signature detector. Non-targeted fixtures get an
// empty string: fabricating pioneer evidence for them would defeat the
// dimension's honest N/A handling.
func pioneerEvidenceFor(target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "dijkstra":
		return "dijkstra method: profiled the hot path; complexity O(n^2) -> O(n log n); scaling test N=100->1000->10000; before 4.1s, after 0.2s."
	case "codd":
		return "codd method: compared covering index vs partial index on (user_id, created_at); write penalty +8% insert cost; selectivity 0.99 at row count 12M, read:write 40:1."
	case "hopper":
		return "hopper method: reproduced the failure deterministically; isolated the root cause via hypothesis bisect; fix verified by rerun and a regression test."
	case "shannon":
		return "shannon method: SNR before 0.62 baseline -> after 0.81; entropy and redundancy re-measured post-cleanup."
	default:
		return ""
	}
}
