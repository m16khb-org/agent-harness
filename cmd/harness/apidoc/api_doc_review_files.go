package apidoc

import (
	"agent-harness/cmd/harness/apidoc/reviewfiles"
)

func ReviewExtraPrompt(repo, promptFile string) (string, error) {
	return reviewfiles.ExtraPrompt(repo, promptFile)
}

func Diff(repo string, files []string, diffFile string) (string, error) {
	return reviewfiles.Diff(repo, files, diffFile)
}

func Input(repo string, files []string, diffFile string, all bool) (string, error) {
	return reviewfiles.Input(repo, files, diffFile, all)
}

func FullContent(repo string, files []string) (string, error) {
	return reviewfiles.FullContent(repo, files)
}

func StagedFiles(repo string) []string {
	return reviewfiles.Staged(repo)
}

func TrackedFiles(repo string) []string {
	return reviewfiles.Tracked(repo)
}

func NormalizeFiles(repo string, files []string) []string {
	return reviewfiles.Normalize(repo, files)
}

func IsCandidate(file string) bool {
	return reviewfiles.IsCandidate(file)
}
