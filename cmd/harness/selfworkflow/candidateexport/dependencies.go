package candidateexport

import (
	"os"

	"agent-harness/cmd/harness/selfworkflow/augmentcatalog"
	"agent-harness/cmd/harness/selfworkflow/model"
)

const (
	selfAugmentCandidateStatusOpen      = augmentcatalog.SelfAugmentCandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = augmentcatalog.SelfAugmentCandidateStatusSatisfied
	selfVerificationKoreanName          = model.SelfVerificationKoreanName
)

type SelfAugmentStateCheckpoint = model.SelfAugmentStateCheckpoint

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
