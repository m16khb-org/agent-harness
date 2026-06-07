package selfworkflow

import (
	"agent-harness/cmd/harness/selfworkflow/augmentcatalog"
	"agent-harness/cmd/harness/selfworkflow/candidateexport"
)

const SelfVerificationCandidateExportKind = candidateexport.SelfVerificationCandidateExportKind

const (
	selfAugmentCandidateStatusOpen      = augmentcatalog.SelfAugmentCandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = augmentcatalog.SelfAugmentCandidateStatusSatisfied
)

type SelfVerificationCandidate = candidateexport.SelfVerificationCandidate
type SelfVerificationCandidateExportResult = candidateexport.SelfVerificationCandidateExportResult
type SelfVerificationCandidateExportStateSnapshot = candidateexport.SelfVerificationCandidateExportStateSnapshot
