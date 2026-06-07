package selfworkflow

import "agent-harness/cmd/harness/selfworkflow/llmeval"

const selfVerifyLLMEvalEnv = llmeval.EnvName

type SelfVerifyLLMEvalConfig = llmeval.SelfVerifyLLMEvalConfig
type SelfVerifyLLMEvalOptions = llmeval.SelfVerifyLLMEvalOptions
type SelfVerifyLLMEvalResult = llmeval.SelfVerifyLLMEvalResult

func ValidateSelfVerifyLLMEvalMode(mode string) error {
	return llmeval.ValidateSelfVerifyLLMEvalMode(mode)
}

func NormalizeSelfVerifyLLMEvalMode(mode string) string {
	return llmeval.NormalizeSelfVerifyLLMEvalMode(mode)
}

func ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet bool, llmEvalFlagValue bool, llmEvalMode string, llmEvalModeFlagSet bool, lookupEnv func(string) (string, bool)) (SelfVerifyLLMEvalConfig, error) {
	return llmeval.ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet, llmEvalFlagValue, llmEvalMode, llmEvalModeFlagSet, lookupEnv)
}

func ParseSelfVerifyLLMEvalEnv(value string) (bool, string, error) {
	return llmeval.ParseSelfVerifyLLMEvalEnv(value)
}

func DecodeSelfVerifyLLMEval(out []byte, eval *SelfVerifyLLMEvalResult) error {
	return llmeval.DecodeSelfVerifyLLMEval(out, eval)
}

func DecodeSelfVerifyLLMEvalStrict(out []byte, eval *SelfVerifyLLMEvalResult) error {
	return llmeval.DecodeSelfVerifyLLMEvalStrict(out, eval)
}

func ExtractSelfVerifyLLMEvalJSON(out []byte) ([]byte, bool) {
	return llmeval.ExtractSelfVerifyLLMEvalJSON(out)
}

func BoundedLLMEvalError(prefix string, err error, output string) string {
	return llmeval.BoundedLLMEvalError(prefix, err, output)
}
