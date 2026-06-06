package selfworkflow

import (
	"encoding/json"
	"os"

	"agent-harness/cmd/harness/commandstep"
)

const (
	DefaultLoopTargetScoreExclusive     = defaultLoopTargetScoreExclusive
	SelfAugmentationLessonKind          = selfAugmentationLessonKind
	SelfAugmentationKoreanName          = selfAugmentationKoreanName
	SelfAugmentationPlanKind            = selfAugmentationPlanKind
	SelfVerificationKoreanName          = selfVerificationKoreanName
	SelfVerificationSummaryKind         = selfVerificationSummaryKind
	LegacySelfAugmentSummaryKind        = legacySelfAugmentSummaryKind
	SelfAugmentCandidateStatusOpen      = selfAugmentCandidateStatusOpen
	SelfAugmentCandidateStatusSatisfied = selfAugmentCandidateStatusSatisfied
)

const selfVerifyStepBudgetMinRegressionMS int64 = 25

type StepResult = commandstep.StepResult

var HarnessRoot = func() string {
	if root := os.Getenv("HARNESS_ROOT"); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

var Version = "dev"

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
