package apidoc

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type apiDocReviewFinding struct {
	File     string `json:"file"`
	Line     *int   `json:"line"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type apiDocReviewResult struct {
	OK         bool                  `json:"ok"`
	Verdict    string                `json:"verdict"`
	Summary    string                `json:"summary"`
	Findings   []apiDocReviewFinding `json:"findings"`
	Files      []string              `json:"files"`
	Skipped    bool                  `json:"skipped,omitempty"`
	Reason     string                `json:"reason,omitempty"`
	Prompt     string                `json:"prompt,omitempty"`
	Schema     map[string]any        `json:"schema,omitempty"`
	ResultFile string                `json:"result_file,omitempty"`
}

type apiDocReviewOptions struct {
	Repo       string
	Files      []string
	All        bool
	DiffFile   string
	PromptFile string
	ResultFile string
	JSON       bool
}

func runAPIDoc(args []string) error {
	if len(args) == 0 {
		apiDocUsage()
		return fmt.Errorf("missing api-doc subcommand")
	}
	switch args[0] {
	case "check":
		return runAPIDocCheck(args[1:])
	case "review":
		return runAPIDocReview(args[1:])
	case "static-check":
		return runAPIDocStaticCheck(args[1:])
	default:
		apiDocUsage()
		return fmt.Errorf("unknown api-doc subcommand %q", args[0])
	}
}

func apiDocUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness api-doc review [--repo PATH] [--all] [--diff-file FILE] [--prompt-file FILE] [--result FILE] [--json] [--] [FILES...]
  agent-harness api-doc static-check [--repo PATH] [--all] [--json] [--] [FILES...]
  agent-harness api-doc check [--repo PATH] [--all] [--diff-file FILE] [--prompt-file FILE] [--result FILE] [--json] [--] [FILES...]
`)
}

func runAPIDocReview(args []string) error {
	fs := flag.NewFlagSet("api-doc review", flag.ContinueOnError)
	repo := fs.String("repo", "", "target git repository; defaults to current working directory")
	all := fs.Bool("all", false, "review all tracked API documentation candidate files instead of staged changes")
	diffFile := fs.String("diff-file", "", "read diff from file instead of git diff --cached")
	promptFile := fs.String("prompt-file", "", "append project-specific review instructions from file")
	resultFile := fs.String("result", "", "read host-agent JSON review result from file")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := ResolveTarget(*repo)
	options := apiDocReviewOptions{Repo: root, Files: fs.Args(), All: *all, DiffFile: *diffFile, PromptFile: *promptFile, ResultFile: *resultFile, JSON: *jsonOut}
	result, err := runAPIDocReviewWithOptions(options)
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printAPIDocReview(result)
	return err
}

func runAPIDocReviewWithOptions(options apiDocReviewOptions) (apiDocReviewResult, error) {
	files := normalizeAPIDocFiles(options.Repo, options.Files)
	if len(files) == 0 && options.All && options.DiffFile == "" {
		files = trackedAPIDocFiles(options.Repo)
	}
	if len(files) == 0 && !options.All && options.DiffFile == "" {
		files = stagedAPIDocFiles(options.Repo)
	}
	if len(files) == 0 && options.DiffFile == "" {
		summary := "No staged API documentation candidate files."
		reason := "no_api_doc_candidate_files"
		if options.All {
			summary = "No tracked API documentation candidate files."
			reason = "no_tracked_api_doc_candidate_files"
		}
		return apiDocReviewResult{OK: true, Verdict: "pass", Summary: summary, Findings: []apiDocReviewFinding{}, Files: []string{}, Skipped: true, Reason: reason}, nil
	}
	diff, err := apiDocInput(options.Repo, files, options.DiffFile, options.All)
	if err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files}, err
	}
	if strings.TrimSpace(diff) == "" {
		summary := "No staged API documentation diff."
		if options.All {
			summary = "No API documentation content."
		}
		return apiDocReviewResult{OK: true, Verdict: "pass", Summary: summary, Findings: []apiDocReviewFinding{}, Files: files, Skipped: true, Reason: "empty_diff"}, nil
	}
	extraPrompt, err := apiDocReviewExtraPrompt(options)
	if err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files}, err
	}
	evidence := ""
	if options.ResultFile == "" {
		evidence = apiDocReviewEvidence(options.Repo, files)
	}
	review, err := runHostAgentAPIDocReview(options, files, diff, extraPrompt, evidence)
	if err != nil {
		return review, err
	}
	return review, nil
}
