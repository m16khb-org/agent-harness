package nativeintegration

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/cmd/harness/commandstep"
)

const aggregateOutputBudgetBytes = 8 * 1024

type StepResult = commandstep.StepResult

func Validate(root string) StepResult {
	return validateNativeIntegration(root)
}

func validateNativeIntegration(root string) StepResult {
	return validateNativeIntegrationWithDeps(root, nativeIntegrationValidationDeps{})
}

func validateNativeIntegrationWithDeps(root string, deps nativeIntegrationValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	home, err := deps.userHomeDir()
	if err != nil {
		return commandstep.FailedStep("native integration", fmt.Errorf("user home: %w", err))
	}
	errs := []string{}
	stdoutParts := []string{}
	nativeSkills, err := deps.listSkills(root)
	if err != nil {
		errs = append(errs, "list native skills: "+err.Error())
	}
	codexSkills, _ := deps.skillNamesForHost(root, nativeSkills, "codex")
	claudeSkills, _ := deps.skillNamesForHost(root, nativeSkills, "claude")
	paths := nativeIntegrationRequiredPaths(root, home, codexSkills, claudeSkills)
	errs = append(errs, nativeIntegrationPathErrors(paths, deps)...)
	errs = append(errs, nativeIntegrationCodexConfigErrors(root, home, deps)...)
	warningErrs, warningOutput := nativeIntegrationDuplicateWarningOutput(deps.duplicateWarningFixture())
	errs = append(errs, warningErrs...)
	stdoutParts = append(stdoutParts, warningOutput)
	if len(errs) > 0 {
		return commandstep.AssertionStepWithOutput("native integration", started, errs, stdoutParts, nil, aggregateOutputBudgetBytes)
	}
	stdoutText, stdoutTruncated, stdoutBytes := commandstep.TailWithBudget(strings.Join(stdoutParts, "\n"), aggregateOutputBudgetBytes)
	return StepResult{
		Label:           "native integration",
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
