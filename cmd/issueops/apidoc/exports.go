package apidoc

import (
	"errors"

	"issueops/cmd/issueops/apidoc/reviewfiles"
)

type (
	CheckResult   = apiDocCheckResult
	ReviewFinding = apiDocReviewFinding
	ReviewOptions = apiDocReviewOptions
	ReviewResult  = apiDocReviewResult
	StaticOptions = apiDocStaticOptions
	StaticResult  = apiDocStaticResult
)

func Run(args []string) error {
	return runAPIDoc(args)
}

func RunCheck(args []string) error {
	return runAPIDocCheck(args)
}

func RunCheckWithOptions(staticOptions StaticOptions, reviewOptions ReviewOptions) (CheckResult, error) {
	return runAPIDocCheckWithOptions(staticOptions, reviewOptions)
}

func RunReview(args []string) error {
	return runAPIDocReview(args)
}

func RunReviewWithOptions(options ReviewOptions) (ReviewResult, error) {
	return runAPIDocReviewWithOptions(options)
}

func RunStaticCheck(args []string) error {
	return runAPIDocStaticCheck(args)
}

func RunStaticCheckWithOptions(options StaticOptions) (StaticResult, error) {
	return runAPIDocStaticCheckWithOptions(options)
}

func Evidence(repo string, files []string) string {
	return reviewfiles.Evidence(repo, files)
}

func PrintReview(result ReviewResult) {
	printAPIDocReview(result)
}

func PrintStaticCheck(result StaticResult) {
	printAPIDocStaticCheck(result)
}

func IsReviewGateError(err error) bool {
	return errors.Is(err, ErrReviewGateFailed) || errors.Is(err, ErrReviewResultRequired)
}

func IsStaticGateError(err error) bool {
	return errors.Is(err, ErrStaticGateFailed)
}
