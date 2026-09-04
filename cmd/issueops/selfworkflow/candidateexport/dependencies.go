package candidateexport

import (
	"os"

	"issueops/cmd/issueops/selfworkflow/augmentcatalog"
	"issueops/cmd/issueops/selfworkflow/model"
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
