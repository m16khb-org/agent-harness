package main

import (
	"fmt"
	"strings"
	"time"
)

func budgetCommandOutput(s string, budget int) (string, bool, int) {
	if budget <= 0 {
		return s, false, len(s)
	}
	return tailWithBudget(s, budget)
}

func combineFailedStep(label string, started time.Time, child StepResult, stdoutParts []string, commands []string) StepResult {
	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	step := StepResult{
		Label:           label,
		Command:         strings.Join(commands, " && "),
		OK:              false,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		Stderr:          child.Stderr,
		StdoutBytes:     stdoutBytes,
		StderrBytes:     child.StderrBytes,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: child.StderrTruncated,
		Error:           child.Label + ": " + child.Error,
	}
	if step.Error == child.Label+": " {
		step.Error = child.Label + " failed"
	}
	return step
}

func assertionStep(label string, started time.Time, errs []string) StepResult {
	step := StepResult{Label: label, OK: len(errs) == 0, DurationMS: time.Since(started).Milliseconds()}
	if len(errs) > 0 {
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) StepResult {
	step := assertionStep(label, started, errs)
	step.Command = strings.Join(commands, " && ")
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return step
}

func failedStep(label string, err error) StepResult {
	return StepResult{Label: label, OK: false, Error: err.Error()}
}

func printStep(step StepResult) {
	if step.OK {
		fmt.Printf("→ %s ok (%dms)\n", step.Label, step.DurationMS)
		return
	}
	fmt.Printf("→ %s failed (%dms): %s\n", step.Label, step.DurationMS, step.Error)
	if step.Stdout != "" {
		fmt.Printf("  stdout:\n%s\n", indentLines(step.Stdout))
	}
	if step.Stderr != "" {
		fmt.Printf("  stderr:\n%s\n", indentLines(step.Stderr))
	}
}

func tail(s string, max int) string {
	out, _, _ := tailWithBudget(s, max)
	return out
}

func tailWithBudget(s string, max int) (string, bool, int) {
	originalBytes := len(s)
	if max <= 0 {
		return "", originalBytes > 0, originalBytes
	}
	if originalBytes <= max {
		return s, false, originalBytes
	}
	tailBudget := max
	for {
		marker := fmt.Sprintf("[truncated: original_bytes=%d omitted_bytes=%d]\n", originalBytes, originalBytes-tailBudget)
		tailBudgetNext := max - len(marker)
		if tailBudgetNext < 0 {
			return marker[:max], true, originalBytes
		}
		if tailBudgetNext == tailBudget {
			return marker + s[originalBytes-tailBudget:], true, originalBytes
		}
		tailBudget = tailBudgetNext
	}
}

func indentLines(s string) string {
	lines := splitLines(s)
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
