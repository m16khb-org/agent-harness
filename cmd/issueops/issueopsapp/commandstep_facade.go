package issueopsapp

import (
	"time"

	"issueops/cmd/issueops/commandstep"
)

type StepResult = commandstep.StepResult

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return commandstep.Run(dir, label, timeout, stdin, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
	return commandstep.RunEnv(dir, label, timeout, stdin, env, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnvWithBudget(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) StepResult {
	return commandstep.RunEnvWithBudget(dir, label, timeout, stdin, env, outputBudget, name, args...)
}

func mergeEnvOverrides(base []string, overrides []string) []string {
	return commandstep.MergeEnvOverrides(base, overrides)
}

func envEntryKey(entry string) (string, bool) {
	return commandstep.EnvEntryKey(entry)
}

func budgetCommandOutput(s string, budget int) (string, bool, int) {
	return commandstep.BudgetCommandOutput(s, budget)
}

func combineFailedStep(label string, started time.Time, child StepResult, stdoutParts []string, commands []string) StepResult {
	return commandstep.CombineFailedStep(label, started, child, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func assertionStep(label string, started time.Time, errs []string) StepResult {
	return commandstep.AssertionStep(label, started, errs)
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) StepResult {
	return commandstep.AssertionStepWithOutput(label, started, errs, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func failedStep(label string, err error) StepResult {
	return commandstep.FailedStep(label, err)
}

func printStep(step StepResult) {
	commandstep.PrintStep(step)
}

func tail(s string, max int) string {
	return commandstep.Tail(s, max)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return commandstep.TailWithBudget(s, max)
}

func indentLines(s string) string {
	return commandstep.IndentLines(s)
}
