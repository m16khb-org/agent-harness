package augmentcatalog

import "agent-harness/cmd/harness/selfworkflow/model"

const (
	SelfAugmentCandidateStatusOpen      = "open"
	SelfAugmentCandidateStatusSatisfied = "already_satisfied"
)

const (
	selfAugmentCandidateStatusOpen      = SelfAugmentCandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = SelfAugmentCandidateStatusSatisfied
)

type SelfAugmentCandidate = model.SelfAugmentCandidate
type SelfAugmentGoal = model.SelfAugmentGoal
type SelfAugmentInfluence = model.SelfAugmentInfluence
type SelfAugmentRepoSignals = model.SelfAugmentRepoSignals

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func docsContainTerm(root, term string) bool {
	return DocsContainTerm(root, term)
}

func dirContainsTerm(root, relDir, term string) bool {
	return DirContainsTerm(root, relDir, term)
}

func fileContainsTerm(root, relPath, term string) bool {
	return FileContainsTerm(root, relPath, term)
}
