package selfworkflow

import (
	"issueops/cmd/issueops/selfworkflow/augmentcatalog"
	"issueops/cmd/issueops/selfworkflow/candidateexport"
)

const SelfVerificationCandidateExportKind = candidateexport.SelfVerificationCandidateExportKind

const (
	selfAugmentCandidateStatusOpen      = augmentcatalog.SelfAugmentCandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = augmentcatalog.SelfAugmentCandidateStatusSatisfied
)

type SelfVerificationCandidate = candidateexport.SelfVerificationCandidate
type SelfVerificationCandidateExportResult = candidateexport.SelfVerificationCandidateExportResult
type SelfVerificationCandidateExportStateSnapshot = candidateexport.SelfVerificationCandidateExportStateSnapshot
