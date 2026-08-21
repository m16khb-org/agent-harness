package apidoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func runHostAgentAPIDocReview(options apiDocReviewOptions, files []string, diff, extraPrompt, evidence string) (apiDocReviewResult, error) {
	prompt := buildAPIDocReviewPrompt(files, diff, extraPrompt, evidence)
	schema := apiDocReviewSchema()
	if options.ResultFile == "" {
		return apiDocReviewResult{
			OK:       false,
			Verdict:  "pending",
			Summary:  "Host-agent API documentation review result is required; run the prompt and pass --result <file>.",
			Findings: []apiDocReviewFinding{},
			Files:    files,
			Reason:   "host_agent_result_required",
			Prompt:   prompt,
			Schema:   schema,
		}, ErrReviewResultRequired
	}

	resultPath := resolveAPIDocReviewResultPath(options.Repo, options.ResultFile)
	b, err := os.ReadFile(resultPath)
	if err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, ResultFile: resultPath}, err
	}
	var result apiDocReviewResult
	if err := json.Unmarshal(b, &result); err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, ResultFile: resultPath}, err
	}
	if result.Verdict != "pass" && result.Verdict != "fail" {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: fmt.Sprintf("review result verdict must be pass or fail, got %q", result.Verdict), Files: files, ResultFile: resultPath}, fmt.Errorf("invalid API doc review result verdict %q", result.Verdict)
	}
	result.Files = files
	result.ResultFile = resultPath
	result.OK = result.Verdict == "pass"
	if result.Verdict == "fail" {
		return result, ErrReviewGateFailed
	}
	return result, nil
}

func resolveAPIDocReviewResultPath(repo, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(repo, path)
}
