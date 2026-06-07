package selfworkflow

import (
	"encoding/json"
	"os"

	"agent-harness/cmd/harness/commandstep"
	"agent-harness/cmd/harness/selfworkflow/model"
)

const (
	DefaultLoopTargetScoreExclusive     = model.DefaultLoopTargetScoreExclusive
	SelfAugmentationLessonKind          = model.SelfAugmentationLessonKind
	SelfAugmentationKoreanName          = model.SelfAugmentationKoreanName
	SelfAugmentationPlanKind            = model.SelfAugmentationPlanKind
	SelfVerificationKoreanName          = model.SelfVerificationKoreanName
	SelfVerificationSummaryKind         = model.SelfVerificationSummaryKind
	LegacySelfAugmentSummaryKind        = model.LegacySelfAugmentSummaryKind
	SelfAugmentCandidateStatusOpen      = selfAugmentCandidateStatusOpen
	SelfAugmentCandidateStatusSatisfied = selfAugmentCandidateStatusSatisfied
)

const (
	defaultLoopTargetScoreExclusive = model.DefaultLoopTargetScoreExclusive
	selfAugmentationLessonKind      = model.SelfAugmentationLessonKind
	selfAugmentationKoreanName      = model.SelfAugmentationKoreanName
	selfAugmentationPlanKind        = model.SelfAugmentationPlanKind
	selfVerificationKoreanName      = model.SelfVerificationKoreanName
	selfVerificationSummaryKind     = model.SelfVerificationSummaryKind
	legacySelfAugmentSummaryKind    = model.LegacySelfAugmentSummaryKind
)

const selfVerifyStepBudgetMinRegressionMS int64 = 25

type StepResult = commandstep.StepResult

type SelfAugmentCandidate = model.SelfAugmentCandidate
type SelfAugmentCompareResult = model.SelfAugmentCompareResult
type SelfAugmentGoal = model.SelfAugmentGoal
type SelfAugmentHistoryEntry = model.SelfAugmentHistoryEntry
type SelfAugmentHistoryResult = model.SelfAugmentHistoryResult
type SelfAugmentHistoryRetention = model.SelfAugmentHistoryRetention
type SelfAugmentHistoryRetentionOptions = model.SelfAugmentHistoryRetentionOptions
type SelfAugmentHistorySkipped = model.SelfAugmentHistorySkipped
type SelfAugmentInfluence = model.SelfAugmentInfluence
type SelfAugmentIteration = model.SelfAugmentIteration
type SelfAugmentLessonRequest = model.SelfAugmentLessonRequest
type SelfAugmentLessonResult = model.SelfAugmentLessonResult
type SelfAugmentLessonStateSnapshot = model.SelfAugmentLessonStateSnapshot
type SelfAugmentPlanRequest = model.SelfAugmentPlanRequest
type SelfAugmentPlanResult = model.SelfAugmentPlanResult
type SelfAugmentPlanStateSnapshot = model.SelfAugmentPlanStateSnapshot
type SelfAugmentPromoteResult = model.SelfAugmentPromoteResult
type SelfAugmentRepoSignals = model.SelfAugmentRepoSignals
type SelfAugmentResult = model.SelfAugmentResult
type SelfAugmentSlowStep = model.SelfAugmentSlowStep
type SelfAugmentStateCheckpoint = model.SelfAugmentStateCheckpoint
type SelfAugmentStateSnapshot = model.SelfAugmentStateSnapshot
type SelfAugmentStepBudgetRegression = model.SelfAugmentStepBudgetRegression
type SelfAugmentStepDurationStat = model.SelfAugmentStepDurationStat
type SelfAugmentSummary = model.SelfAugmentSummary
type SelfAugmentSlowStepRegression = model.SelfAugmentSlowStepRegression
type SelfVerificationContract = model.SelfVerificationContract
type SelfVerificationCoverage = model.SelfVerificationCoverage
type SelfVerificationCoverageDefinition = model.SelfVerificationCoverageDefinition
type SelfVerificationFailureCluster = model.SelfVerificationFailureCluster
type SelfVerificationGoalDefinition = model.SelfVerificationGoalDefinition
type SelfVerificationGoalScore = model.SelfVerificationGoalScore

type selfAugmentHistoryRetentionOptions = model.SelfAugmentHistoryRetentionOptions
type selfVerificationCoverageDefinition = model.SelfVerificationCoverageDefinition
type selfVerificationGoalDefinition = model.SelfVerificationGoalDefinition

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
