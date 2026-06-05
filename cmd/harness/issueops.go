package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core"
)

func runIssueOps(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		issueOpsUsage()
		return nil
	}
	switch args[0] {
	case "start":
		fs := flag.NewFlagSet("issueops start", flag.ContinueOnError)
		repo := fs.String("repo", "", "repository path")
		branch := fs.String("branch", "", "working branch")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: *repo, Branch: *branch})
		return printIssueOpsResult(record, *jsonOut, err)
	case "status":
		fs := flag.NewFlagSet("issueops status", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-issue":
		fs := flag.NewFlagSet("issueops link-issue", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		issueURL := fs.String("issue-url", "", "GitHub/GitLab issue URL")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), *id, *issueURL)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-plan":
		fs := flag.NewFlagSet("issueops link-plan", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		planPath := fs.String("plan-path", "", "issue-driven plan path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), *id, *planPath)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-worktree":
		fs := flag.NewFlagSet("issueops link-worktree", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		worktreePath := fs.String("worktree-path", "", "issue-driven worktree path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), *id, *worktreePath)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-child":
		fs := flag.NewFlagSet("issueops link-child", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		childURL := fs.String("child-url", "", "GitHub sub-issue or GitLab child item URL")
		title := fs.String("title", "", "optional child issue title")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.LinkIssueOpsChild(core.IssueOpsStateRoot(), *id, *childURL, *title)
		return printIssueOpsResult(record, *jsonOut, err)
	case "branch":
		return runIssueOpsBranch(args[1:])
	case "worktree":
		return runIssueOpsWorktree(args[1:])
	case "phase":
		fs := flag.NewFlagSet("issueops phase", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		to := fs.String("to", "", "target phase: problem, grill, plan, implement, ai-slop-clean, feedback, pr, done")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), *id, *to)
		return printIssueOpsResult(record, *jsonOut, err)
	case "feedback":
		return runIssueOpsFeedback(args[1:])
	case "cleanup":
		return runIssueOpsCleanup(args[1:])
	case "benchmark":
		return runIssueOpsBenchmark(args[1:])
	case "remote":
		return runIssueOpsRemote(args[1:])
	case "remote-score":
		return runIssueOpsRemote(append([]string{"score"}, args[1:]...))
	case "pr-readiness":
		fs := flag.NewFlagSet("issueops pr-readiness", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		strict := fs.Bool("strict", false, "verify git cleanliness, upstream sync, plan path, and linked worktree path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
		if err != nil {
			return err
		}
		readiness := core.IssueOpsPRReadiness(record)
		if *strict {
			readiness = core.IssueOpsStrictPRReadiness(record)
		}
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

func parseIssueOpsFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func issueOpsUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness issueops start --repo PATH [--branch NAME] [--json]
  agent-harness issueops status --id ID [--json]
  agent-harness issueops link-issue --id ID --issue-url URL [--json]
  agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--link-verified] [--json]
  agent-harness issueops link-plan --id ID --plan-path PATH [--json]
  agent-harness issueops link-worktree --id ID --worktree-path PATH [--json]
  agent-harness issueops worktree prepare-tools --id ID [--json]
  agent-harness issueops phase --id ID --to problem|grill|plan|implement|ai-slop-clean|feedback|pr|done [--json]
  agent-harness issueops feedback add --id ID --source TEXT --body TEXT [--classification TEXT] [--json]
  agent-harness issueops feedback mark-issue-updated --id ID [--json]
  agent-harness issueops pr-readiness --id ID [--strict] [--json]
  agent-harness issueops cleanup status --id ID [--merged] [--json]
  agent-harness issueops remote score --input PATH [--judge none|agy] [--json]
  agent-harness issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --label LABEL --assignee USER [--json]
`)
}

type issueOpsWorktreeToolPrepareResult struct {
	OK                   bool     `json:"ok"`
	ID                   string   `json:"id"`
	WorktreePath         string   `json:"worktree_path"`
	PackageManager       string   `json:"package_manager,omitempty"`
	DependenciesChecked  bool     `json:"dependencies_checked,omitempty"`
	DependenciesReady    bool     `json:"dependencies_ready,omitempty"`
	DependenciesAction   string   `json:"dependencies_action,omitempty"`
	CodeGraphProjectPath string   `json:"codegraph_project_path"`
	CodeGraphChecked     bool     `json:"codegraph_checked"`
	CodeGraphInitialized bool     `json:"codegraph_initialized,omitempty"`
	CodeGraphReady       bool     `json:"codegraph_ready"`
	Messages             []string `json:"messages,omitempty"`
}

func runIssueOpsWorktree(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops worktree prepare-tools --id ID [--json]")
		return nil
	}
	if args[0] != "prepare-tools" {
		return fmt.Errorf("unknown issueops worktree subcommand")
	}
	fs := flag.NewFlagSet("issueops worktree prepare-tools", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
	if err != nil {
		return err
	}
	result, err := prepareIssueOpsWorktreeTools(record)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("worktree: %s\n", result.WorktreePath)
	if result.DependenciesChecked {
		fmt.Printf("package_manager: %s\n", result.PackageManager)
		fmt.Printf("dependencies_ready: %v\n", result.DependenciesReady)
		if result.DependenciesAction != "" {
			fmt.Printf("dependencies_action: %s\n", result.DependenciesAction)
		}
	}
	fmt.Printf("codegraph_project_path: %s\n", result.CodeGraphProjectPath)
	fmt.Printf("codegraph_ready: %v\n", result.CodeGraphReady)
	for _, message := range result.Messages {
		fmt.Printf("- %s\n", message)
	}
	return nil
}

func prepareIssueOpsWorktreeTools(record core.IssueOpsRecord) (issueOpsWorktreeToolPrepareResult, error) {
	worktree := strings.TrimSpace(record.WorktreePath)
	result := issueOpsWorktreeToolPrepareResult{
		OK:                   true,
		ID:                   record.ID,
		WorktreePath:         worktree,
		CodeGraphProjectPath: worktree,
	}
	if worktree == "" {
		result.OK = false
		return result, fmt.Errorf("worktree_path is required")
	}
	if info, err := os.Stat(worktree); err != nil || !info.IsDir() {
		result.OK = false
		return result, fmt.Errorf("worktree_path does not exist or is not a directory: %s", worktree)
	}
	if err := prepareIssueOpsWorktreeDependencies(worktree, &result); err != nil {
		result.OK = false
		return result, err
	}
	result.CodeGraphChecked = true
	if err := exec.Command("codegraph", "status", worktree).Run(); err == nil {
		result.CodeGraphReady = true
		result.Messages = append(result.Messages, "CodeGraph index already ready for IssueOps worktree")
	} else if out, err := exec.Command("codegraph", "init", "-i", worktree).CombinedOutput(); err != nil {
		result.OK = false
		return result, fmt.Errorf("initialize CodeGraph for IssueOps worktree: %w: %s", err, strings.TrimSpace(string(out)))
	} else {
		result.CodeGraphInitialized = true
		result.CodeGraphReady = true
		result.Messages = append(result.Messages, "initialized CodeGraph index for IssueOps worktree")
	}
	return result, nil
}

func prepareIssueOpsWorktreeDependencies(worktree string, result *issueOpsWorktreeToolPrepareResult) error {
	manager := issueOpsWorktreePackageManager(worktree)
	if manager == "" {
		return nil
	}
	result.PackageManager = manager
	result.DependenciesChecked = true
	if info, err := os.Stat(filepath.Join(worktree, "node_modules")); err == nil && info.IsDir() {
		result.DependenciesReady = true
		result.DependenciesAction = "already_present"
		result.Messages = append(result.Messages, "node_modules already present in IssueOps worktree")
		return nil
	}
	switch manager {
	case "pnpm":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "pnpm", "install", "--frozen-lockfile", "--prefer-offline")
		cmd.Dir = worktree
		out, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("install pnpm dependencies for IssueOps worktree timed out")
		}
		if err != nil {
			return fmt.Errorf("install pnpm dependencies for IssueOps worktree: %w: %s", err, strings.TrimSpace(string(out)))
		}
		result.DependenciesReady = true
		result.DependenciesAction = "pnpm_install"
		result.Messages = append(result.Messages, "installed pnpm dependencies for IssueOps worktree")
		return nil
	default:
		result.DependenciesAction = "manual"
		result.Messages = append(result.Messages, "detected "+manager+" dependencies; install them in the IssueOps worktree before running tests")
		return nil
	}
}

func issueOpsWorktreePackageManager(worktree string) string {
	if _, err := os.Stat(filepath.Join(worktree, "package.json")); err != nil {
		return ""
	}
	switch {
	case fileExists(filepath.Join(worktree, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(worktree, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(worktree, "package-lock.json")):
		return "npm"
	default:
		return ""
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func runIssueOpsBranch(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--link-verified] [--json]")
		return nil
	}
	if args[0] != "prepare" {
		return fmt.Errorf("unknown issueops branch subcommand")
	}
	fs := flag.NewFlagSet("issueops branch prepare", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	provider := fs.String("provider", "", "remote provider: github or gitlab")
	issueURL := fs.String("issue-url", "", "GitHub/GitLab issue URL")
	branch := fs.String("branch", "", "provider-linked issue-number branch name")
	baseBranch := fs.String("base-branch", "", "remote base branch or ref")
	baseSHA := fs.String("base-sha", "", "optional resolved base commit SHA")
	remoteBranchURL := fs.String("remote-branch-url", "", "optional provider branch URL after creation")
	linkVerified := fs.Bool("link-verified", false, "record that the provider issue shows the branch link")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), *id, core.IssueOpsBranchPrepareRequest{
		Provider:        *provider,
		IssueURL:        *issueURL,
		Branch:          *branch,
		BaseBranch:      *baseBranch,
		BaseSHA:         *baseSHA,
		RemoteBranchURL: *remoteBranchURL,
		LinkVerified:    *linkVerified,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsRemote(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops remote score --input PATH [--judge none|agy] [--json]\n       agent-harness issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --label LABEL --assignee USER [--json]")
		return nil
	}
	if args[0] == "remote-score" {
		args[0] = "score"
	}
	if len(args) == 0 {
		return fmt.Errorf("unknown issueops remote subcommand")
	}
	switch args[0] {
	case "score":
		fs := flag.NewFlagSet("issueops remote score", flag.ContinueOnError)
		input := fs.String("input", "", "IssueOps remote scoring request JSON file")
		judge := fs.String("judge", "agy", "judge backend: agy or none")
		agyCommand := fs.String("agy-command", "agy", "agy command path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		req, err := readIssueOpsRemoteScoringRequestFile(*input)
		if err != nil {
			return err
		}
		var result core.IssueOpsRemoteScoringResult
		switch *judge {
		case "agy":
			result, err = core.RunIssueOpsRemoteAgyJudge(core.IssueOpsRemoteAgyJudgeRequest{
				RepoRoot:   ".",
				AgyCommand: *agyCommand,
				Request:    req,
			})
		case "none":
			result, err = core.ScoreIssueOpsRemoteCandidates(req)
		default:
			err = fmt.Errorf("unsupported issueops remote score judge %q", *judge)
		}
		if err != nil {
			return err
		}
		if *jsonOut {
			return printJSON(result)
		}
		fmt.Printf("provider=%s threshold=%.2f related_issues=%d labels=%d\n", result.Provider, result.Threshold, len(result.SelectedRelatedIssues), len(result.SelectedLabels))
		for _, issue := range result.SelectedRelatedIssues {
			fmt.Printf("- related issue: %s score=%.2f\n", formatIssueOpsRemoteIssueRef(issue), issue.Score)
		}
		for _, label := range result.SelectedLabels {
			fmt.Printf("- label: %s score=%.2f\n", label.Name, label.Score)
		}
		return nil
	case "verify-artifact":
		fs := flag.NewFlagSet("issueops remote verify-artifact", flag.ContinueOnError)
		id := fs.String("id", "", "IssueOps id")
		provider := fs.String("provider", "", "remote provider: github or gitlab")
		kind := fs.String("kind", "", "remote artifact kind: pr or mr")
		url := fs.String("url", "", "remote PR/MR URL")
		var labels repeatedFlag
		var assignees repeatedFlag
		fs.Var(&labels, "label", "verified remote label; may be repeated")
		fs.Var(&labels, "labels", "verified remote label; may be repeated")
		fs.Var(&assignees, "assignee", "verified remote assignee; may be repeated")
		fs.Var(&assignees, "assignees", "verified remote assignee; may be repeated")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.VerifyIssueOpsRemoteArtifact(core.IssueOpsStateRoot(), *id, core.IssueOpsRemoteArtifactVerificationRequest{
			Provider:  *provider,
			Kind:      *kind,
			URL:       *url,
			Labels:    labels,
			Assignees: assignees,
		})
		return printIssueOpsResult(record, *jsonOut, err)
	default:
		return fmt.Errorf("unknown issueops remote subcommand %q", args[0])
	}
}

func formatIssueOpsRemoteIssueRef(issue core.IssueOpsRemoteScoredItem) string {
	ref := firstNonEmptyMain(issue.ID, issue.URL)
	title := strings.TrimSpace(issue.Title)
	if title == "" {
		return firstNonEmptyMain(ref, issue.Title)
	}
	if ref == "" {
		return title
	}
	return fmt.Sprintf("%s (%s)", ref, title)
}

func runIssueOpsBenchmark(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops benchmark run|compare|gate [--json]")
		return nil
	}
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
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
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
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
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
	case "gate":
		fs := flag.NewFlagSet("issueops benchmark gate", flag.ContinueOnError)
		baselineID := fs.String("baseline", "", "baseline benchmark id")
		candidateID := fs.String("candidate", "", "candidate benchmark id")
		candidateFile := fs.String("candidate-file", "", "IssueOps autoresearch candidate JSON file")
		var changedPaths repeatedFlag
		fs.Var(&changedPaths, "changed-path", "changed path to check against the candidate edit surface; repeatable")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		candidate, err := readIssueOpsAutoresearchCandidateFile(*candidateFile)
		if err != nil {
			return err
		}
		baseline, err := core.ReadIssueOpsBenchmarkRun(core.StateDir(), *baselineID)
		if err != nil {
			return err
		}
		candidateRun, err := core.ReadIssueOpsBenchmarkRun(core.StateDir(), *candidateID)
		if err != nil {
			return err
		}
		result := core.EvaluateIssueOpsAutoresearchGate(core.IssueOpsAutoresearchGateRequest{
			Candidate:    candidate,
			BaselineRun:  baseline,
			CandidateRun: candidateRun,
			ChangedPaths: changedPaths,
		})
		if *jsonOut {
			return printJSON(result)
		}
		fmt.Printf("keep_candidate=%v ok=%v discard_reasons=%d\n", result.KeepCandidate, result.OK, len(result.DiscardReasons))
		for _, reason := range result.DiscardReasons {
			fmt.Printf("- discard: %s\n", reason)
		}
		return nil
	default:
		return fmt.Errorf("unknown issueops benchmark subcommand %q", args[0])
	}
}

func readIssueOpsRemoteScoringRequestFile(path string) (core.IssueOpsRemoteScoringRequest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return core.IssueOpsRemoteScoringRequest{}, fmt.Errorf("input is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return core.IssueOpsRemoteScoringRequest{}, err
	}
	var req core.IssueOpsRemoteScoringRequest
	req, err = core.DecodeIssueOpsRemoteScoringRequest(b)
	if err != nil {
		return core.IssueOpsRemoteScoringRequest{}, fmt.Errorf("parse input file %s: %w", path, err)
	}
	return req, nil
}

func firstNonEmptyMain(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

type repeatedFlag []string

func (f *repeatedFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func readIssueOpsAutoresearchCandidateFile(path string) (core.IssueOpsAutoresearchCandidate, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return core.IssueOpsAutoresearchCandidate{}, fmt.Errorf("candidate-file is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return core.IssueOpsAutoresearchCandidate{}, err
	}
	var candidate core.IssueOpsAutoresearchCandidate
	if err := json.Unmarshal(b, &candidate); err != nil {
		return core.IssueOpsAutoresearchCandidate{}, fmt.Errorf("parse candidate file %s: %w", path, err)
	}
	return candidate, nil
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
			"- 관련 이슈 링크: https://example.com/acme/agent-harness/issues/" + issueNumber,
			"- 브랜치 요구사항: issue #" + issueNumber + " 기반 `" + branchName + "`",
			"- 격리 worktree: `" + worktreePath + "`",
			"- 가이드라인: `" + guideline + "`",
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
			"Classification: valid_review, stale_review, contract_change, defect, question, noise, rollout_evidence_missing, or environment_debt.",
			"Verification: file/line, command output, diff evidence, or live evidence decides validity.",
			"Thread reply: original review thread gets a verdict and evidence.",
			"Resolution: unresolved, fixed, resolved, obsolete, or split to follow-up is re-checked.",
		}, "\n"),
		CompletionHygiene: strings.Join([]string{
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

func runIssueOpsFeedback(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops feedback add --id ID --source TEXT --body TEXT [--classification TEXT] [--json]\n       agent-harness issueops feedback mark-issue-updated --id ID [--json]")
		return nil
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("issueops feedback add", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		source := fs.String("source", "", "feedback source")
		body := fs.String("body", "", "feedback body")
		classification := fs.String("classification", "", "optional feedback classification, such as contract_change, defect, question, or noise")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.AddIssueOpsFeedback(core.IssueOpsStateRoot(), *id, *source, *body, *classification)
		return printIssueOpsResult(record, *jsonOut, err)
	case "mark-issue-updated":
		fs := flag.NewFlagSet("issueops feedback mark-issue-updated", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.MarkIssueOpsContractFeedbackIssueUpdated(core.IssueOpsStateRoot(), *id)
		return printIssueOpsResult(record, *jsonOut, err)
	default:
		return fmt.Errorf("unknown issueops feedback subcommand")
	}
}

func runIssueOpsCleanup(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops cleanup status --id ID [--merged] [--json]")
		return nil
	}
	switch args[0] {
	case "status":
		fs := flag.NewFlagSet("issueops cleanup status", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		merged := fs.Bool("merged", false, "confirm the remote PR/MR was verified merged before cleanup")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		status, err := core.IssueOpsCleanupStatusByID(core.IssueOpsStateRoot(), *id, core.IssueOpsCleanupStatusRequest{Merged: *merged})
		if err != nil {
			return err
		}
		if *jsonOut {
			return printJSON(status)
		}
		fmt.Printf("ready: %v\n", status.Ready)
		for _, missing := range status.Missing {
			fmt.Printf("- missing: %s\n", missing)
		}
		if len(status.Choices) > 0 {
			fmt.Println("선택지:")
			for _, choice := range status.Choices {
				fmt.Println(choice)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown issueops cleanup subcommand")
	}
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
