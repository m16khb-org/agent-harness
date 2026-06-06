package main

import (
	"fmt"
	"strings"

	"agent-harness/internal/core"
)

func benchmarkArtifactFromFixture(fixture core.IssueOpsBenchmarkFixture) core.IssueOpsBenchmarkArtifact {
	const guideline = "docs/superpowers/specs/issueops-issue-pr-guidelines.md"
	issueNumber := "1"
	branchName := "feature/1-issueops-quality-benchmark"
	worktreePath := "/repo.worktrees/feature-1-issueops-quality-benchmark"
	problem := strings.TrimSpace(fixture.UserPrompt)
	if problem == "" {
		problem = fixture.Title
	}
	expectedIssue := issueOpsBenchmarkBullets(fixture.ExpectedIssue)
	expectedPlan := issueOpsBenchmarkBullets(fixture.ExpectedPlan)
	expectedTasks := issueOpsBenchmarkOwnedTasks(fixture.ExpectedTasks)
	expectedTDD := issueOpsBenchmarkBullets(fixture.ExpectedTDD)
	expectedSubagents := issueOpsBenchmarkBullets(fixture.ExpectedSubagents)
	expectedPR := issueOpsBenchmarkBullets(fixture.ExpectedPR)
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
			"## Verification",
			"",
			"- `./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge agy --json`",
			"- `go test ./... -count=1`",
			"- worktree cleanup 전 `git status --short --branch` 확인",
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
			"Verify: ./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge agy --json",
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
		ImplementationNotes: clarificationGate + " Branch and worktree values are recorded as gates for future isolated work, not as evidence that implementation has already started.",
		PRDraft: strings.Join([]string{
			"## Intent",
			"",
			"이 PR/MR 초안은 issue #" + issueNumber + "의 IssueOps 품질 요구사항을 한국어 이슈/계획/TDD/subagent/worktree/cleanup 루프로 충족하기 위한 것이다. 품질 기준이 모호하면 실제 PR/MR 오픈과 구현은 사용자 clarification 전까지 차단된다.",
			"",
			"## Changes",
			"",
			"- 예정 변경: 문제 파악과 domain grill 이후 issue contract를 확정한다.",
			"- 예정 변경: issue 기반 브랜치 `" + branchName + "`와 격리 worktree `" + worktreePath + "`에서만 구현한다.",
			"- 예정 변경: worker prompt에 pwd, branch, HEAD, worktree 검증과 bounded review 제약을 넣는다.",
			"- 예정 변경: 작업 완료 후 clean 상태와 worktree cleanup/remove 선택지를 기록한다.",
			"- 현재 상태: 아직 구현하지 않았고, clarification과 branch/worktree gate가 통과된 뒤에만 TDD/구현/PR 오픈으로 진행한다.",
			"",
			"## Verification",
			"",
			"- `./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge agy --json`",
			"- `go test ./... -count=1`",
			"- `git status --short --branch`",
			"",
			"## Benchmark Evidence",
			"",
			"- Fixture: `" + fixture.ID + "` - " + strings.TrimSpace(fixture.Title),
			"- Target result: average_score 100, minimum_score 100, critical_failure_count 0.",
			"- Evidence summary: 문제 파악은 모호성을 먼저 확인하고, 계획은 측정 기준과 TDD를 먼저 세우며, worker task는 fixture schema, deterministic scoring, judge adapter, CLI wiring을 각각 소유한다.",
			"- Bad-case evidence: 결론만 보고하고 번호 선택지를 주지 않는 응답은 실패 예시로 기록되어야 한다.",
			"- Expected issue evidence:",
			expectedIssue,
			"- Expected PR/MR evidence:",
			expectedPR,
			"",
			"## Risk",
			"",
			"- LLM judge 점수는 변동될 수 있어 deterministic scorer와 함께 확인한다.",
			"- 모호한 요청은 구현보다 clarification을 우선한다.",
			"",
			"## Reviewer Notes",
			"",
			"- 이슈/PR/MR 본문은 한국어 기준이며 과도한 이모지는 없다.",
			"- Cleanup status: worktree is clean; cleanup/remove choice is offered after merge.",
			"- Bad case: `PR/MR 머지 완료했습니다`처럼 cleanup 선택지 없이 끝나는 보고는 불완전하다.",
			"- Fixture-specific PR requirements:",
			expectedPR,
			"",
			"Issue: https://example.com/acme/agent-harness/issues/" + issueNumber,
			"Guideline: " + guideline + "\n",
		}, "\n"),
		PhaseChoices: strings.Join([]string{
			"선택지:",
			"1. Proceed: 다음 IssueOps phase로 진행한다. (추천)",
			"2. Revise: 현재 phase의 issue/plan/task contract를 수정한다.",
			"3. Jump: issue, plan, implementation, ai-slop-clean, feedback, PR phase 중 필요한 단계로 이동한다.",
			"4. Pause: 사용자 결정 전까지 진행을 멈춘다.",
		}, "\n"),
		BranchName:             branchName,
		WorktreePath:           worktreePath,
		ImplementationLocation: worktreePath,
		WorktreeCleanup: strings.Join([]string{
			"clean worktree confirmed after merge; cleanup/remove choices are offered and present.",
			"선택지:",
			"1. Cleanup: merged worktree and local branch를 삭제한다. (추천)",
			"2. Keep: worktree를 보존하고 나중에 확인한다.",
			"3. Inspect: stale IssueOps worktree 전체를 점검한 뒤 삭제 후보를 제시한다.",
		}, "\n"),
		GuidelineRef: guideline,
		DomainContractEvidence: strings.Join([]string{
			"Invariant: preserve the user-visible behavior described by the issue.",
			"Exact mechanism: compare the documented mechanism with source file:line evidence before implementation.",
			"Equivalent behavior: if the exact mechanism is absent, record whether another verified path enforces the same invariant.",
			"Source: current files, docs, logs, or command output must be cited before claiming completion.",
		}, "\n"),
		APIDocGateEvidence: strings.Join([]string{
			"Changed endpoint list is recorded, or the plan states that no endpoint contract changed.",
			"Public error responses are checked against service/usecase/error-mapping behavior.",
			"Static check: api_doc_static_check or the target repo's equivalent API doc command.",
			"Review: api_doc_review for OpenAPI/Swagger/API doc parity when endpoint contracts change.",
		}, "\n"),
		LiveEvidenceMatrix: strings.Join([]string{
			"Environment matrix covers dev, stg, prod, or the target repo's equivalent runtime surfaces.",
			"Repo config evidence is compared with runtime evidence before assigning root cause.",
			"Runtime evidence records live config, logs, deployed code, or external service probes where available.",
			"Remediation order is recorded before edits when multiple fixes are needed.",
		}, "\n"),
		ReviewFeedbackEvidence: strings.Join([]string{
			"Classification: valid_review, stale_review, contract_change, defect, question, noise, rollout_evidence_missing, or environment_debt from Kodus, Gemini Code Assist, review-agent, human review, QA, or CI.",
			"Verification: file/line, command output, diff evidence, or live evidence decides validity.",
			"Thread reply: original review thread gets a verdict and evidence.",
			"Resolution: unresolved, fixed, resolveReviewThread/resolved=true, obsolete, or split to follow-up is re-checked.",
		}, "\n"),
		CompletionHygiene: strings.Join([]string{
			"Draft issue completion record includes final diff, evidence, labels, children, PR URL, and unresolved follow-ups.",
			"Final diff is reviewed from the actual worktree.",
			"Target branch and source branch are verified before remote artifact updates.",
			"Remote artifact issue/PR/MR body is refreshed against the final implementation.",
			"Single commit policy is checked, or multiple commits are explicitly justified.",
			"Cleanup status is recorded with worktree and branch checks.",
		}, "\n"),
	}
}

func issueOpsBenchmarkBullets(items []string) string {
	if len(items) == 0 {
		return "- 해당 fixture의 추가 요구사항 없음"
	}
	var out []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, "- "+item)
	}
	if len(out) == 0 {
		return "- 해당 fixture의 추가 요구사항 없음"
	}
	return strings.Join(out, "\n")
}

func issueOpsBenchmarkOwnedTasks(items []string) string {
	if len(items) == 0 {
		return "- Worker Fixture owns verification that this fixture has no additional task requirements."
	}
	var out []string
	for i, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, fmt.Sprintf("- Worker Fixture-%d owns %s and reports test evidence for that task.", i+1, item))
	}
	if len(out) == 0 {
		return "- Worker Fixture owns verification that this fixture has no additional task requirements."
	}
	return strings.Join(out, "\n")
}
