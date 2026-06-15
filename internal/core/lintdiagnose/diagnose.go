package lintdiagnose

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"agent-harness/internal/core/externalllm"
	"agent-harness/internal/core/repopath"
)

type LintDiagnoseRequest struct {
	RepoRoot        string        `json:"repo_root"`
	CommandArgv     []string      `json:"command_argv"`
	Model           string        `json:"model,omitempty"`
	AgyCommand      string        `json:"agy_command,omitempty"`
	AgyModel        string        `json:"agy_model,omitempty"`
	AgySettingsPath string        `json:"-"`
	Timeout         time.Duration `json:"-"`
}

type LintDiagnoseResult struct {
	OK          bool     `json:"ok"`
	CommandArgv []string `json:"command_argv"`
	ExitCode    int      `json:"exit_code"`
	Failed      bool     `json:"failed"`
	Diagnosis   string   `json:"diagnosis,omitempty"`
	Model       string   `json:"model,omitempty"`
}

type lintDiagnoseResponse struct {
	Diagnosis string `json:"diagnosis"`
}

func DiagnoseCommand(req LintDiagnoseRequest) (LintDiagnoseResult, error) {
	root, err := repopath.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return LintDiagnoseResult{}, err
	}

	if len(req.CommandArgv) == 0 {
		return LintDiagnoseResult{}, fmt.Errorf("missing command to execute")
	}

	// 1. Run the targeted command
	cmdName := req.CommandArgv[0]
	cmdArgs := req.CommandArgv[1:]

	execCmd := exec.Command(cmdName, cmdArgs...)
	execCmd.Dir = root

	outputBytes, runErr := execCmd.CombinedOutput()
	outputStr := string(outputBytes)

	exitCode := 0
	failed := false
	if runErr != nil {
		failed = true
		if exitError, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := LintDiagnoseResult{
		OK:          true,
		CommandArgv: req.CommandArgv,
		ExitCode:    exitCode,
		Failed:      failed,
	}

	if !failed {
		return result, nil
	}

	// 2. Resolve model
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(req.AgyModel) // backward compat
	}
	if model == "" {
		model = externalllm.DefaultModel()
	}
	result.Model = model

	// 3. Determine provider
	provider := ""
	agyCommand := strings.TrimSpace(req.AgyCommand)
	if agyCommand != "" {
		provider = agyCommand // legacy agy path
	}

	// 4. Compose prompt
	lines := strings.Split(outputStr, "\n")
	if len(lines) > 150 {
		lines = lines[len(lines)-150:]
	}
	logTail := strings.Join(lines, "\n")

	llmPrompt := BuildPrompt(exitCode, logTail)

	// 5. Run external LLM
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	llm, err := externalllm.RunExternalLLMPrint(externalllm.ExternalLLMPrintRequest{
		Provider: provider,
		Model:    model,
		WorkDir:  root,
		Prompt:   llmPrompt,
		Timeout:  timeout,
	})
	if err != nil {
		result.Diagnosis = fmt.Sprintf("[Error running external LLM: %v]\nOriginal Output:\n%s", err, outputStr)
		return result, nil
	}
	var response lintDiagnoseResponse
	if err := externalllm.DecodeExternalLLMStructuredJSONObject("lint diagnose", llm.Output, &response); err != nil {
		result.Diagnosis = fmt.Sprintf("[Error parsing LLM JSON: %v]\nOriginal Output:\n%s", err, outputStr)
		return result, nil
	}

	diagnosis := strings.TrimSpace(response.Diagnosis)
	if diagnosis == "" {
		result.Diagnosis = fmt.Sprintf("[Error parsing LLM JSON: missing diagnosis]\nOriginal Output:\n%s", outputStr)
		return result, nil
	}
	result.Diagnosis = diagnosis
	return result, nil
}
