package main

import "agent-harness/cmd/harness/selfworkflow"

type selfVerifyPromoteDeps struct {
	promote func(fromKey, baselineKey string, confirm bool) (SelfAugmentPromoteResult, error)
}

func compareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (SelfAugmentCompareResult, error) {
	return selfworkflow.CompareSelfAugmentSummaries(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func compareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate SelfAugmentStateSnapshot) SelfAugmentCompareResult {
	return selfworkflow.CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate)
}

func newSelfAugmentCompareResult(baselineKey, candidateKey string, maxElapsedRegressionPct float64) SelfAugmentCompareResult {
	return selfworkflow.NewSelfAugmentCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func runSelfVerifyCompare(args []string) error {
	return selfworkflow.RunSelfVerifyCompare(args)
}

func runSelfVerifyHistory(args []string) error {
	return selfworkflow.RunSelfVerifyHistory(args)
}

func selfAugmentHistory(prefix string, limit int, retentionOptions ...selfAugmentHistoryRetentionOptions) (SelfAugmentHistoryResult, error) {
	options := []selfworkflow.SelfAugmentHistoryRetentionOptions{}
	for _, option := range retentionOptions {
		options = append(options, selfworkflow.SelfAugmentHistoryRetentionOptions(option))
	}
	return selfworkflow.SelfAugmentHistory(prefix, limit, options...)
}

func runSelfVerifyPromote(args []string) error {
	return selfworkflow.RunSelfVerifyPromote(args)
}

func runSelfVerifyPromoteWithDeps(args []string, deps selfVerifyPromoteDeps) error {
	return selfworkflow.RunSelfVerifyPromoteWithDeps(args, selfworkflow.SelfVerifyPromoteDeps{Promote: deps.promote})
}

func promoteSelfAugmentBaseline(fromKey, baselineKey string, confirm bool) (SelfAugmentPromoteResult, error) {
	return selfworkflow.PromoteSelfAugmentBaseline(fromKey, baselineKey, confirm)
}

func readSelfAugmentStateSnapshot(key string) (SelfAugmentStateSnapshot, error) {
	return selfworkflow.ReadSelfAugmentStateSnapshot(key)
}

func isSelfVerificationSummaryKind(kind string) bool {
	return selfworkflow.IsSelfVerificationSummaryKind(kind)
}

func writeSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	return selfworkflow.WriteSelfAugmentSnapshotRecord(dir, key, snapshot)
}
