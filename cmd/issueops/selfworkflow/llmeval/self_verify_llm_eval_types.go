package llmeval

import "issueops/cmd/issueops/selfworkflow/model"

type SelfVerifyLLMEvalOptions struct {
	Enabled     bool
	Mode        string
	TargetScore float64
}

type SelfVerifyLLMEvalResult = model.SelfVerifyLLMEvalResult
