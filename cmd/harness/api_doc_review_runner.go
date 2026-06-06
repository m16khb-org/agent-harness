package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runCodexAPIDocReview(options apiDocReviewOptions, files []string, diff, extraPrompt string) (apiDocReviewResult, error) {
	tmpDir, err := os.MkdirTemp("", "agent-harness-api-doc-review-")
	if err != nil {
		return apiDocReviewResult{}, err
	}
	defer os.RemoveAll(tmpDir)
	schemaPath := filepath.Join(tmpDir, "schema.json")
	outputPath := filepath.Join(tmpDir, "review.json")
	if err := os.WriteFile(schemaPath, mustJSON(apiDocReviewSchema()), 0o600); err != nil {
		return apiDocReviewResult{}, err
	}
	cmd := exec.Command("codex", "--ask-for-approval", "never", "exec", "--model", options.Model, "--config", fmt.Sprintf("model_reasoning_effort=\"%s\"", options.Effort), "--cd", options.Repo, "--sandbox", "read-only", "--output-schema", schemaPath, "--output-last-message", outputPath, "-")
	cmd.Stdin = strings.NewReader(buildAPIDocReviewPrompt(files, diff, extraPrompt))
	cmd.Dir = options.Repo
	cmd.Env = os.Environ()
	if options.Timeout <= 0 {
		options.Timeout = defaultAPIDocReviewTimeout
	}
	if err := runWithTimeout(cmd, options.Timeout); err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, Model: options.Model, Effort: options.Effort}, err
	}
	b, err := os.ReadFile(outputPath)
	if err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, Model: options.Model, Effort: options.Effort}, err
	}
	var result apiDocReviewResult
	if err := json.Unmarshal(b, &result); err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, Model: options.Model, Effort: options.Effort}, err
	}
	result.Files = files
	result.Model = options.Model
	result.Effort = options.Effort
	result.OK = result.Verdict == "pass"
	if result.Verdict == "fail" {
		return result, errAPIDocReviewGateFailed
	}
	return result, nil
}

func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("codex failed: %w: %s", err, stderr.String())
		}
		return nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return fmt.Errorf("codex timed out after %s", timeout)
	}
}
