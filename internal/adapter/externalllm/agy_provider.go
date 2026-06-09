// Package externalllm provides the agy-based ExternalLLM adapter.
package externalllm

import (
	"encoding/json"
	"strings"
	"time"

	corexternalllm "agent-harness/internal/core/externalllm"
	"agent-harness/internal/port"
)

// AgyProvider implements port.ExternalLLM using the agy CLI.
type AgyProvider struct {
	// Command is the agy executable path. Defaults to "agy".
	Command string
	// Timeout is the per-call deadline. Defaults to 2 minutes.
	Timeout time.Duration
	// WorkDir is the working directory for agy invocations.
	WorkDir string
}

// NewAgyProvider returns an AgyProvider with sensible defaults.
func NewAgyProvider() *AgyProvider {
	return &AgyProvider{
		Command: "agy",
		Timeout: 2 * time.Minute,
	}
}

func (p *AgyProvider) command() string {
	if strings.TrimSpace(p.Command) == "" {
		return "agy"
	}
	return p.Command
}

func (p *AgyProvider) timeout() time.Duration {
	if p.Timeout <= 0 {
		return 2 * time.Minute
	}
	return p.Timeout
}

// Query sends a prompt to agy -p and returns the raw text response.
func (p *AgyProvider) Query(prompt string) (string, error) {
	result, err := corexternalllm.RunExternalLLMPrint(corexternalllm.ExternalLLMPrintRequest{
		Command: p.command(),
		WorkDir: p.WorkDir,
		Prompt:  prompt,
		Timeout: p.timeout(),
	})
	if err != nil {
		return "", err
	}
	return string(result.Output), nil
}

// QueryStructured sends a prompt with a schema to agy -p and returns the
// parsed JSON response.
func (p *AgyProvider) QueryStructured(prompt string, schema map[string]any) (map[string]any, error) {
	fullPrompt := prompt
	if schema != nil {
		example := formatSchemaExample(schema)
		if example != "" {
			fullPrompt = prompt + "\n\n" + example
		}
	}
	result, err := corexternalllm.RunExternalLLMPrint(corexternalllm.ExternalLLMPrintRequest{
		Command: p.command(),
		WorkDir: p.WorkDir,
		Prompt:  fullPrompt,
		Timeout: p.timeout(),
	})
	if err != nil {
		return nil, err
	}
	var target map[string]any
	if err := corexternalllm.DecodeExternalLLMStructuredJSONObject("agy", result.Output, &target); err != nil {
		return nil, err
	}
	return target, nil
}

// formatSchemaExample creates a minimal JSON example from a schema map.
func formatSchemaExample(schema map[string]any) string {
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return "{}"
	}
	example := make(map[string]any, len(props))
	for key, val := range props {
		propMap, ok := val.(map[string]any)
		if !ok {
			example[key] = "..."
			continue
		}
		switch propMap["type"] {
		case "string":
			example[key] = "..."
		case "number", "integer":
			example[key] = 0
		case "boolean":
			example[key] = false
		case "array":
			example[key] = []string{"..."}
		case "object":
			example[key] = map[string]any{"...": "..."}
		default:
			example[key] = "..."
		}
	}
	b, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// Ensure AgyProvider implements port.ExternalLLM.
var _ port.ExternalLLM = (*AgyProvider)(nil)
