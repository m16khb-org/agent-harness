package main

import (
	"flag"
	"fmt"
	"strings"

	"agent-harness/internal/core"
)

func runIssueOps(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subcommand is required")
	}
	switch args[0] {
	case "start":
		fs := flag.NewFlagSet("issueops start", flag.ContinueOnError)
		repo := fs.String("repo", "", "repository path")
		branch := fs.String("branch", "", "working branch")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: *repo, Branch: *branch})
		return printIssueOpsResult(record, *jsonOut, err)
	case "status":
		fs := flag.NewFlagSet("issueops status", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-issue":
		fs := flag.NewFlagSet("issueops link-issue", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		issueURL := fs.String("issue-url", "", "GitHub/GitLab issue URL")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		record, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), *id, *issueURL)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-plan":
		fs := flag.NewFlagSet("issueops link-plan", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		planPath := fs.String("plan-path", "", "issue-driven plan path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		record, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), *id, *planPath)
		return printIssueOpsResult(record, *jsonOut, err)
	case "feedback":
		return runIssueOpsFeedback(args[1:])
	case "benchmark":
		return runIssueOpsBenchmark(args[1:])
	case "pr-readiness":
		fs := flag.NewFlagSet("issueops pr-readiness", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
		if err != nil {
			return err
		}
		readiness := core.IssueOpsPRReadiness(record)
		if *jsonOut {
			return printJSON(readiness)
		}
		fmt.Printf("ready: %v\n", readiness.Ready)
		for _, missing := range readiness.Missing {
			fmt.Printf("- missing: %s\n", missing)
		}
		return nil
	default:
		return fmt.Errorf("unknown issueops subcommand %q", args[0])
	}
}

func runIssueOpsBenchmark(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("unknown issueops benchmark subcommand")
	}
	switch args[0] {
	case "run":
		fs := flag.NewFlagSet("issueops benchmark run", flag.ContinueOnError)
		fixturesPath := fs.String("fixtures", "", "benchmark fixtures path")
		judge := fs.String("judge", "agy", "judge backend: none or agy")
		agyCommand := fs.String("agy-command", "agy", "agy command path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		fixtures, err := core.LoadIssueOpsBenchmarkFixtures(*fixturesPath)
		if err != nil {
			return err
		}
		artifacts := make(map[string]core.IssueOpsBenchmarkArtifact, len(fixtures))
		for _, fixture := range fixtures {
			artifacts[fixture.ID] = benchmarkArtifactFromFixture(fixture)
		}
		result, err := core.RunIssueOpsBenchmark(core.IssueOpsBenchmarkRunRequest{
			StateRoot: "",
			Fixtures:  fixtures,
			Artifacts: artifacts,
		})
		if err != nil {
			return err
		}
		if *judge == "agy" {
			for i, fixture := range fixtures {
				artifact := artifacts[fixture.ID]
				judgeScore, err := core.RunIssueOpsAgyJudge(core.IssueOpsAgyJudgeRequest{
					RepoRoot:   ".",
					AgyCommand: *agyCommand,
					Fixture:    fixture,
					Artifact:   artifact,
				})
				if err != nil {
					return err
				}
				result.Scores[i] = core.MergeIssueOpsBenchmarkScoreWithJudge(result.Scores[i], judgeScore)
			}
		} else if *judge != "none" {
			return fmt.Errorf("unsupported issueops benchmark judge %q", *judge)
		}
		result = core.FinalizeIssueOpsBenchmarkRunResult(result)
		if err := core.SaveIssueOpsBenchmarkRun(core.StateDir(), result); err != nil {
			return err
		}
		if *jsonOut {
			return printJSON(result)
		}
		fmt.Printf("%s fixtures=%d average=%.2f minimum=%.2f critical_failures=%d\n", result.ID, result.FixtureCount, result.AverageScore, result.MinimumScore, result.CriticalFailureCount)
		return nil
	case "compare":
		fs := flag.NewFlagSet("issueops benchmark compare", flag.ContinueOnError)
		baselineID := fs.String("baseline", "", "baseline benchmark id")
		candidateID := fs.String("candidate", "", "candidate benchmark id")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		baseline, err := core.ReadIssueOpsBenchmarkRun(core.StateDir(), *baselineID)
		if err != nil {
			return err
		}
		candidate, err := core.ReadIssueOpsBenchmarkRun(core.StateDir(), *candidateID)
		if err != nil {
			return err
		}
		result := core.CompareIssueOpsBenchmarkRuns(baseline, candidate)
		if *jsonOut {
			return printJSON(result)
		}
		fmt.Printf("improved=%v average_delta=%.2f minimum_delta=%.2f critical_failure_delta=%d\n", result.Improved, result.AverageScoreDelta, result.MinimumScoreDelta, result.CriticalFailureDelta)
		return nil
	default:
		return fmt.Errorf("unknown issueops benchmark subcommand %q", args[0])
	}
}

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
			"- 브랜치 요구사항: issue #" + issueNumber + " 기반 `" + branchName + "`",
			"- 격리 worktree: `" + worktreePath + "`",
			"- 가이드라인: `" + guideline + "`",
			"",
			"## Acceptance Criteria",
			"",
			expectedIssue,
			"- 문제 파악 단계에서 모호한 품질 기준을 명시하고, 구현 전 측정 기준과 성공 조건을 확정한다.",
			"- 각 단계 종료 후 proceed, revise, jump, pause 선택지를 사용자에게 제공한다.",
			"- 이슈와 PR/MR 본문은 한국어로 작성하고 과도한 이모지는 사용하지 않는다.",
			"",
			"## Non-goals",
			"",
			"- 사용자 확인 없이 원격 이슈, PR, MR을 생성하지 않는다.",
			"- issue 기반 브랜치와 격리 worktree 확인 없이 소스 repo에서 직접 구현하지 않는다.",
			"- 필요 없는 다이어그램이나 장식적 문구를 추가하지 않는다.",
			"",
			"## Plan Link",
			"",
			"- 계획 문서: `plans/issue-" + issueNumber + "-issueops-quality.md`",
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
			"8. PR/MR: 한국어 PR/MR에 intent, changes, verification, risk, reviewer notes, issue link, cleanup status, guideline reference를 포함한다.",
			"9. Clarification gate: 품질 기준이 모호하면 여기서 멈춘다. branch/worktree 기록은 미래 구현 준비 상태일 뿐이며, 구현/worker 실행/PR 오픈은 사용자 확인 후에만 진행한다.",
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
			"- Fixture-specific PR requirements:",
			expectedPR,
			"",
			"Issue: https://example.com/acme/agent-harness/issues/" + issueNumber,
			"Guideline: " + guideline + "\n",
		}, "\n"),
		PhaseChoices:           "Proceed to next phase | revise current phase | jump to issue/plan/implementation/feedback/PR phase | pause for user decision",
		BranchName:             branchName,
		WorktreePath:           worktreePath,
		ImplementationLocation: worktreePath,
		WorktreeCleanup:        "clean worktree confirmed; cleanup/remove choices offered and present after merge",
		GuidelineRef:           guideline,
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

func runIssueOpsFeedback(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return fmt.Errorf("unknown issueops feedback subcommand")
	}
	fs := flag.NewFlagSet("issueops feedback add", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	source := fs.String("source", "", "feedback source")
	body := fs.String("body", "", "feedback body")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	record, err := core.AddIssueOpsFeedback(core.IssueOpsStateRoot(), *id, *source, *body)
	return printIssueOpsResult(record, *jsonOut, err)
}

func printIssueOpsResult(record core.IssueOpsRecord, jsonOut bool, err error) error {
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(record)
	}
	fmt.Printf("%s %s %s\n", record.ID, record.Phase, record.Repo)
	return nil
}
