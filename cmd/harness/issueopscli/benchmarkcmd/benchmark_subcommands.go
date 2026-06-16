package benchmarkcmd

import (
	"flag"
	"fmt"

	"agent-harness/cmd/harness/issueopscli/benchmarkartifact"
	"agent-harness/internal/core"
)

// One handler per `issueops benchmark <subcommand>`. Run (benchmark.go) routes
// through the benchmarkSubcommands registry, keeping the router low-branch and
// each subcommand independently testable.

func runBenchmarkRun(args []string) error {
	fs := flag.NewFlagSet("issueops benchmark run", flag.ContinueOnError)
	fixturesPath := fs.String("fixtures", "", "benchmark fixtures path")
	judge := fs.String("judge", "agy", "judge backend: none, file, or agy (legacy: external agy -p; prefer file)")
	judgeFile := fs.String("judge-file", "", "provenanced judge map JSON path for --judge file ({\"source_run_id\":..,\"provenance\":..,\"scores\":{\"<fixtureID>\":<score>}}); reads stdin when empty")
	agyCommand := fs.String("agy-command", "agy", "agy command path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	fixtures, err := core.LoadIssueOpsBenchmarkFixtures(*fixturesPath)
	if err != nil {
		return err
	}
	artifacts := make(map[string]core.IssueOpsBenchmarkArtifact, len(fixtures))
	for _, fixture := range fixtures {
		artifacts[fixture.ID] = benchmarkartifact.FromFixture(fixture)
	}
	result, err := core.RunIssueOpsBenchmark(core.IssueOpsBenchmarkRunRequest{
		StateRoot: "",
		Fixtures:  fixtures,
		Artifacts: artifacts,
	})
	if err != nil {
		return err
	}
	if err := applyBenchmarkJudge(*judge, *judgeFile, *agyCommand, result, fixtures, artifacts); err != nil {
		return err
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
}

// applyBenchmarkJudge augments the deterministic run result with judge scores
// from the selected backend (agy external command, provenance-checked file, or
// none). Scores merge in place through the shared result.Scores backing array.
func applyBenchmarkJudge(judge, judgeFile, agyCommand string, result core.IssueOpsBenchmarkRunResult, fixtures []core.IssueOpsBenchmarkFixture, artifacts map[string]core.IssueOpsBenchmarkArtifact) error {
	switch judge {
	case "agy":
		for i, fixture := range fixtures {
			artifact := artifacts[fixture.ID]
			judgeScore, err := core.RunIssueOpsAgyJudge(core.IssueOpsAgyJudgeRequest{
				RepoRoot:   ".",
				AgyCommand: agyCommand,
				Fixture:    fixture,
				Artifact:   artifact,
			})
			if err != nil {
				return err
			}
			result.Scores[i] = core.MergeIssueOpsBenchmarkScoreWithJudge(result.Scores[i], judgeScore)
		}
	case "file":
		judgeMap, judgeScores, err := readIssueOpsJudgeMap(judgeFile, fixtures)
		if err != nil {
			return err
		}
		// Provenance is validated BEFORE merge and fails closed: a judge map
		// self-attributed to this run (or naming a non-existent source run) is
		// rejected. This is a self-reference guard, not a proof of judge
		// independence.
		if err := core.ValidateJudgeProvenance(judgeMap, result.ID, core.StateDir()); err != nil {
			return err
		}
		for i, fixture := range fixtures {
			result.Scores[i] = core.MergeIssueOpsBenchmarkScoreWithJudge(result.Scores[i], judgeScores[fixture.ID])
		}
	case "none":
	default:
		return fmt.Errorf("unsupported issueops benchmark judge %q", judge)
	}
	return nil
}

func runBenchmarkCompare(args []string) error {
	fs := flag.NewFlagSet("issueops benchmark compare", flag.ContinueOnError)
	baselineID := fs.String("baseline", "", "baseline benchmark id")
	candidateID := fs.String("candidate", "", "candidate benchmark id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
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
}

func runBenchmarkGate(args []string) error {
	fs := flag.NewFlagSet("issueops benchmark gate", flag.ContinueOnError)
	baselineID := fs.String("baseline", "", "baseline benchmark id")
	candidateID := fs.String("candidate", "", "candidate benchmark id")
	candidateFile := fs.String("candidate-file", "", "IssueOps autoresearch candidate JSON file")
	var changedPaths repeatedFlag
	fs.Var(&changedPaths, "changed-path", "changed path to check against the candidate edit surface; repeatable")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
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
}

func runBenchmarkReliability(args []string) error {
	fs := flag.NewFlagSet("issueops benchmark reliability", flag.ContinueOnError)
	outcomesPath := fs.String("outcomes", "", "recorded offline outcomes JSON path ({\"runs\":[{\"run_id\":..,\"provenance\":..,\"outcomes\":{\"<fixtureID>\":<bool>}}]}); reads stdin when empty")
	alpha := fs.Float64("alpha", 0.05, "confidence level alpha for Clopper-Pearson intervals (0<alpha<1)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	rec, err := readRecordedOutcomes(*outcomesPath)
	if err != nil {
		return err
	}
	report, err := core.ComputeReliability(rec, *alpha)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(report)
	}
	fmt.Printf("runs=%d macro_pass_at_1=%.4f max_k=%d\n", report.Runs, report.MacroPassAt1, report.MaxK)
	for _, point := range report.PassPowKCurve {
		fmt.Printf("- pass^%d=%.4f\n", point.K, point.PassPowK)
	}
	return nil
}

func runBenchmarkConsensus(args []string) error {
	fs := flag.NewFlagSet("issueops benchmark consensus", flag.ContinueOnError)
	samplesPath := fs.String("samples", "", "offline-recorded judge samples JSON ({\"samples\":[{\"sample_id\":..,\"provenance\":..,\"score\":<IssueOpsBenchmarkScore>}]}); reads stdin when empty")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	samples, err := readJudgeSamples(*samplesPath)
	if err != nil {
		return err
	}
	verdict, err := core.ConsensusJudgeVerdict(samples)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(verdict)
	}
	fmt.Printf("samples=%d majority_passed=%t pass_agreement=%.4f median=%.4f spread=%.4f variance=%.4f\n",
		verdict.Samples, verdict.MajorityPassed, verdict.PassAgreement, verdict.MedianAverageScore, verdict.ScoreSpread, verdict.SampleVariance)
	fmt.Printf("caveat: %s\n", verdict.Caveat)
	return nil
}
