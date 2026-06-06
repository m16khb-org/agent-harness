package core

import "agent-harness/internal/core/commitsuggest"

type CommitSuggestRequest = commitsuggest.CommitSuggestRequest
type CommitSuggestResult = commitsuggest.CommitSuggestResult

func SuggestCommit(req CommitSuggestRequest) (CommitSuggestResult, error) {
	return commitsuggest.SuggestCommit(req)
}

func buildCommitSuggestPrompt(diff string) string {
	return commitsuggest.BuildPrompt(diff)
}
