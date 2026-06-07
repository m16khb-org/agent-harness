package selfworkflow

import (
	"time"

	"agent-harness/cmd/harness/selfworkflow/augmentcatalog"
	"agent-harness/cmd/harness/selfworkflow/augmentplan"
	"agent-harness/cmd/harness/selfworkflow/historycompare"
)

func ApplySelfAugmentHistoryRetention(result *SelfAugmentHistoryResult, options SelfAugmentHistoryRetentionOptions) error {
	return historycompare.ApplySelfAugmentHistoryRetention(result, options)
}

func allSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool {
	return augmentcatalog.AllSelfAugmentGoalsPassed(goals)
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

func collectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) SelfAugmentRepoSignals {
	return augmentcatalog.CollectSelfAugmentRepoSignals(root, docsIndexed, skills, geniusText)
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

func docsContainTerm(root, term string) bool {
	return augmentcatalog.DocsContainTerm(root, term)
}

func DirContainsTerm(root, relDir, term string) bool {
	return dirContainsTerm(root, relDir, term)
}

func dirContainsTerm(root, relDir, term string) bool {
	return augmentcatalog.DirContainsTerm(root, relDir, term)
}

func FileContainsTerm(root, relPath, term string) bool {
	return fileContainsTerm(root, relPath, term)
}

func fileContainsTerm(root, relPath, term string) bool {
	return augmentcatalog.FileContainsTerm(root, relPath, term)
}

func FormatScore(score float64) string {
	return formatScore(score)
}

func MarkSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	markSatisfiedSelfAugmentCandidate(candidate, signals)
}

func markSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	augmentcatalog.MarkSatisfiedSelfAugmentCandidate(candidate, signals)
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

func planSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	return augmentplan.Plan(req, HarnessRoot(), Version)
}

func ScoreBool(ok bool) float64 {
	return scoreBool(ok)
}

func scoreBool(ok bool) float64 {
	return augmentcatalog.ScoreBool(ok)
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

func selectGeniusFormulas(text string) []string {
	return augmentcatalog.SelectGeniusFormulas(text)
}

func SelectedCandidateID(candidate *SelfAugmentCandidate) string {
	return selectedCandidateID(candidate)
}

func selectedCandidateID(candidate *SelfAugmentCandidate) string {
	return augmentcatalog.SelectedCandidateID(candidate)
}

func SelfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return selfAugmentCandidates(signals)
}

func selfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return augmentcatalog.SelfAugmentCandidates(signals)
}

func SelfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return selfAugmentCandidateScore(candidate)
}

func selfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return augmentcatalog.SelfAugmentCandidateScore(candidate)
}

func SelfAugmentCandidateIDsByStatus(candidates []SelfAugmentCandidate, status string) []string {
	return selfAugmentCandidateIDsByStatus(candidates, status)
}

func SelfAugmentResearchInfluences() []SelfAugmentInfluence {
	return selfAugmentResearchInfluences()
}

func selfAugmentResearchInfluences() []SelfAugmentInfluence {
	return augmentcatalog.SelfAugmentResearchInfluences()
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
