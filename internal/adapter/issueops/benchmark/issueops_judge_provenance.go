package benchmark

import (
	"fmt"
	"strings"
)

// runReader resolves a persisted run id to confirm it exists; ReadIssueOpsBenchmarkRun
// satisfies it. Injected so tests need not touch global state.
type runReader func(stateRoot, id string) (IssueOpsBenchmarkRunResult, error)

// ValidateJudgeProvenance fails closed when a --judge file map lacks provenance
// or is self-attributed to the scored run. See the type doc for the honest
// scope of this guard.
func ValidateJudgeProvenance(judge IssueOpsJudgeMap, scoredRunID, stateRoot string) error {
	return validateJudgeProvenance(judge, scoredRunID, stateRoot, ReadIssueOpsBenchmarkRun)
}

func validateJudgeProvenance(judge IssueOpsJudgeMap, scoredRunID, stateRoot string, read runReader) error {
	sourceID := strings.TrimSpace(judge.SourceRunID)
	if sourceID == "" {
		return fmt.Errorf("judge map missing source_run_id (judge provenance is required for --judge file)")
	}
	if strings.TrimSpace(judge.Provenance) == "" {
		return fmt.Errorf("judge map missing provenance label (name how the judge scores were produced)")
	}
	if sourceID == strings.TrimSpace(scoredRunID) {
		return fmt.Errorf("judge map source_run_id %q is the scored run itself — a self-attributed judge map (one run dressed as a judge of itself) is rejected", sourceID)
	}
	if _, err := read(stateRoot, sourceID); err != nil {
		return fmt.Errorf("judge map source_run_id %q does not resolve to a persisted run: %w", sourceID, err)
	}
	return nil
}

// JudgeDownwardOverrideRate reports the fraction of COMPARABLE dimensions where
// the judge lowered the deterministic score. comparable = dimensions that are
// not N/A in the deterministic score AND were scored by the judge — the exact
// set MergeIssueOpsBenchmarkScoreWithJudge can act on.
//
// This is NOT a symmetric "agreement" metric: the deterministic scorer is
// bimodal (0/100) and the merge only lets the judge LOWER a dimension, so the
// rate measures downward divergence, not agreement. On a clean 100/100 run the
// judge has no downward room, so a 0 rate means "no override", not "agreement".
func JudgeDownwardOverrideRate(deterministic, judge IssueOpsBenchmarkScore) (rate float64, comparable int) {
	judgeByDimension := make(map[string]float64, len(judge.DimensionScores))
	for _, dim := range judge.DimensionScores {
		judgeByDimension[dim.Dimension] = dim.Score
	}
	lowered := 0
	for _, dim := range deterministic.DimensionScores {
		if dim.NotApplicable {
			continue
		}
		judgeScore, ok := judgeByDimension[dim.Dimension]
		if !ok {
			continue
		}
		comparable++
		if judgeScore < dim.Score {
			lowered++
		}
	}
	if comparable == 0 {
		return 0, 0
	}
	return round4(float64(lowered) / float64(comparable)), comparable
}
