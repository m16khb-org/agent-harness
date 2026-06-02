package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultExternalLLMCommand = "agy"

type ExternalLLMPrintRequest struct {
	Command string
	WorkDir string
	Prompt  string
	Timeout time.Duration
}

type ExternalLLMPrintResult struct {
	Command string
	Argv    []string
	Output  []byte
}

func RunExternalLLMPrint(req ExternalLLMPrintRequest) (ExternalLLMPrintResult, error) {
	command := strings.TrimSpace(req.Command)
	if command == "" {
		command = defaultExternalLLMCommand
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return ExternalLLMPrintResult{Command: command}, fmt.Errorf("external llm prompt is required")
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	argv := []string{"--dangerously-skip-permissions", "-p", prompt}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, argv...)
	if strings.TrimSpace(req.WorkDir) != "" {
		cmd.Dir = req.WorkDir
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return ExternalLLMPrintResult{Command: command, Argv: argv, Output: out}, fmt.Errorf("external llm print timed out after %s", timeout)
	}
	if err != nil {
		return ExternalLLMPrintResult{Command: command, Argv: argv, Output: out}, err
	}
	return ExternalLLMPrintResult{Command: command, Argv: argv, Output: out}, nil
}

func ExternalLLMPrintCommandPreview(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		command = defaultExternalLLMCommand
	}
	return joinHandoffArgs([]string{command, "--dangerously-skip-permissions", "-p", "<prompt>"})
}
