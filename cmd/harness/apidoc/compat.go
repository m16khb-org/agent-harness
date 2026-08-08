package apidoc

type apiDocStaticViolation = StaticViolation

func checkNestControllerStatic(file, text string) []apiDocStaticViolation {
	return CheckNestControllerStatic(file, text)
}

func checkNestDTOStatic(file, text string) []apiDocStaticViolation {
	return CheckNestDTOStatic(file, text)
}

func buildAPIDocReviewPrompt(files []string, diff, extraPrompt string) string {
	return BuildReviewPrompt(files, diff, extraPrompt)
}

func apiDocReviewSchema() map[string]any {
	return ReviewSchema()
}

func apiDocReviewExtraPrompt(options apiDocReviewOptions) (string, error) {
	return ReviewExtraPrompt(options.Repo, options.PromptFile)
}

func apiDocDiff(repo string, files []string, diffFile string) (string, error) {
	return Diff(repo, files, diffFile)
}

func apiDocInput(repo string, files []string, diffFile string, all bool) (string, error) {
	return Input(repo, files, diffFile, all)
}

func stagedAPIDocFiles(repo string) []string {
	return StagedFiles(repo)
}

func trackedAPIDocFiles(repo string) []string {
	return TrackedFiles(repo)
}

func normalizeAPIDocFiles(repo string, files []string) []string {
	return NormalizeFiles(repo, files)
}

func isAPIDocCandidate(file string) bool {
	return IsCandidate(file)
}
