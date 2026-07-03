package core

import (
	"encoding/json"
	"fmt"
	"time"

	"agent-harness/internal/core/externalllm"
)

// Wiring the recorder here (core root) keeps the externalllm package free of
// state dependencies while observing every production caller: leaf packages
// call externalllm directly, but every shipped binary imports core root.
func init() {
	externalllm.SetUsageRecorder(recordExternalLLMUsage)
}

type externalLLMUsageStateSnapshot struct {
	SchemaVersion    int    `json:"schema_version"`
	Kind             string `json:"kind"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`
	DurationMS       int64  `json:"duration_ms"`
	OK               bool   `json:"ok"`
	GeneratedAt      string `json:"generated_at"`
}

const (
	externalLLMUsageStateKeyPrefix = "external-llm-usage-"
	observationStateMaxAge         = 30 * 24 * time.Hour
	observationStateMaxRecords     = 10000
)

// recordExternalLLMUsage appends one usage observation per LLM call as an
// external-llm-usage-* state record. Observation only: every failure path
// returns silently so recording can never block or fail the call it observes.
func recordExternalLLMUsage(obs externalllm.ExternalLLMUsageObservation) {
	now := time.Now().UTC()
	snapshot := externalLLMUsageStateSnapshot{
		SchemaVersion: 1,
		Kind:          "external_llm_usage",
		Provider:      obs.Provider,
		Model:         obs.Model,
		DurationMS:    obs.DurationMS,
		OK:            obs.OK,
		GeneratedAt:   now.Format(time.RFC3339Nano),
	}
	if obs.Usage != nil {
		snapshot.PromptTokens = obs.Usage.PromptTokens
		snapshot.CompletionTokens = obs.Usage.CompletionTokens
		snapshot.TotalTokens = obs.Usage.TotalTokens
	}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return
	}
	key := fmt.Sprintf("%s%s-%09d", externalLLMUsageStateKeyPrefix, now.Format("20060102T150405Z"), now.Nanosecond())
	// Best-effort by design: a broken state dir must not surface into the call.
	if _, err := StateWrite(key, string(b)); err == nil {
		_, _ = StatePrunePrefix(externalLLMUsageStateKeyPrefix, observationStateMaxAge, observationStateMaxRecords, true)
	}
}
