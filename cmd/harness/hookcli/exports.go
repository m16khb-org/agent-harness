package hookcli

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
	return repoFromHookInput(input)
}

func SourceFromHookInput(input []byte) string {
	return sourceFromHookInput(input)
}

func PathsFromHookInput(input []byte) []string {
	return pathsFromHookInput(input)
}

func PromptFromHookInput(input []byte) string {
	return promptFromHookInput(input)
}

func IsStopHookContinuationPrompt(prompt string) bool {
	return isStopHookContinuationPrompt(prompt)
}

func LastAssistantMessageFromHookInput(input []byte) string {
	return lastAssistantMessageFromHookInput(input)
}

func TranscriptPathFromHookInput(input []byte) string {
	return transcriptPathFromHookInput(input)
}

func ReadLastAssistantMessageFromTranscript(path string) string {
	return readLastAssistantMessageFromTranscript(path)
}

func ToolNameFromHookInput(input []byte) string {
	return toolNameFromHookInput(input)
}

func CommandFromHookInput(input []byte) string {
	return commandFromHookInput(input)
}

func ProjectPathFromHookInput(input []byte) string {
	return projectPathFromHookInput(input)
}

func EnvBool(name string) bool {
	return envBool(name)
}

func EnvFloat(name string) float64 {
	return envFloat(name)
}
