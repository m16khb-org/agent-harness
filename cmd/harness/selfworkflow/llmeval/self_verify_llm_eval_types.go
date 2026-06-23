package llmeval

import (
	"time"

	"agent-harness/cmd/harness/selfworkflow/model"
)

type SelfVerifyLLMEvalOptions struct {
	Enabled     bool
	Mode        string
	Model       string
	TargetScore float64
	Timeout     time.Duration
}

type SelfVerifyLLMEvalResult = model.SelfVerifyLLMEvalResult
