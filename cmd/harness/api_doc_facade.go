package main

import (
	"encoding/json"

	"agent-harness/cmd/harness/apidoc"
)

const (
	defaultAPIDocReviewModel     = apidoc.DefaultReviewModel
	defaultAPIDocReviewReasoning = apidoc.DefaultReviewReasoning
	defaultAPIDocReviewTimeout   = apidoc.DefaultReviewTimeout
)

type (
	apiDocCheckResult     = apidoc.CheckResult
	apiDocReviewFinding   = apidoc.ReviewFinding
	apiDocReviewOptions   = apidoc.ReviewOptions
	apiDocReviewResult    = apidoc.ReviewResult
	apiDocStaticOptions   = apidoc.StaticOptions
	apiDocStaticResult    = apidoc.StaticResult
	apiDocStaticViolation = apidoc.StaticViolation
)

func runAPIDoc(args []string) error {
	return apidoc.Run(args)
}

func runAPIDocCheck(args []string) error {
	return apidoc.RunCheck(args)
}

func runAPIDocCheckWithOptions(staticOptions apiDocStaticOptions, reviewOptions apiDocReviewOptions) (apiDocCheckResult, error) {
	return apidoc.RunCheckWithOptions(staticOptions, reviewOptions)
}

func runAPIDocReview(args []string) error {
	return apidoc.RunReview(args)
}

func runAPIDocReviewWithOptions(options apiDocReviewOptions) (apiDocReviewResult, error) {
	return apidoc.RunReviewWithOptions(options)
}

func runAPIDocStaticCheck(args []string) error {
	return apidoc.RunStaticCheck(args)
}

func runAPIDocStaticCheckWithOptions(options apiDocStaticOptions) (apiDocStaticResult, error) {
	return apidoc.RunStaticCheckWithOptions(options)
}

func runCodexAPIDocReview(options apiDocReviewOptions, files []string, diff, extraPrompt string) (apiDocReviewResult, error) {
	return apidoc.RunCodexReview(options, files, diff, extraPrompt)
}

func printAPIDocReview(result apiDocReviewResult) {
	apidoc.PrintReview(result)
}

func printAPIDocStaticCheck(result apiDocStaticResult) {
	apidoc.PrintStaticCheck(result)
}

func mustJSON(value any) []byte {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return b
}

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
