package selfworkflow

import (
	"time"

	"issueops/cmd/issueops/selfworkflow/stateio"
)

func IsSelfVerificationSummaryKind(kind string) bool {
	return stateio.IsSelfVerificationSummaryKind(kind)
}

func NewSelfVerificationSummarySnapshot(result SelfAugmentResult, generatedAt time.Time) SelfAugmentStateSnapshot {
	return stateio.NewSelfVerificationSummarySnapshot(result, generatedAt)
}

func PromoteSelfAugmentBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error) {
	return stateio.PromoteSelfAugmentBaseline(fromKey, baselineKey, confirm, allowFailedSource)
}

func ReadSelfAugmentStateSnapshot(key string) (SelfAugmentStateSnapshot, error) {
	return stateio.ReadSelfAugmentStateSnapshot(key)
}

func SaveSelfAugmentSummary(result *SelfAugmentResult, key string) error {
	return stateio.SaveSelfAugmentSummary(result, key)
}

func SaveSelfVerificationSummary(result *SelfAugmentResult, key string) error {
	return stateio.SaveSelfVerificationSummary(result, key)
}

func WriteSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	return stateio.WriteSelfAugmentSnapshotRecord(dir, key, snapshot)
}

func saveSelfAugmentPlan(result *SelfAugmentPlanResult, key string) error {
	return stateio.SaveSelfAugmentPlan(result, key)
}

func selfAugmentCandidateIDsByStatus(candidates []SelfAugmentCandidate, status string) []string {
	return stateio.SelfAugmentCandidateIDsByStatus(candidates, status)
}
