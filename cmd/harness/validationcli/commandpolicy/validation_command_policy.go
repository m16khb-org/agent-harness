package commandpolicy

import (
	"agent-harness/cmd/harness/commandstep"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024

type StepResult = commandstep.StepResult

type commandPolicyCommandRunner func(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult

type commandPolicyValidationDeps struct {
	makeTempDir func(kind string) (string, error)
	removeAll   func(path string) error
	exists      func(path string) bool
	run         commandPolicyCommandRunner
}

func (deps commandPolicyValidationDeps) withDefaults() commandPolicyValidationDeps {
	if deps.makeTempDir == nil {
		deps.makeTempDir = func(kind string) (string, error) {
			switch kind {
			case "workspace":
				return os.MkdirTemp("", "agent-harness-policy-*")
			case "outside":
				return os.MkdirTemp("", "agent-harness-policy-outside-*")
			default:
				return "", fmt.Errorf("unknown command policy temp kind: %s", kind)
			}
		}
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.run == nil {
		deps.run = func(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
			return commandstep.Run(dir, label, timeout, stdin, commandOutputBudgetBytes, name, args...)
		}
	}
	return deps
}

func Validate(binary, root string) StepResult {
	return validateCommandPolicyWithDeps(binary, root, commandPolicyValidationDeps{})
}

func validateCommandPolicy(binary, root string) StepResult {
	return Validate(binary, root)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func validateCommandPolicyWithDeps(binary, root string, deps commandPolicyValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempWorkspace, err := deps.makeTempDir("workspace")
	if err != nil {
		return commandstep.FailedStep("command policy smoke", err)
	}
	defer deps.removeAll(tempWorkspace)
	outside, err := deps.makeTempDir("outside")
	if err != nil {
		return commandstep.FailedStep("command policy smoke", err)
	}
	defer deps.removeAll(outside)

	stdoutParts := []string{}
	commands := []string{}
	for _, check := range commandPolicyChecks(binary, tempWorkspace, outside) {
		step := deps.run(root, check.label, 30*time.Second, "", check.name, check.args...)
		stdoutParts = append(stdoutParts, step.Stdout)
		commands = append(commands, step.Command)
		if !step.OK {
			return commandstep.CombineFailedStep("command policy smoke", started, step, stdoutParts, commands, aggregateOutputBudgetBytes)
		}
		if errs := check.validate(step.Stdout); len(errs) > 0 {
			return commandstep.AssertionStepWithOutput("command policy smoke", started, errs, stdoutParts, commands, aggregateOutputBudgetBytes)
		}
	}
	marker := filepath.Join(tempWorkspace, "marker")
	if deps.exists(marker) {
		return commandstep.AssertionStepWithOutput("command policy smoke", started, []string{"fake-run created marker; command executed unexpectedly"}, stdoutParts, commands, aggregateOutputBudgetBytes)
	}

	stdoutText, stdoutTruncated, stdoutBytes := commandstep.TailWithBudget(strings.Join(stdoutParts, "\n"), aggregateOutputBudgetBytes)
	return StepResult{
		Label:           "command policy smoke",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
