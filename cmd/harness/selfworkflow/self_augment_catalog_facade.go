package selfworkflow

import "agent-harness/cmd/harness/selfworkflow/augmentcatalog"

const (
	selfAugmentCandidateStatusOpen      = augmentcatalog.SelfAugmentCandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = augmentcatalog.SelfAugmentCandidateStatusSatisfied
)

func allSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool {
	return augmentcatalog.AllSelfAugmentGoalsPassed(goals)
}

func collectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) SelfAugmentRepoSignals {
	return augmentcatalog.CollectSelfAugmentRepoSignals(root, docsIndexed, skills, geniusText)
}

func docsContainTerm(root, term string) bool {
	return augmentcatalog.DocsContainTerm(root, term)
}

func dirContainsTerm(root, relDir, term string) bool {
	return augmentcatalog.DirContainsTerm(root, relDir, term)
}

func fileContainsTerm(root, relPath, term string) bool {
	return augmentcatalog.FileContainsTerm(root, relPath, term)
}

func markSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	augmentcatalog.MarkSatisfiedSelfAugmentCandidate(candidate, signals)
}

func scoreBool(ok bool) float64 {
	return augmentcatalog.ScoreBool(ok)
}

func selectedCandidateID(candidate *SelfAugmentCandidate) string {
	return augmentcatalog.SelectedCandidateID(candidate)
}

func selectGeniusFormulas(text string) []string {
	return augmentcatalog.SelectGeniusFormulas(text)
}

func selfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return augmentcatalog.SelfAugmentCandidates(signals)
}

func selfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return augmentcatalog.SelfAugmentCandidateScore(candidate)
}

func selfAugmentResearchInfluences() []SelfAugmentInfluence {
	return augmentcatalog.SelfAugmentResearchInfluences()
}
