package main

import "agent-harness/cmd/harness/hookcli"

func configureHookCLI() {
	hookcli.ResolveTarget = resolveTarget
}

func runHook(args []string) error {
	configureHookCLI()
	return hookcli.RunHook(args)
}

func runHookUserPrompt(args []string) error {
	configureHookCLI()
	return hookcli.RunHookUserPrompt(args)
}

func runHookPreToolUse(args []string) error {
	configureHookCLI()
	return hookcli.RunHookPreToolUse(args)
}

func runHookPostToolUse(args []string) error {
	configureHookCLI()
	return hookcli.RunHookPostToolUse(args)
}

func runHookPreCompact(args []string) error {
	configureHookCLI()
	return hookcli.RunHookPreCompact(args)
}

func runHookPostCompact(args []string) error {
	configureHookCLI()
	return hookcli.RunHookPostCompact(args)
}

func runHookSessionStart(args []string) error {
	configureHookCLI()
	return hookcli.RunHookSessionStart(args)
}

func runHookStop(args []string) error {
	configureHookCLI()
	return hookcli.RunHookStop(args)
}

func runHookFailures(args []string) error {
	configureHookCLI()
	return hookcli.RunHookFailures(args)
}

func hookArgValue(args []string, flagName string) string {
	return hookcli.HookArgValue(args, flagName)
}

func repoFromHookInput(input []byte) string {
	return hookcli.RepoFromHookInput(input)
}

func sourceFromHookInput(input []byte) string {
	return hookcli.SourceFromHookInput(input)
}

func pathsFromHookInput(input []byte) []string {
	return hookcli.PathsFromHookInput(input)
}

func promptFromHookInput(input []byte) string {
	return hookcli.PromptFromHookInput(input)
}

func isStopHookContinuationPrompt(prompt string) bool {
	return hookcli.IsStopHookContinuationPrompt(prompt)
}

func lastAssistantMessageFromHookInput(input []byte) string {
	return hookcli.LastAssistantMessageFromHookInput(input)
}

func transcriptPathFromHookInput(input []byte) string {
	return hookcli.TranscriptPathFromHookInput(input)
}

func readLastAssistantMessageFromTranscript(path string) string {
	return hookcli.ReadLastAssistantMessageFromTranscript(path)
}

func toolNameFromHookInput(input []byte) string {
	return hookcli.ToolNameFromHookInput(input)
}

func commandFromHookInput(input []byte) string {
	return hookcli.CommandFromHookInput(input)
}

func projectPathFromHookInput(input []byte) string {
	return hookcli.ProjectPathFromHookInput(input)
}

func envBool(name string) bool {
	return hookcli.EnvBool(name)
}

func envFloat(name string) float64 {
	return hookcli.EnvFloat(name)
}
