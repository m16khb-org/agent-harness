package lintdiagnose

import (
	"fmt"
	"os/exec"
	"strings"

	"agent-harness/internal/adapter/repopath"
)

type LintDiagnoseRequest struct {
	RepoRoot    string   `json:"repo_root"`
	CommandArgv []string `json:"command_argv"`
}

type LintDiagnoseResult struct {
	OK          bool     `json:"ok"`
	CommandArgv []string `json:"command_argv"`
	ExitCode    int      `json:"exit_code"`
	Failed      bool     `json:"failed"`
	Diagnosis   string   `json:"diagnosis,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
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

	lines := strings.Split(outputStr, "\n")
	if len(lines) > 150 {
		lines = lines[len(lines)-150:]
	}
	logTail := strings.Join(lines, "\n")

	result.Prompt = BuildPrompt(exitCode, logTail)
	result.Diagnosis = "command failed; prompt contains the host-agent judgement request"
	return result, nil
}
