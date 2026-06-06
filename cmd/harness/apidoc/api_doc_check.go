package apidoc

import (
	"flag"
	"fmt"
	"time"
)

type apiDocCheckResult struct {
	OK     bool               `json:"ok"`
	Static apiDocStaticResult `json:"static"`
	Review apiDocReviewResult `json:"review"`
	Reason string             `json:"reason,omitempty"`
}

func runAPIDocCheck(args []string) error {
	fs := flag.NewFlagSet("api-doc check", flag.ContinueOnError)
	repo := fs.String("repo", "", "target git repository; defaults to current working directory")
	all := fs.Bool("all", false, "check all tracked API documentation candidate files instead of staged changes")
	model := fs.String("model", defaultAPIDocReviewModel, "Codex model")
	effort := fs.String("reasoning", defaultAPIDocReviewReasoning, "Codex model_reasoning_effort")
	timeoutText := fs.String("timeout", defaultAPIDocReviewTimeout.String(), "Codex timeout")
	diffFile := fs.String("diff-file", "", "read diff from file instead of git diff --cached")
	promptFile := fs.String("prompt-file", "", "append project-specific review instructions from file")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	timeout, err := time.ParseDuration(*timeoutText)
	if err != nil {
		return err
	}
	root := ResolveTarget(*repo)
	result, err := runAPIDocCheckWithOptions(apiDocStaticOptions{Repo: root, Files: fs.Args(), All: *all, JSON: true}, apiDocReviewOptions{Repo: root, Model: *model, Effort: *effort, Timeout: timeout, Files: fs.Args(), All: *all, DiffFile: *diffFile, PromptFile: *promptFile, JSON: true})
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printAPIDocStaticCheck(result.Static)
	if result.Static.OK {
		printAPIDocReview(result.Review)
	}
	return err
}

func runAPIDocCheckWithOptions(staticOptions apiDocStaticOptions, reviewOptions apiDocReviewOptions) (apiDocCheckResult, error) {
	staticResult, staticErr := runAPIDocStaticCheckWithOptions(staticOptions)
	if staticErr != nil || !staticResult.OK {
		return apiDocCheckResult{OK: false, Static: staticResult, Review: apiDocReviewResult{OK: true, Verdict: "pass", Summary: "Agent review skipped because static API documentation check failed.", Findings: []apiDocReviewFinding{}, Files: staticResult.Files, Skipped: true, Reason: "static_check_failed"}, Reason: "static_check_failed"}, fmt.Errorf("api documentation static check failed")
	}
	reviewResult, reviewErr := runAPIDocReviewWithOptions(reviewOptions)
	return apiDocCheckResult{OK: reviewErr == nil && reviewResult.OK, Static: staticResult, Review: reviewResult}, reviewErr
}
