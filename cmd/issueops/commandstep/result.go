package commandstep

import (
	"fmt"
	"strings"
	"time"
)

func BudgetCommandOutput(s string, budget int) (string, bool, int) {
	if budget <= 0 {
		return s, false, len(s)
	}
	return TailWithBudget(s, budget)
}

func CombineFailedStep(label string, started time.Time, child StepResult, stdoutParts []string, commands []string, outputBudget int) StepResult {
	stdoutText, stdoutTruncated, stdoutBytes := TailWithBudget(strings.Join(stdoutParts, "\n"), outputBudget)
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

func AssertionStep(label string, started time.Time, errs []string) StepResult {
	step := StepResult{Label: label, OK: len(errs) == 0, DurationMS: time.Since(started).Milliseconds()}
	if len(errs) > 0 {
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func AssertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string, outputBudget int) StepResult {
	step := AssertionStep(label, started, errs)
	step.Command = strings.Join(commands, " && ")
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = TailWithBudget(strings.Join(stdoutParts, "\n"), outputBudget)
	return step
}

func FailedStep(label string, err error) StepResult {
	return StepResult{Label: label, OK: false, Error: err.Error()}
}

func PrintStep(step StepResult) {
	if step.OK {
		fmt.Printf("→ %s ok (%dms)\n", step.Label, step.DurationMS)
		return
	}
	fmt.Printf("→ %s failed (%dms): %s\n", step.Label, step.DurationMS, step.Error)
	if step.Stdout != "" {
		fmt.Printf("  stdout:\n%s\n", IndentLines(step.Stdout))
	}
	if step.Stderr != "" {
		fmt.Printf("  stderr:\n%s\n", IndentLines(step.Stderr))
	}
}

func Tail(s string, max int) string {
	out, _, _ := TailWithBudget(s, max)
	return out
}

func TailWithBudget(s string, max int) (string, bool, int) {
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

func IndentLines(s string) string {
	lines := splitLines(s)
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}
