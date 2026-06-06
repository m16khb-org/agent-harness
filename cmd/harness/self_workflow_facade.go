package main

import (
	"time"

	"agent-harness/cmd/harness/selfworkflow"
)

const (
	selfVerificationSummaryKind         = selfworkflow.SelfVerificationSummaryKind
	legacySelfAugmentSummaryKind        = selfworkflow.LegacySelfAugmentSummaryKind
	selfVerificationKoreanName          = selfworkflow.SelfVerificationKoreanName
	selfAugmentationLessonKind          = selfworkflow.SelfAugmentationLessonKind
	selfAugmentationKoreanName          = selfworkflow.SelfAugmentationKoreanName
	selfAugmentationPlanKind            = selfworkflow.SelfAugmentationPlanKind
	defaultLoopTargetScoreExclusive     = selfworkflow.DefaultLoopTargetScoreExclusive
	selfAugmentCandidateStatusOpen      = selfworkflow.SelfAugmentCandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = selfworkflow.SelfAugmentCandidateStatusSatisfied
)

type SelfAugmentStateCheckpoint = selfworkflow.SelfAugmentStateCheckpoint
type SelfAugmentPromoteResult = selfworkflow.SelfAugmentPromoteResult
type SelfAugmentIteration = selfworkflow.SelfAugmentIteration
type SelfAugmentCompareResult = selfworkflow.SelfAugmentCompareResult
type SelfAugmentSlowStepRegression = selfworkflow.SelfAugmentSlowStepRegression
type SelfAugmentStepBudgetRegression = selfworkflow.SelfAugmentStepBudgetRegression
type SelfAugmentHistoryResult = selfworkflow.SelfAugmentHistoryResult
type SelfAugmentHistoryRetention = selfworkflow.SelfAugmentHistoryRetention
type selfAugmentHistoryRetentionOptions = selfworkflow.SelfAugmentHistoryRetentionOptions
type SelfAugmentHistoryEntry = selfworkflow.SelfAugmentHistoryEntry
type SelfAugmentHistorySkipped = selfworkflow.SelfAugmentHistorySkipped
type SelfAugmentPlanRequest = selfworkflow.SelfAugmentPlanRequest
type SelfAugmentPlanResult = selfworkflow.SelfAugmentPlanResult
type SelfAugmentInfluence = selfworkflow.SelfAugmentInfluence
type SelfAugmentGoal = selfworkflow.SelfAugmentGoal
type SelfAugmentCandidate = selfworkflow.SelfAugmentCandidate
type SelfAugmentRepoSignals = selfworkflow.SelfAugmentRepoSignals
type SelfAugmentLessonRequest = selfworkflow.SelfAugmentLessonRequest
type SelfAugmentLessonResult = selfworkflow.SelfAugmentLessonResult
type SelfAugmentLessonStateSnapshot = selfworkflow.SelfAugmentLessonStateSnapshot
type SelfAugmentPlanStateSnapshot = selfworkflow.SelfAugmentPlanStateSnapshot
type SelfAugmentStateSnapshot = selfworkflow.SelfAugmentStateSnapshot
type SelfAugmentResult = selfworkflow.SelfAugmentResult
type SelfAugmentSummary = selfworkflow.SelfAugmentSummary
type SelfVerifyLLMEvalResult = selfworkflow.SelfVerifyLLMEvalResult
type SelfVerificationContract = selfworkflow.SelfVerificationContract
type selfVerificationCoverageDefinition = selfworkflow.SelfVerificationCoverageDefinition
type SelfVerificationGoalScore = selfworkflow.SelfVerificationGoalScore
type SelfVerificationCoverage = selfworkflow.SelfVerificationCoverage
type SelfVerificationFailureCluster = selfworkflow.SelfVerificationFailureCluster
type SelfAugmentSlowStep = selfworkflow.SelfAugmentSlowStep
type SelfAugmentStepDurationStat = selfworkflow.SelfAugmentStepDurationStat

func applySelfAugmentHistoryRetention(result *SelfAugmentHistoryResult, options selfAugmentHistoryRetentionOptions) error {
	return selfworkflow.ApplySelfAugmentHistoryRetention(result, options)
}

func parseSelfAugmentTimestamp(value string) (time.Time, bool) {
	return selfworkflow.ParseSelfAugmentTimestamp(value)
}

func nonNilStringSlice(items []string) []string {
	return selfworkflow.NonNilStringSlice(items)
}

func nonNilSlowStepSlice(items []SelfAugmentSlowStep) []SelfAugmentSlowStep {
	return selfworkflow.NonNilSlowStepSlice(items)
}

func collectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) SelfAugmentRepoSignals {
	return selfworkflow.CollectSelfAugmentRepoSignals(root, docsIndexed, skills, geniusText)
}

func selfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return selfworkflow.SelfAugmentCandidates(signals)
}

func scoreBool(ok bool) float64 {
	return selfworkflow.ScoreBool(ok)
}

func allSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool {
	return selfworkflow.AllSelfAugmentGoalsPassed(goals)
}

func selectedCandidateID(candidate *SelfAugmentCandidate) string {
	return selfworkflow.SelectedCandidateID(candidate)
}

func docsContainTerm(root, term string) bool {
	return selfworkflow.DocsContainTerm(root, term)
}

func fileContainsTerm(root, relPath, term string) bool {
	return selfworkflow.FileContainsTerm(root, relPath, term)
}

func dirContainsTerm(root, relDir, term string) bool {
	return selfworkflow.DirContainsTerm(root, relDir, term)
}

func selectGeniusFormulas(text string) []string {
	return selfworkflow.SelectGeniusFormulas(text)
}

func selfAugmentResearchInfluences() []SelfAugmentInfluence {
	return selfworkflow.SelfAugmentResearchInfluences()
}

func markSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	selfworkflow.MarkSatisfiedSelfAugmentCandidate(candidate, signals)
}

func selfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return selfworkflow.SelfAugmentCandidateScore(candidate)
}

func compareSlowestStepRegressions(baseline, candidate []SelfAugmentSlowStep, maxRegressionPct float64) []SelfAugmentSlowStepRegression {
	return selfworkflow.CompareSlowestStepRegressions(baseline, candidate, maxRegressionPct)
}

func compareStepBudgetRegressions(baseline, candidate []SelfAugmentStepDurationStat, maxRegressionPct float64) []SelfAugmentStepBudgetRegression {
	return selfworkflow.CompareStepBudgetRegressions(baseline, candidate, maxRegressionPct)
}

func missingStrings(want, have []string) []string {
	return selfworkflow.MissingStrings(want, have)
}

func stepDurationStatByLabel(stats []SelfAugmentStepDurationStat) map[string]SelfAugmentStepDurationStat {
	return selfworkflow.StepDurationStatByLabel(stats)
}

func maxSlowStepDurationByLabel(steps []SelfAugmentSlowStep) map[string]int64 {
	return selfworkflow.MaxSlowStepDurationByLabel(steps)
}

func buildStepDurationStats(durationsByLabel map[string][]int64) []SelfAugmentStepDurationStat {
	return selfworkflow.BuildStepDurationStats(durationsByLabel)
}

func stepDurationStatsForCompare(summary SelfAugmentSummary) []SelfAugmentStepDurationStat {
	return selfworkflow.StepDurationStatsForCompare(summary)
}

func summarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return selfworkflow.SummarizeSelfAugment(result)
}

func summarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	return selfworkflow.SummarizeSelfVerification(result, targetScore)
}

func classifySelfVerificationFailure(result SelfAugmentResult, summary SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	return selfworkflow.ClassifySelfVerificationFailure(result, summary)
}

func selfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	return selfworkflow.SelfVerificationFailureClusters(result)
}

func selfVerifyRerunCommands(failedStep string, iterations int, baseSeed int64, targetScore float64) []string {
	return selfworkflow.SelfVerifyRerunCommands(failedStep, iterations, baseSeed, targetScore)
}

func selfVerifyStepRerunCommand(label string) (string, bool) {
	return selfworkflow.SelfVerifyStepRerunCommand(label)
}

func formatScore(score float64) string {
	return selfworkflow.FormatScore(score)
}

func scoreSelfVerificationGoals(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	return selfworkflow.ScoreSelfVerificationGoals(result, targetScore)
}

func selfVerificationContract() SelfVerificationContract {
	return selfworkflow.BuildSelfVerificationContract()
}

func selfVerificationCoverage(stepLabels []string) ([]SelfVerificationCoverage, []string) {
	return selfworkflow.BuildSelfVerificationCoverage(stepLabels)
}

func selfVerificationCoverageDefinitions() []selfVerificationCoverageDefinition {
	return selfworkflow.SelfVerificationCoverageDefinitions()
}
