package validationcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
		deps.run = runCommandStep
	}
	return deps
}

func validateCommandPolicy(binary, root string) StepResult {
	return validateCommandPolicyWithDeps(binary, root, commandPolicyValidationDeps{})
}

func validateCommandPolicyWithDeps(binary, root string, deps commandPolicyValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempWorkspace, err := deps.makeTempDir("workspace")
	if err != nil {
		return failedStep("command policy smoke", err)
	}
	defer deps.removeAll(tempWorkspace)
	outside, err := deps.makeTempDir("outside")
	if err != nil {
		return failedStep("command policy smoke", err)
	}
	defer deps.removeAll(outside)

	stdoutParts := []string{}
	commands := []string{}
	for _, check := range commandPolicyChecks(binary, tempWorkspace, outside) {
		step := deps.run(root, check.label, 30*time.Second, "", check.name, check.args...)
		stdoutParts = append(stdoutParts, step.Stdout)
		commands = append(commands, step.Command)
		if !step.OK {
			return combineFailedStep("command policy smoke", started, step, stdoutParts, commands)
		}
		if errs := check.validate(step.Stdout); len(errs) > 0 {
			return assertionStepWithOutput("command policy smoke", started, errs, stdoutParts, commands)
		}
	}
	marker := filepath.Join(tempWorkspace, "marker")
	if deps.exists(marker) {
		return assertionStepWithOutput("command policy smoke", started, []string{"fake-run created marker; command executed unexpectedly"}, stdoutParts, commands)
	}

	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
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
