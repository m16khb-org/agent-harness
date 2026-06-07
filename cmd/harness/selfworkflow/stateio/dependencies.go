package stateio

import (
	"agent-harness/cmd/harness/selfworkflow/augmentcatalog"
	"agent-harness/cmd/harness/selfworkflow/model"
)

const (
	legacySelfAugmentSummaryKind        = model.LegacySelfAugmentSummaryKind
	selfAugmentCandidateStatusOpen      = augmentcatalog.SelfAugmentCandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = augmentcatalog.SelfAugmentCandidateStatusSatisfied
	selfAugmentationPlanKind            = model.SelfAugmentationPlanKind
	selfVerificationSummaryKind         = model.SelfVerificationSummaryKind
)

type SelfAugmentCandidate = model.SelfAugmentCandidate
type SelfAugmentPlanResult = model.SelfAugmentPlanResult
type SelfAugmentPlanStateSnapshot = model.SelfAugmentPlanStateSnapshot
type SelfAugmentPromoteResult = model.SelfAugmentPromoteResult
type SelfAugmentResult = model.SelfAugmentResult
type SelfAugmentStateCheckpoint = model.SelfAugmentStateCheckpoint
type SelfAugmentStateSnapshot = model.SelfAugmentStateSnapshot
