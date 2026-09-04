package benchmarkcmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	issueopscontract "issueops/internal/contract/issueops"
	"os"
	"strings"
)

var benchmarkSubcommands = map[string]func([]string) error{
	"run":         runBenchmarkRun,
	"compare":     runBenchmarkCompare,
	"gate":        runBenchmarkGate,
	"reliability": runBenchmarkReliability,
	"consensus":   runBenchmarkConsensus,
}

func Run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: issueops benchmark run|compare|gate|reliability|consensus [--json]")
		return nil
	}
	handler, ok := benchmarkSubcommands[args[0]]
	if !ok {
		return fmt.Errorf("unknown issueops benchmark subcommand %q", args[0])
	}
	return handler(args[1:])
}

// readJudgeSamples reads the offline-recorded judge samples JSON (file path, or
// stdin when the path is empty). ConsensusJudgeVerdict enforces the independence
// guard (distinct sample ids + non-empty provenance) so this loader only decodes.
func readJudgeSamples(path string) ([]issueopscontract.JudgeSample, error) {
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
	var payload struct {
		Samples []issueopscontract.JudgeSample `json:"samples"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse judge samples: %w", err)
	}
	return payload.Samples, nil
}

// readRecordedOutcomes reads the offline recorded-outcomes JSON (file path, or
// stdin when the path is empty). ComputeReliability enforces the provenance
// guard (distinct run ids + non-empty provenance) so this loader only decodes.
func readRecordedOutcomes(path string) (issueopscontract.RecordedOutcomes, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(path) == "" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return issueopscontract.RecordedOutcomes{}, err
	}
	var rec issueopscontract.RecordedOutcomes
	if err := json.Unmarshal(raw, &rec); err != nil {
		return issueopscontract.RecordedOutcomes{}, fmt.Errorf("parse recorded outcomes: %w", err)
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

// readIssueOpsJudgeMap reads the wrapped judge map
// {"source_run_id":..,"provenance":..,"scores":{"<fixtureID>":<score>}} (file
// path, or stdin when empty), strict-decodes each score, and fails closed when
// the scores' keys do not exactly match the run's fixture IDs. The wrapper is
// the ONLY accepted shape: a legacy flat {"<fixtureID>": <score>} map fails to
// decode (its fixture keys are unknown top-level fields) rather than silently
// bypassing the provenance gate. Missing keys must error here: merging a
// zero-value judge score would silently fail the fixture via JudgeFailures.
func readIssueOpsJudgeMap(path string, fixtures []issueopscontract.IssueOpsBenchmarkFixture) (issueopscontract.IssueOpsJudgeMap, map[string]issueopscontract.IssueOpsBenchmarkScore, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(path) == "" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return issueopscontract.IssueOpsJudgeMap{}, nil, err
	}
	var wrapper struct {
		SourceRunID string                     `json:"source_run_id"`
		Provenance  string                     `json:"provenance"`
		Scores      map[string]json.RawMessage `json:"scores"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wrapper); err != nil {
		return issueopscontract.IssueOpsJudgeMap{}, nil, fmt.Errorf("parse judge map (expect {\"source_run_id\":..,\"provenance\":..,\"scores\":{...}}): %w", err)
	}
	if wrapper.Scores == nil {
		return issueopscontract.IssueOpsJudgeMap{}, nil, fmt.Errorf("judge map missing \"scores\" object")
	}
	scores := make(map[string]issueopscontract.IssueOpsBenchmarkScore, len(wrapper.Scores))
	known := make(map[string]bool, len(fixtures))
	for _, fixture := range fixtures {
		known[fixture.ID] = true
		value, ok := wrapper.Scores[fixture.ID]
		if !ok {
			return issueopscontract.IssueOpsJudgeMap{}, nil, fmt.Errorf("judge map missing fixture %q", fixture.ID)
		}
		score, err := benchmarkDeps.DecodeIssueOpsBenchmarkJudgeJSON(value)
		if err != nil {
			return issueopscontract.IssueOpsJudgeMap{}, nil, fmt.Errorf("judge score for fixture %q: %w", fixture.ID, err)
		}
		scores[fixture.ID] = score
	}
	for key := range wrapper.Scores {
		if !known[key] {
			return issueopscontract.IssueOpsJudgeMap{}, nil, fmt.Errorf("judge map has unknown fixture %q", key)
		}
	}
	return issueopscontract.IssueOpsJudgeMap{SourceRunID: wrapper.SourceRunID, Provenance: wrapper.Provenance}, scores, nil
}

func readIssueOpsAutoresearchCandidateFile(path string) (issueopscontract.IssueOpsAutoresearchCandidate, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return issueopscontract.IssueOpsAutoresearchCandidate{}, fmt.Errorf("candidate-file is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return issueopscontract.IssueOpsAutoresearchCandidate{}, err
	}
	var candidate issueopscontract.IssueOpsAutoresearchCandidate
	if err := json.Unmarshal(b, &candidate); err != nil {
		return issueopscontract.IssueOpsAutoresearchCandidate{}, fmt.Errorf("parse candidate file %s: %w", path, err)
	}
	return candidate, nil
}
