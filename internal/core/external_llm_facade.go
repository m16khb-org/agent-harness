package core

import (
	"agent-harness/internal/core/externalllm"
	"agent-harness/internal/core/prompt"
)

type ExternalLLMPrintRequest = externalllm.ExternalLLMPrintRequest
type ExternalLLMPrintResult = externalllm.ExternalLLMPrintResult

func RunExternalLLMPrint(req ExternalLLMPrintRequest) (ExternalLLMPrintResult, error) {
	return externalllm.RunExternalLLMPrint(req)
}

func ExternalLLMPrintCommandPreview(command string) string {
	return externalllm.ExternalLLMPrintCommandPreview(command)
}

func BuildExternalLLMJSONSchemaSection(example string, fieldTypes []string) prompt.PromptDataSection {
	return externalllm.BuildExternalLLMJSONSchemaSection(example, fieldTypes)
}

func DecodeExternalLLMStructuredJSONObject(label string, out []byte, target any) error {
	return externalllm.DecodeExternalLLMStructuredJSONObject(label, out, target)
}
