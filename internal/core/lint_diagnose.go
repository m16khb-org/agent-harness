package core

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type LintDiagnoseRequest struct {
	RepoRoot        string        `json:"repo_root"`
	CommandArgv     []string      `json:"command_argv"`
	AgyCommand      string        `json:"agy_command"`
	AgyModel        string        `json:"agy_model"`
	AgySettingsPath string        `json:"-"`
	Timeout         time.Duration `json:"-"`
}

type LintDiagnoseResult struct {
	OK          bool     `json:"ok"`
	CommandArgv []string `json:"command_argv"`
	ExitCode    int      `json:"exit_code"`
	Failed      bool     `json:"failed"`
	Diagnosis   string   `json:"diagnosis,omitempty"`
	AgyCommand  string   `json:"agy_command"`
	AgyModel    string   `json:"agy_model"`
}

type lintDiagnoseAgyResponse struct {
	Diagnosis string `json:"diagnosis"`
}

func DiagnoseCommand(req LintDiagnoseRequest) (LintDiagnoseResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
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

	// Capture stdout and stderr together
	outputBytes, runErr := execCmd.CombinedOutput()
	outputStr := string(outputBytes)

	exitCode := 0
	failed := false
	if runErr != nil {
		failed = true
		if exitError, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Other startup errors
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

	// 2. Resolve agy settings & model
	agyCommand := strings.TrimSpace(req.AgyCommand)
	if agyCommand == "" {
		agyCommand = "agy"
	}
	settingsPath := resolveAgySettingsPath(req.AgySettingsPath)
	configuredModel, err := readAgyConfiguredModel(settingsPath)
	if err != nil {
		configuredModel = "default"
	}
	agyModel := strings.TrimSpace(req.AgyModel)
	if agyModel == "" {
		agyModel = configuredModel
	}

	result.AgyCommand = agyCommand
	result.AgyModel = agyModel

	// 3. Compose prompt for diagnosis
	// Restrict to last 150 lines to keep context clean
	lines := strings.Split(outputStr, "\n")
	if len(lines) > 150 {
		lines = lines[len(lines)-150:]
	}
	logTail := strings.Join(lines, "\n")

	prompt := buildLintDiagnosePrompt(exitCode, logTail)

	// 4. Run agy
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	llm, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Command: agyCommand, WorkDir: root, Prompt: prompt, Timeout: timeout})
	if err != nil {
		// Do not return hard error if agy itself fails, just record the error in diagnosis
		result.Diagnosis = fmt.Sprintf("[Error running agy: %v]\nOriginal Output:\n%s", err, outputStr)
		return result, nil
	}
	var response lintDiagnoseAgyResponse
	if err := DecodeExternalLLMStructuredJSONObject("agy lint diagnose", llm.Output, &response); err != nil {
		result.Diagnosis = fmt.Sprintf("[Error parsing agy JSON: %v]\nOriginal Output:\n%s", err, outputStr)
		return result, nil
	}

	diagnosis := strings.TrimSpace(response.Diagnosis)
	if diagnosis == "" {
		result.Diagnosis = fmt.Sprintf("[Error parsing agy JSON: missing diagnosis]\nOriginal Output:\n%s", outputStr)
		return result, nil
	}
	result.Diagnosis = diagnosis
	return result, nil
}
