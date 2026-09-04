package llmeval

import (
	"fmt"
	"os"
	"strings"
)

const EnvName = "ISSUEOPS_SELF_VERIFY_LLM_EVAL"

type SelfVerifyLLMEvalConfig struct {
	Enabled bool
	Mode    string
}

func ValidateSelfVerifyLLMEvalMode(mode string) error {
	switch NormalizeSelfVerifyLLMEvalMode(mode) {
	case "advisory", "gate":
		return nil
	default:
		return fmt.Errorf("llm-eval-mode must be advisory or gate")
	}
}

func NormalizeSelfVerifyLLMEvalMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return "advisory"
	}
	return mode
}

func ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet bool, llmEvalFlagValue bool, llmEvalMode string, llmEvalModeFlagSet bool, lookupEnv func(string) (string, bool)) (SelfVerifyLLMEvalConfig, error) {
	config := SelfVerifyLLMEvalConfig{Mode: "advisory"}
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	ignoreEnv := llmEvalFlagSet && !llmEvalFlagValue
	if value, ok := lookupEnv(EnvName); ok && !ignoreEnv {
		enabled, mode, err := ParseSelfVerifyLLMEvalEnv(value)
		if err != nil {
			return config, err
		}
		config.Enabled = enabled
		config.Mode = mode
	}
	if llmEvalModeFlagSet {
		mode := NormalizeSelfVerifyLLMEvalMode(llmEvalMode)
		if err := ValidateSelfVerifyLLMEvalMode(mode); err != nil {
			return config, err
		}
		config.Mode = mode
	}
	if llmEvalFlagSet {
		config.Enabled = llmEvalFlagValue
	}
	return config, nil
}

func ParseSelfVerifyLLMEvalEnv(value string) (bool, string, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "0", "false", "no", "off", "disabled":
		return false, "advisory", nil
	case "1", "true", "yes", "on", "enabled", "advisory":
		return true, "advisory", nil
	case "gate":
		return true, "gate", nil
	default:
		return false, "advisory", fmt.Errorf("%s must be off, advisory, or gate", EnvName)
	}
}
