package smoke

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-harness/cmd/harness/commandstep"
	docs "agent-harness/internal/adapter/docs"
	inspect "agent-harness/internal/contract/inspect"
)

// The smoke steps parse the captured stdout as JSON, so the budget must
// comfortably exceed the live docs index (38KB and growing). Parsing a
// tail-truncated capture surfaces a misleading "invalid character" decode
// error and deterministically fails the 95-gate.
const commandOutputBudgetBytes = 4 * 1024 * 1024

// rejectTruncatedCapture fails a step explicitly when its stdout was
// budget-truncated, before any JSON parse attempt.
func rejectTruncatedCapture(step StepResult) (StepResult, bool) {
	if !step.StdoutTruncated {
		return step, false
	}
	step.OK = false
	step.Error = fmt.Sprintf("%s: stdout truncated (original %d bytes exceeds the smoke output budget); cannot parse JSON — raise commandOutputBudgetBytes", step.Label, step.StdoutBytes)
	return step, true
}

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
	if rejected, truncated := rejectTruncatedCapture(step); truncated {
		return rejected
	}
	var info inspect.InspectInfo
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
	if rejected, truncated := rejectTruncatedCapture(step); truncated {
		return rejected
	}
	var index docs.DocsIndexResult
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
