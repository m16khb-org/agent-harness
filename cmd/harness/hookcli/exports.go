package hookcli

import (
	"agent-harness/cmd/harness/hookcli/hookenv"
	"agent-harness/cmd/harness/hookcli/hookinput"
)

func RunHook(args []string) error {
	return runHook(args)
}

func RunHookUserPrompt(args []string) error {
	return runHookUserPrompt(args)
}

func RunHookPreToolUse(args []string) error {
	return runHookPreToolUse(args)
}

func RunHookPostToolUse(args []string) error {
	return runHookPostToolUse(args)
}

func RunHookPreCompact(args []string) error {
	return runHookPreCompact(args)
}

func RunHookPostCompact(args []string) error {
	return runHookPostCompact(args)
}

func RunHookSessionStart(args []string) error {
	return runHookSessionStart(args)
}

func RunHookStop(args []string) error {
	return runHookStop(args)
}

func RunHookFailures(args []string) error {
	return runHookFailures(args)
}

func HookArgValue(args []string, flagName string) string {
	return hookArgValue(args, flagName)
}

func RepoFromHookInput(input []byte) string {
	return hookinput.RepoFromHookInput(input)
}

func SourceFromHookInput(input []byte) string {
	return hookinput.SourceFromHookInput(input)
}

func PathsFromHookInput(input []byte) []string {
	return hookinput.PathsFromHookInput(input)
}

func PromptFromHookInput(input []byte) string {
	return promptFromHookInput(input)
}

func IsStopHookContinuationPrompt(prompt string) bool {
	return isStopHookContinuationPrompt(prompt)
}

func LastAssistantMessageFromHookInput(input []byte) string {
	return hookinput.LastAssistantMessageFromHookInput(input)
}

func TranscriptPathFromHookInput(input []byte) string {
	return hookinput.TranscriptPathFromHookInput(input)
}

func ReadLastAssistantMessageFromTranscript(path string) string {
	return hookinput.ReadLastAssistantMessageFromTranscript(path)
}

func ToolNameFromHookInput(input []byte) string {
	return hookinput.ToolNameFromHookInput(input)
}

func CommandFromHookInput(input []byte) string {
	return hookinput.CommandFromHookInput(input)
}

func ProjectPathFromHookInput(input []byte) string {
	return hookinput.ProjectPathFromHookInput(input)
}

func EnvBool(name string) bool {
	return hookenv.Bool(name)
}

func EnvFloat(name string) float64 {
	return hookenv.Float(name)
}
