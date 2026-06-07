package selfworkflow

import (
	"time"

	"agent-harness/cmd/harness/selfworkflow/historycompare"
)

func ApplySelfAugmentHistoryRetention(result *SelfAugmentHistoryResult, options SelfAugmentHistoryRetentionOptions) error {
	return historycompare.ApplySelfAugmentHistoryRetention(result, options)
}

func AllSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool {
	return allSelfAugmentGoalsPassed(goals)
}

func BuildStepDurationStats(durationsByLabel map[string][]int64) []SelfAugmentStepDurationStat {
	return buildStepDurationStats(durationsByLabel)
}

func ClassifySelfVerificationFailure(result SelfAugmentResult, summary SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	return classifySelfVerificationFailure(result, summary)
}

func CollectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) SelfAugmentRepoSignals {
	return collectSelfAugmentRepoSignals(root, docsIndexed, skills, geniusText)
}

func CompareSlowestStepRegressions(baseline, candidate []SelfAugmentSlowStep, maxRegressionPct float64) []SelfAugmentSlowStepRegression {
	return compareSlowestStepRegressions(baseline, candidate, maxRegressionPct)
}

func CompareStepBudgetRegressions(baseline, candidate []SelfAugmentStepDurationStat, maxRegressionPct float64) []SelfAugmentStepBudgetRegression {
	return compareStepBudgetRegressions(baseline, candidate, maxRegressionPct)
}

func DocsContainTerm(root, term string) bool {
	return docsContainTerm(root, term)
}

func DirContainsTerm(root, relDir, term string) bool {
	return dirContainsTerm(root, relDir, term)
}

func FileContainsTerm(root, relPath, term string) bool {
	return fileContainsTerm(root, relPath, term)
}

func FormatScore(score float64) string {
	return formatScore(score)
}

func MarkSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	markSatisfiedSelfAugmentCandidate(candidate, signals)
}

func MaxSlowStepDurationByLabel(steps []SelfAugmentSlowStep) map[string]int64 {
	return maxSlowStepDurationByLabel(steps)
}

func MissingStrings(want, have []string) []string {
	return missingStrings(want, have)
}

func NonNilSlowStepSlice(items []SelfAugmentSlowStep) []SelfAugmentSlowStep {
	return nonNilSlowStepSlice(items)
}

func NonNilStringSlice(items []string) []string {
	return nonNilStringSlice(items)
}

func ParseSelfAugmentTimestamp(value string) (time.Time, bool) {
	return parseSelfAugmentTimestamp(value)
}

func PlanSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	return planSelfAugmentation(req)
}

func ScoreBool(ok bool) float64 {
	return scoreBool(ok)
}

func SaveSelfAugmentLesson(req SelfAugmentLessonRequest) (SelfAugmentLessonResult, error) {
	return saveSelfAugmentLesson(req)
}

func SaveSelfAugmentPlan(result *SelfAugmentPlanResult, key string) error {
	return saveSelfAugmentPlan(result, key)
}

func ScoreSelfVerificationGoals(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	return scoreSelfVerificationGoals(result, targetScore)
}

func SelectGeniusFormulas(text string) []string {
	return selectGeniusFormulas(text)
}

func SelectedCandidateID(candidate *SelfAugmentCandidate) string {
	return selectedCandidateID(candidate)
}

func SelfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return selfAugmentCandidates(signals)
}

func SelfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return selfAugmentCandidateScore(candidate)
}

func SelfAugmentCandidateIDsByStatus(candidates []SelfAugmentCandidate, status string) []string {
	return selfAugmentCandidateIDsByStatus(candidates, status)
}

func SelfAugmentResearchInfluences() []SelfAugmentInfluence {
	return selfAugmentResearchInfluences()
}

func BuildSelfVerificationContract() SelfVerificationContract {
	return selfVerificationContract()
}

func BuildSelfVerificationCoverage(stepLabels []string) ([]SelfVerificationCoverage, []string) {
	return selfVerificationCoverage(stepLabels)
}

func SelfVerificationCoverageDefinitions() []SelfVerificationCoverageDefinition {
	definitions := selfVerificationCoverageDefinitions()
	out := make([]SelfVerificationCoverageDefinition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, SelfVerificationCoverageDefinition(definition))
	}
	return out
}

func SelfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	return selfVerificationFailureClusters(result)
}

func SelfVerificationGoalDefinitions() []SelfVerificationGoalDefinition {
	definitions := selfVerificationGoalDefinitions()
	out := make([]SelfVerificationGoalDefinition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, SelfVerificationGoalDefinition(definition))
	}
	return out
}

func SelfVerifyRerunCommands(failedStep string, iterations int, baseSeed int64, targetScore float64) []string {
	return selfVerifyRerunCommands(failedStep, iterations, baseSeed, targetScore)
}

func SelfVerifyStepRerunCommand(label string) (string, bool) {
	return selfVerifyStepRerunCommand(label)
}

func StepDurationStatByLabel(stats []SelfAugmentStepDurationStat) map[string]SelfAugmentStepDurationStat {
	return stepDurationStatByLabel(stats)
}

func StepDurationStatsForCompare(summary SelfAugmentSummary) []SelfAugmentStepDurationStat {
	return stepDurationStatsForCompare(summary)
}

func RunSelfAugmentLesson(args []string) error {
	return runSelfAugmentLesson(args)
}

func StateKeySlug(s string) string {
	return stateKeySlug(s)
}

func SummarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return summarizeSelfAugment(result)
}

func SummarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	return summarizeSelfVerification(result, targetScore)
}
