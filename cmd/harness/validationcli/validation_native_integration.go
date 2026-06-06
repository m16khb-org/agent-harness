package validationcli

import (
	"fmt"
	"strings"
	"time"
)

func validateNativeIntegration(root string) StepResult {
	return validateNativeIntegrationWithDeps(root, nativeIntegrationValidationDeps{})
}

func validateNativeIntegrationWithDeps(root string, deps nativeIntegrationValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	home, err := deps.userHomeDir()
	if err != nil {
		return failedStep("native integration", fmt.Errorf("user home: %w", err))
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
	errs = append(errs, nativeIntegrationCodexConfigErrors(home, deps)...)
	warningErrs, warningOutput := nativeIntegrationDuplicateWarningOutput(deps.duplicateWarningFixture())
	errs = append(errs, warningErrs...)
	stdoutParts = append(stdoutParts, warningOutput)
	if len(errs) > 0 {
		return assertionStepWithOutput("native integration", started, errs, stdoutParts, nil)
	}
	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "native integration",
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
