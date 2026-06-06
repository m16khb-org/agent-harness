package main

import (
	"encoding/json"
	"strings"
	"time"

	"agent-harness/internal/core"
)

type validationCommandRunner func(dir, label string, timeout time.Duration, stdin, name string, args ...string) StepResult

func validateInspect(binary, root string) StepResult {
	return validateInspectWithDeps(binary, root, runCommandStep)
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

func validateDocsIndex(binary, root string) StepResult {
	return validateDocsIndexWithDeps(binary, root, runCommandStep)
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
