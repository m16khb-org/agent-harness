package smoke

import (
	"encoding/json"
	"strings"
	"time"

	"agent-harness/cmd/harness/commandstep"
	"agent-harness/internal/core"
)

const commandOutputBudgetBytes = 32 * 1024

type StepResult = commandstep.StepResult
type validationCommandRunner func(dir, label string, timeout time.Duration, stdin, name string, args ...string) StepResult

func ValidateInspect(binary, root string) StepResult {
	return validateInspectWithDeps(binary, root, runCommandStep)
}

func validateInspect(binary, root string) StepResult {
	return ValidateInspect(binary, root)
}

func validateInspectWithDeps(binary, root string, run validationCommandRunner) StepResult {
	step := run(root, "inspect smoke", 30*time.Second, "", binary, "inspect", "--json")
	if !step.OK {
		return step
	}
	var info core.InspectInfo
	if err := json.Unmarshal([]byte(step.Stdout), &info); err != nil {
		step.OK = false
		step.Error = err.Error()
		return step
	}
	errs := inspectSmokeValidationErrors(info, step.Stdout, root)
	if len(errs) > 0 {
		step.OK = false
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func ValidateDocsIndex(binary, root string) StepResult {
	return validateDocsIndexWithDeps(binary, root, runCommandStep)
}

func validateDocsIndex(binary, root string) StepResult {
	return ValidateDocsIndex(binary, root)
}

func validateDocsIndexWithDeps(binary, root string, run validationCommandRunner) StepResult {
	step := run(root, "docs index smoke", 30*time.Second, "", binary, "docs", "--json")
	if !step.OK {
		return step
	}
	var index core.DocsIndexResult
	if err := json.Unmarshal([]byte(step.Stdout), &index); err != nil {
		step.OK = false
		step.Error = err.Error()
		return step
	}
	errs := docsIndexSmokeValidationErrors(index, root)
	if len(errs) > 0 {
		step.OK = false
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return commandstep.Run(dir, label, timeout, stdin, commandOutputBudgetBytes, name, args...)
}
