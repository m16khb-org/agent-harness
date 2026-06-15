package benchmarkcmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"agent-harness/cmd/harness/issueopscli/benchmarkartifact"
	"agent-harness/internal/core"
)

func Run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops benchmark run|compare|gate|reliability [--json]")
		return nil
	}
	if len(args) == 0 {
		return fmt.Errorf("unknown issueops benchmark subcommand")
	}
	switch args[0] {
	case "run":
		fs := flag.NewFlagSet("issueops benchmark run", flag.ContinueOnError)
		fixturesPath := fs.String("fixtures", "", "benchmark fixtures path")
		judge := fs.String("judge", "agy", "judge backend: none, file, or agy (legacy: external agy -p; prefer file)")
		judgeFile := fs.String("judge-file", "", "judge score map JSON path for --judge file ({\"<fixtureID>\": <score>, ...}); reads stdin when empty")
		agyCommand := fs.String("agy-command", "agy", "agy command path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseFlags(fs, args[1:]); help || err != nil {
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
		switch *judge {
		case "agy":
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
		case "file":
			judgeScores, err := readIssueOpsJudgeScoreMap(*judgeFile, fixtures)
			if err != nil {
				return err
			}
			for i, fixture := range fixtures {
				result.Scores[i] = core.MergeIssueOpsBenchmarkScoreWithJudge(result.Scores[i], judgeScores[fixture.ID])
			}
		case "none":
		default:
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
		if help, err := parseFlags(fs, args[1:]); help || err != nil {
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
		if help, err := parseFlags(fs, args[1:]); help || err != nil {
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
	case "reliability":
		fs := flag.NewFlagSet("issueops benchmark reliability", flag.ContinueOnError)
		outcomesPath := fs.String("outcomes", "", "recorded offline outcomes JSON path ({\"runs\":[{\"run_id\":..,\"provenance\":..,\"outcomes\":{\"<fixtureID>\":<bool>}}]}); reads stdin when empty")
		alpha := fs.Float64("alpha", 0.05, "confidence level alpha for Clopper-Pearson intervals (0<alpha<1)")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseFlags(fs, args[1:]); help || err != nil {
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
	default:
		return fmt.Errorf("unknown issueops benchmark subcommand %q", args[0])
	}
}

// readRecordedOutcomes reads the offline recorded-outcomes JSON (file path, or
// stdin when the path is empty). ComputeReliability enforces the provenance
// guard (distinct run ids + non-empty provenance) so this loader only decodes.
func readRecordedOutcomes(path string) (core.RecordedOutcomes, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(path) == "" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return core.RecordedOutcomes{}, err
	}
	var rec core.RecordedOutcomes
	if err := json.Unmarshal(raw, &rec); err != nil {
		return core.RecordedOutcomes{}, fmt.Errorf("parse recorded outcomes: %w", err)
	}
	return rec, nil
}

func parseFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type repeatedFlag []string

func (f *repeatedFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// readIssueOpsJudgeScoreMap reads a {"<fixtureID>": <score>} map (file path,
// or stdin when the path is empty), strict-decodes each score value through
// the same decoder the agy judge uses, and fails closed when the map's keys
// do not exactly match the run's fixture IDs. Missing keys must error here:
// merging a zero-value judge score would silently fail the fixture via
// JudgeFailures instead of surfacing the operator mistake.
func readIssueOpsJudgeScoreMap(path string, fixtures []core.IssueOpsBenchmarkFixture) (map[string]core.IssueOpsBenchmarkScore, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(path) == "" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, fmt.Errorf("parse judge score map: %w", err)
	}
	scores := make(map[string]core.IssueOpsBenchmarkScore, len(outer))
	known := make(map[string]bool, len(fixtures))
	for _, fixture := range fixtures {
		known[fixture.ID] = true
		value, ok := outer[fixture.ID]
		if !ok {
			return nil, fmt.Errorf("judge score map missing fixture %q", fixture.ID)
		}
		score, err := core.DecodeIssueOpsBenchmarkJudgeJSON(value)
		if err != nil {
			return nil, fmt.Errorf("judge score for fixture %q: %w", fixture.ID, err)
		}
		scores[fixture.ID] = score
	}
	for key := range outer {
		if !known[key] {
			return nil, fmt.Errorf("judge score map has unknown fixture %q", key)
		}
	}
	return scores, nil
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
