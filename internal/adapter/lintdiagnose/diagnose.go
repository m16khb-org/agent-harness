package lintdiagnose

import (
	"fmt"
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
	"os/exec"
	"strings"
)

func DiagnoseCommand(req lintdiagnosecontract.LintDiagnoseRequest) (lintdiagnosecontract.LintDiagnoseResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return lintdiagnosecontract.LintDiagnoseResult{}, err
	}

	if len(req.CommandArgv) == 0 {
		return lintdiagnosecontract.LintDiagnoseResult{}, fmt.Errorf("missing command to execute")
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

	result := lintdiagnosecontract.LintDiagnoseResult{
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
