package summary

import (
	"issueops/cmd/issueops/commandstep"
	"issueops/cmd/issueops/selfworkflow/model"
)

const defaultLoopTargetScoreExclusive = model.DefaultLoopTargetScoreExclusive

type SelfAugmentResult = model.SelfAugmentResult
type SelfAugmentSlowStep = model.SelfAugmentSlowStep
type SelfAugmentSlowStepRegression = model.SelfAugmentSlowStepRegression
type SelfAugmentStepBudgetRegression = model.SelfAugmentStepBudgetRegression
type SelfAugmentStepDurationStat = model.SelfAugmentStepDurationStat
type SelfAugmentSummary = model.SelfAugmentSummary
type SelfVerificationContract = model.SelfVerificationContract
type SelfVerificationCoverage = model.SelfVerificationCoverage
type SelfVerificationCoverageDefinition = model.SelfVerificationCoverageDefinition
type SelfVerificationFailureCluster = model.SelfVerificationFailureCluster
type SelfVerificationGoalDefinition = model.SelfVerificationGoalDefinition
type SelfVerificationGoalScore = model.SelfVerificationGoalScore
type StepResult = commandstep.StepResult
