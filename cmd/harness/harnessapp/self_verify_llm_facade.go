package harnessapp

import "agent-harness/cmd/harness/selfworkflow"

type SelfVerifyLLMEvalConfig = selfworkflow.SelfVerifyLLMEvalConfig
type SelfVerifyLLMEvalInput = selfworkflow.SelfVerifyLLMEvalInput
type SelfVerifyLLMEvalOptions = selfworkflow.SelfVerifyLLMEvalOptions

func applySelfVerifyLLMEval(result SelfAugmentResult, opts SelfVerifyLLMEvalOptions) (SelfAugmentResult, error) {
	return selfworkflow.ApplySelfVerifyLLMEval(result, opts)
}

func validateSelfVerifyLLMEvalMode(mode string) error {
	return selfworkflow.ValidateSelfVerifyLLMEvalMode(mode)
}

func normalizeSelfVerifyLLMEvalMode(mode string) string {
	return selfworkflow.NormalizeSelfVerifyLLMEvalMode(mode)
}

func resolveSelfVerifyLLMEvalConfig(llmEvalFlagSet bool, llmEvalFlagValue bool, llmEvalMode string, llmEvalModeFlagSet bool, lookupEnv func(string) (string, bool)) (SelfVerifyLLMEvalConfig, error) {
	return selfworkflow.ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet, llmEvalFlagValue, llmEvalMode, llmEvalModeFlagSet, lookupEnv)
}

func parseSelfVerifyLLMEvalEnv(value string) (bool, string, error) {
	return selfworkflow.ParseSelfVerifyLLMEvalEnv(value)
}

func decodeSelfVerifyLLMEval(out []byte, eval *SelfVerifyLLMEvalResult) error {
	return selfworkflow.DecodeSelfVerifyLLMEval(out, eval)
}

func decodeSelfVerifyLLMEvalStrict(out []byte, eval *SelfVerifyLLMEvalResult) error {
	return selfworkflow.DecodeSelfVerifyLLMEvalStrict(out, eval)
}

func extractSelfVerifyLLMEvalJSON(out []byte) ([]byte, bool) {
	return selfworkflow.ExtractSelfVerifyLLMEvalJSON(out)
}

func boundedLLMEvalError(prefix string, err error, output string) string {
	return selfworkflow.BoundedLLMEvalError(prefix, err, output)
}

func applySelfVerifyLLMGate(result SelfAugmentResult, targetScore float64) (SelfAugmentResult, error) {
	return selfworkflow.ApplySelfVerifyLLMGate(result, targetScore)
}

func buildSelfVerifyLLMEvalPrompt(result SelfAugmentResult) (string, int, error) {
	return selfworkflow.BuildSelfVerifyLLMEvalPrompt(result)
}

func selfVerifyLLMResponseSchemaExample() string {
	return selfworkflow.SelfVerifyLLMResponseSchemaExample()
}

func selfVerifyLLMResponseFieldTypes() []string {
	return selfworkflow.SelfVerifyLLMResponseFieldTypes()
}
