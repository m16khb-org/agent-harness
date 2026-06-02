package main

import (
	"flag"
	"fmt"

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
	return core.IssueOpsBenchmarkArtifact{
		ProblemSummary:         fixture.UserPrompt,
		IssueDraft:             "## Problem\n\n문제 요약\n\n## Current Evidence\n\n현재 근거\n\n## Acceptance Criteria\n\n완료 기준\n\n## Non-goals\n\n비목표\n\n## Verification\n\n검증\n\n## Feedback Log\n\n피드백 기록\n\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
		Plan:                   "Run: go test ./... -count=1\n",
		TDDPlan:                "Write failing test before implementation.\n",
		TaskBreakdown:          "Worker A owns internal/core/issueops_benchmark.go. Worker B owns cmd/harness/issueops.go.",
		SubagentPrompts:        "You are not alone in the codebase. Do not revert others. Own internal/core only. Expected output: tests and implementation. Before work, report pwd, branch, HEAD, and worktree path; stop on mismatch. For narrow review, use verifier or direct bounded review. If code-reviewer is required, do not spawn subagents and use a 5 minute time budget.",
		PRDraft:                "Intent\n의도\nChanges\n변경사항\nVerification\n검증\nRisk\n위험\nReviewer Notes\n리뷰어 참고\nIssue: https://github.com/m16khb/agent-harness/issues/1\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
		PhaseChoices:           "Proceed to plan | revise current phase | jump to issue | pause",
		BranchName:             "feature/1-issueops-quality-benchmark",
		WorktreePath:           "/repo.worktrees/feature-1-issueops-quality-benchmark",
		ImplementationLocation: "/repo.worktrees/feature-1-issueops-quality-benchmark",
		WorktreeCleanup:        "clean worktree; cleanup choices offered after merge",
		GuidelineRef:           "docs/superpowers/specs/issueops-issue-pr-guidelines.md",
	}
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
