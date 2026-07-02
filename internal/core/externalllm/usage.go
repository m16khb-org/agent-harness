package externalllm

// ExternalLLMUsage mirrors the OpenAI-compatible usage block of a response.
type ExternalLLMUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ExternalLLMUsageObservation is the per-call observation handed to the usage
// recorder. Recording is observation only: it must never influence or block
// the LLM call itself.
type ExternalLLMUsageObservation struct {
	Provider   string
	Model      string
	Usage      *ExternalLLMUsage
	DurationMS int64
	OK         bool
}

// usageRecorder is a package-level hook so every caller of RunExternalLLMPrint
// is observed regardless of whether it goes through the core facade. It
// defaults to nil (no-op) so unit tests of leaf packages record nothing.
var usageRecorder func(ExternalLLMUsageObservation)

// SetUsageRecorder installs the usage observation hook and returns the
// previous one (tests restore it).
func SetUsageRecorder(f func(ExternalLLMUsageObservation)) (previous func(ExternalLLMUsageObservation)) {
	previous = usageRecorder
	usageRecorder = f
	return previous
}
