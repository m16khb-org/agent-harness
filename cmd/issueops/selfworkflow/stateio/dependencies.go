package stateio

import (
	"issueops/cmd/issueops/selfworkflow/augmentcatalog"
	"issueops/cmd/issueops/selfworkflow/model"
)

const (
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
