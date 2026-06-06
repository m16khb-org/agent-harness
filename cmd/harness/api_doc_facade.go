package main

import "agent-harness/cmd/harness/apidoc"

type apiDocStaticViolation = apidoc.StaticViolation

func checkNestControllerStatic(file, text string) []apiDocStaticViolation {
	return apidoc.CheckNestControllerStatic(file, text)
}

func checkNestDTOStatic(file, text string) []apiDocStaticViolation {
	return apidoc.CheckNestDTOStatic(file, text)
}

func buildAPIDocReviewPrompt(files []string, diff, extraPrompt string) string {
	return apidoc.BuildReviewPrompt(files, diff, extraPrompt)
}

func apiDocReviewSchema() map[string]any {
	return apidoc.ReviewSchema()
}

func apiDocReviewExtraPrompt(options apiDocReviewOptions) (string, error) {
	return apidoc.ReviewExtraPrompt(options.Repo, options.PromptFile)
}

func apiDocDiff(repo string, files []string, diffFile string) (string, error) {
	return apidoc.Diff(repo, files, diffFile)
}

func apiDocInput(repo string, files []string, diffFile string, all bool) (string, error) {
	return apidoc.Input(repo, files, diffFile, all)
}

func apiDocFullContent(repo string, files []string) (string, error) {
	return apidoc.FullContent(repo, files)
}

func stagedAPIDocFiles(repo string) []string {
	return apidoc.StagedFiles(repo)
}

func trackedAPIDocFiles(repo string) []string {
	return apidoc.TrackedFiles(repo)
}

func normalizeAPIDocFiles(repo string, files []string) []string {
	return apidoc.NormalizeFiles(repo, files)
}

func isAPIDocCandidate(file string) bool {
	return apidoc.IsCandidate(file)
}
