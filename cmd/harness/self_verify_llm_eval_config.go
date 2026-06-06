package main

import (
	"fmt"
	"os"
	"strings"
)

const selfVerifyLLMEvalEnv = "HARNESS_SELF_VERIFY_LLM_EVAL"

type SelfVerifyLLMEvalConfig struct {
	Enabled bool
	Mode    string
}

func validateSelfVerifyLLMEvalMode(mode string) error {
	switch normalizeSelfVerifyLLMEvalMode(mode) {
	case "advisory", "gate":
		return nil
	default:
		return fmt.Errorf("llm-eval-mode must be advisory or gate")
	}
}

func normalizeSelfVerifyLLMEvalMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return "advisory"
	}
	return mode
}

func resolveSelfVerifyLLMEvalConfig(llmEvalFlagSet bool, llmEvalFlagValue bool, llmEvalMode string, llmEvalModeFlagSet bool, lookupEnv func(string) (string, bool)) (SelfVerifyLLMEvalConfig, error) {
	config := SelfVerifyLLMEvalConfig{Mode: "advisory"}
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	ignoreEnv := llmEvalFlagSet && !llmEvalFlagValue
	if value, ok := lookupEnv(selfVerifyLLMEvalEnv); ok && !ignoreEnv {
		enabled, mode, err := parseSelfVerifyLLMEvalEnv(value)
		if err != nil {
			return config, err
		}
		config.Enabled = enabled
		config.Mode = mode
	}
	if llmEvalModeFlagSet {
		mode := normalizeSelfVerifyLLMEvalMode(llmEvalMode)
		if err := validateSelfVerifyLLMEvalMode(mode); err != nil {
			return config, err
		}
		config.Mode = mode
	}
	if llmEvalFlagSet {
		config.Enabled = llmEvalFlagValue
	}
	return config, nil
}

func parseSelfVerifyLLMEvalEnv(value string) (bool, string, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "0", "false", "no", "off", "disabled":
		return false, "advisory", nil
	case "1", "true", "yes", "on", "enabled", "advisory":
		return true, "advisory", nil
	case "gate":
		return true, "gate", nil
	default:
		return false, "advisory", fmt.Errorf("%s must be off, advisory, or gate", selfVerifyLLMEvalEnv)
	}
}
