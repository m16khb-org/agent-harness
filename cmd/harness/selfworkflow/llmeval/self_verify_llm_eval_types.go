package llmeval

import "agent-harness/cmd/harness/selfworkflow/model"

type SelfVerifyLLMEvalOptions struct {
	Enabled     bool
	Mode        string
	TargetScore float64
}

type SelfVerifyLLMEvalResult = model.SelfVerifyLLMEvalResult
