package externalllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultProvider = "zai"
	defaultModel    = "glm-5-turbo"
)

// baseURL is the Z.AI Coding Plan endpoint. Overridden in tests.
var baseURL = "https://api.z.ai/api/coding/paas/v4/chat/completions"

// ExternalLLMPrintRequest configures an external LLM call.
//
// By default, the harness uses Z.AI Coding Plan (glm-5-turbo) with
// structured output (response_format: json_object) and thinking disabled.
type ExternalLLMPrintRequest struct {
	// Provider selects the backend. Empty and "zai" both use Z.AI.
	Provider string
	// Model for Z.AI provider (default "glm-5-turbo").
	Model string
	// APIKey overrides the $Z_AI_API_KEY environment variable.
	APIKey string
	// WorkDir is retained for callers that still carry workspace context.
	WorkDir string
	// Prompt is the text sent to the LLM.
	Prompt string
	// Timeout caps the total call duration.
	Timeout time.Duration
	// DisableStructuredJSON turns off response_format: json_object (rare).
	DisableStructuredJSON bool
}

// ExternalLLMPrintResult holds the LLM response plus per-call observation
// data (resolved model, wall-clock duration, and the provider usage block
// when the response carries one).
type ExternalLLMPrintResult struct {
	Output     []byte
	Model      string
	DurationMS int64
	Usage      *ExternalLLMUsage
}

// RunExternalLLMPrint sends a prompt to the external LLM and returns the raw
// response. By default this uses the Z.AI Coding Plan HTTP API with
// glm-5-turbo, structured JSON output, and thinking disabled.
func RunExternalLLMPrint(req ExternalLLMPrintRequest) (ExternalLLMPrintResult, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return ExternalLLMPrintResult{}, fmt.Errorf("external llm prompt is required")
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = defaultProvider
	}

	if provider != defaultProvider {
		return ExternalLLMPrintResult{}, fmt.Errorf("unsupported external llm provider %q; use zai:%s", provider, defaultModel)
	}

	return runZAI(req, timeout)
}

// runZAI calls the Z.AI Coding Plan HTTP API.
func runZAI(req ExternalLLMPrintRequest, timeout time.Duration) (ExternalLLMPrintResult, error) {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		apiKey = os.Getenv("Z_AI_API_KEY")
	}
	if apiKey == "" {
		return ExternalLLMPrintResult{}, fmt.Errorf("external llm: Z_AI_API_KEY is not set")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = defaultModel
	}
	started := time.Now()

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type thinking struct {
		Type string `json:"type"`
	}
	payload := map[string]any{
		"model":    model,
		"messages": []message{{Role: "user", Content: req.Prompt}},
		"thinking": thinking{Type: "disabled"},
	}
	if !req.DisableStructuredJSON {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ExternalLLMPrintResult{}, fmt.Errorf("external llm: marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return ExternalLLMPrintResult{}, fmt.Errorf("external llm: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ExternalLLMPrintResult{}, fmt.Errorf("external llm timed out after %s", timeout)
		}
		return ExternalLLMPrintResult{}, fmt.Errorf("external llm request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ExternalLLMPrintResult{}, fmt.Errorf("external llm: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return ExternalLLMPrintResult{Output: respBody}, fmt.Errorf("external llm returned HTTP %d: %s", resp.StatusCode, boundedOutputText(string(respBody)))
	}

	// Extract the content from the OpenAI-compatible response.
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *ExternalLLMUsage `json:"usage,omitempty"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return ExternalLLMPrintResult{Output: respBody}, fmt.Errorf("external llm: parse response: %w", err)
	}
	if chatResp.Error != nil && chatResp.Error.Message != "" {
		return ExternalLLMPrintResult{Output: respBody}, fmt.Errorf("external llm API error: %s (code %s)", chatResp.Error.Message, chatResp.Error.Code)
	}
	if len(chatResp.Choices) == 0 {
		return ExternalLLMPrintResult{Output: respBody}, fmt.Errorf("external llm returned no choices")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	result := ExternalLLMPrintResult{
		Output:     []byte(content),
		Model:      model,
		DurationMS: time.Since(started).Milliseconds(),
		Usage:      chatResp.Usage,
	}
	if usageRecorder != nil {
		usageRecorder(ExternalLLMUsageObservation{
			Provider:   defaultProvider,
			Model:      model,
			Usage:      result.Usage,
			DurationMS: result.DurationMS,
			OK:         true,
		})
	}
	return result, nil
}

// ExternalLLMPrintCommandPreview returns a human-readable description of the
// default external LLM configuration.
func ExternalLLMPrintCommandPreview() string {
	return fmt.Sprintf("zai:%s (thinking=disabled, structured_json=enabled)", defaultModel)
}

// DefaultModel returns the default model name used when none is specified.
func DefaultModel() string { return defaultModel }

// SetBaseURL overrides the Z.AI API endpoint. For tests only.
func SetBaseURL(u string) (previous string) {
	previous = baseURL
	baseURL = u
	return
}
