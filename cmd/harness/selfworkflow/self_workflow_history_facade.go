package selfworkflow

import (
	"time"

	"agent-harness/cmd/harness/selfworkflow/historycompare"
)

func CompareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (SelfAugmentCompareResult, error) {
	return historycompare.CompareSelfAugmentSummaries(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate SelfAugmentStateSnapshot) SelfAugmentCompareResult {
	return historycompare.CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate)
}

func NewSelfAugmentCompareResult(baselineKey, candidateKey string, maxElapsedRegressionPct float64) SelfAugmentCompareResult {
	return historycompare.NewSelfAugmentCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func SelfAugmentHistory(prefix string, limit int, retentionOptions ...SelfAugmentHistoryRetentionOptions) (SelfAugmentHistoryResult, error) {
	return historycompare.SelfAugmentHistory(prefix, limit, retentionOptions...)
}

func RunSelfVerifyCompare(args []string) error {
	return historycompare.RunSelfVerifyCompare(args, historyCLIDeps())
}

func RunSelfVerifyHistory(args []string) error {
	return historycompare.RunSelfVerifyHistory(args, historyCLIDeps())
}

func historyCLIDeps() historycompare.CLIDeps {
	return historycompare.CLIDeps{PrintJSON: printJSON}
}

func compareSlowestStepRegressions(baseline, candidate []SelfAugmentSlowStep, maxRegressionPct float64) []SelfAugmentSlowStepRegression {
	return historycompare.CompareSlowestStepRegressions(baseline, candidate, maxRegressionPct)
}

func compareStepBudgetRegressions(baseline, candidate []SelfAugmentStepDurationStat, maxRegressionPct float64) []SelfAugmentStepBudgetRegression {
	return historycompare.CompareStepBudgetRegressions(baseline, candidate, maxRegressionPct)
}

func missingStrings(want, have []string) []string {
	return historycompare.MissingStrings(want, have)
}

func nonNilSlowStepSlice(items []SelfAugmentSlowStep) []SelfAugmentSlowStep {
	return historycompare.NonNilSlowStepSlice(items)
}

func nonNilStringSlice(items []string) []string {
	return historycompare.NonNilStringSlice(items)
}

func parseSelfAugmentTimestamp(value string) (time.Time, bool) {
	return historycompare.ParseSelfAugmentTimestamp(value)
}
