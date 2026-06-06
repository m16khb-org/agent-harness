package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/core"
)

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
