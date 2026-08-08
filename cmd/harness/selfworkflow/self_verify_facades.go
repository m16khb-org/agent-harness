package selfworkflow

import (
	"io"

	"agent-harness/cmd/harness/selfworkflow/llmeval"
	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/cmd/harness/selfworkflow/progress"
	"agent-harness/cmd/harness/selfworkflow/rerun"
	"agent-harness/cmd/harness/selfworkflow/steps"
)

const SelfVerifyLLMEvalEvidenceBudgetBytes = llmeval.SelfVerifyLLMEvalEvidenceBudgetBytes

type SelfVerifyLLMEvalConfig = llmeval.SelfVerifyLLMEvalConfig
type SelfVerifyLLMEvalInput = llmeval.SelfVerifyLLMEvalInput
type SelfVerifyLLMEvalOptions = llmeval.SelfVerifyLLMEvalOptions
type SelfVerifyLLMEvalResult = model.SelfVerifyLLMEvalResult
type SelfVerifyPlannedStep = steps.SelfVerifyPlannedStep
type SelfVerifyProgressEvent = progress.SelfVerifyProgressEvent
type SelfVerifyProgressReporter = progress.SelfVerifyProgressReporter
type SelfVerifyStepDeps = steps.SelfVerifyStepDeps

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

func ApplySelfVerifyLLMEval(result SelfAugmentResult, opts SelfVerifyLLMEvalOptions) (SelfAugmentResult, error) {
	return llmeval.ApplySelfVerifyLLMEval(result, opts)
}

func ApplySelfVerifyLLMGate(result SelfAugmentResult, targetScore float64) (SelfAugmentResult, error) {
	return llmeval.ApplySelfVerifyLLMGate(result, targetScore)
}

func BuildSelfVerifyLLMEvalPrompt(result SelfAugmentResult) (string, int, error) {
	return llmeval.BuildSelfVerifyLLMEvalPrompt(result)
}

func SelfVerifyLLMResponseSchemaExample() string {
	return llmeval.SelfVerifyLLMResponseSchemaExample()
}

func SelfVerifyLLMResponseFieldTypes() []string {
	return llmeval.SelfVerifyLLMResponseFieldTypes()
}

func NewSelfVerifyProgressReporter(mode string, writer io.Writer) (*SelfVerifyProgressReporter, error) {
	return progress.NewSelfVerifyProgressReporter(mode, writer)
}

func PlannedSelfVerifySteps(root string, tempBin string, seed int64, goTestStep *StepResult, deps SelfVerifyStepDeps) []SelfVerifyPlannedStep {
	return steps.PlannedSelfVerifySteps(root, tempBin, seed, goTestStep, deps)
}

func CachedContractGoldenStep(goTestStep StepResult, deps SelfVerifyStepDeps) StepResult {
	return steps.CachedContractGoldenStep(goTestStep, deps)
}

func selfVerifyRerunCommands(failedStep string, iterations int, baseSeed int64, targetScore float64) []string {
	return rerun.SelfVerifyRerunCommands(failedStep, iterations, baseSeed, targetScore)
}

func selfVerifyStepRerunCommand(label string) (string, bool) {
	return rerun.SelfVerifyStepRerunCommand(label)
}

func formatScore(score float64) string {
	return rerun.FormatScore(score)
}

func boolPtr(value bool) *bool {
	return &value
}
