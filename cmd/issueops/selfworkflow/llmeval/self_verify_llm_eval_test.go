package llmeval

import (
	"encoding/json"
	"strings"
	"testing"

	"issueops/cmd/issueops/selfworkflow/model"
)

func TestSelfVerifyLLMEvalDefaultOmittedFromJSON(t *testing.T) {
	result := model.SelfAugmentResult{OK: true, LoopKind: "self_verification"}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "llm_eval") {
		t.Fatalf("default self-verify JSON must omit llm_eval: %s", b)
	}
}

func TestResolveSelfVerifyLLMEvalConfigDefaultsOff(t *testing.T) {
	config, err := ResolveSelfVerifyLLMEvalConfig(false, false, "advisory", false, envLookupForSelfVerifyTest(nil))
	if err != nil {
		t.Fatal(err)
	}
	if config.Enabled || config.Mode != "advisory" {
		t.Fatalf("default LLM eval config should stay off/advisory, got %+v", config)
	}
}

func TestResolveSelfVerifyLLMEvalConfigUsesEnvGate(t *testing.T) {
	config, err := ResolveSelfVerifyLLMEvalConfig(false, false, "advisory", false, envLookupForSelfVerifyTest(map[string]string{
		EnvName: "gate",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !config.Enabled || config.Mode != "gate" {
		t.Fatalf("ISSUEOPS_SELF_VERIFY_LLM_EVAL=gate should enable gate mode, got %+v", config)
	}
}

func TestResolveSelfVerifyLLMEvalConfigCLIOverridesEnv(t *testing.T) {
	config, err := ResolveSelfVerifyLLMEvalConfig(true, false, "advisory", false, envLookupForSelfVerifyTest(map[string]string{
		EnvName: "strict",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if config.Enabled || config.Mode != "advisory" {
		t.Fatalf("explicit --llm-eval=false should ignore env and keep default advisory mode, got %+v", config)
	}
}

func TestResolveSelfVerifyLLMEvalConfigRejectsInvalidEnv(t *testing.T) {
	_, err := ResolveSelfVerifyLLMEvalConfig(false, false, "advisory", false, envLookupForSelfVerifyTest(map[string]string{
		EnvName: "strict",
	}))
	if err == nil || !strings.Contains(err.Error(), EnvName) {
		t.Fatalf("expected env validation error naming %s, got %v", EnvName, err)
	}
}

func TestParseSelfVerifyLLMEvalEnvParsesDisabledAliasesAndRejectsUnknown(t *testing.T) {
	for _, value := range []string{"", "0", "false", "no", "off", "disabled"} {
		enabled, mode, err := ParseSelfVerifyLLMEvalEnv(value)
		if err != nil || enabled || mode != "advisory" {
			t.Fatalf("ParseSelfVerifyLLMEvalEnv(%q) enabled=%v mode=%q err=%v", value, enabled, mode, err)
		}
	}
	enabled, mode, err := ParseSelfVerifyLLMEvalEnv(" gate ")
	if err != nil || !enabled || mode != "gate" {
		t.Fatalf("gate env parse enabled=%v mode=%q err=%v", enabled, mode, err)
	}
	if _, _, err := ParseSelfVerifyLLMEvalEnv("maybe"); err == nil || !strings.Contains(err.Error(), EnvName) {
		t.Fatalf("expected named env parse error, got %v", err)
	}
}

func TestDecodeSelfVerifyLLMEvalStrictRejectsExtraJSONValue(t *testing.T) {
	var eval model.SelfVerifyLLMEvalResult
	err := DecodeSelfVerifyLLMEvalStrict([]byte(`{"ok":true,"mode":"advisory","execution_class":"foreground_blocking","read_only":true,"score":99,"blockers":[],"risks":[],"recommended_next_actions":[],"evidence_packet_bytes":10} {"ok":false}`), &eval)
	if err == nil || !strings.Contains(err.Error(), "unexpected extra JSON value") {
		t.Fatalf("expected extra JSON value error, got %v", err)
	}
}

func TestDecodeSelfVerifyLLMEvalReadsHostJudgementJSON(t *testing.T) {
	var eval model.SelfVerifyLLMEvalResult
	err := DecodeSelfVerifyLLMEval([]byte(`{"ok":true,"score":99,"summary":"looks safe","risks":["watch flakes"],"recommended_next_actions":["ship"]}`), &eval)
	if err != nil {
		t.Fatalf("decode host judgement result: %v", err)
	}
	if !eval.OK || eval.Score != 99 || eval.Summary != "looks safe" {
		t.Fatalf("unexpected host judgement result: %+v", eval)
	}
	if len(eval.Risks) != 1 || len(eval.RecommendedNextActions) != 1 {
		t.Fatalf("host judgement should keep structured review fields: %+v", eval)
	}
}

func TestDecodeSelfVerifyLLMEvalExtractsNoisyHostJudgementJSON(t *testing.T) {
	var eval model.SelfVerifyLLMEvalResult
	err := DecodeSelfVerifyLLMEval([]byte("review note\n"+`{"ok":true,"score":99,"summary":"looks safe","blockers":[],"risks":[],"recommended_next_actions":[]}`), &eval)
	if err != nil {
		t.Fatalf("decode noisy host judgement result: %v", err)
	}
	if !eval.OK || eval.Score != 99 || eval.Error != "" {
		t.Fatalf("noisy host judgement output should extract strict JSON object: %+v", eval)
	}
}

func TestDecodeSelfVerifyLLMEvalRejectsMalformedOutput(t *testing.T) {
	var eval model.SelfVerifyLLMEvalResult
	err := DecodeSelfVerifyLLMEval([]byte(`not-json`), &eval)
	if err == nil {
		t.Fatal("expected malformed host judgement output error")
	}
	bounded := BoundedLLMEvalError("parse host judgement JSON", err, strings.Repeat("x", 2048))
	if len(bounded) > 512 {
		t.Fatalf("host judgement error should be bounded, got %d bytes", len(bounded))
	}
}

func TestSelfVerifyLLMEvalRendersPromptOnlyResult(t *testing.T) {
	result := model.SelfAugmentResult{OK: true, TerminationEligible: true}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory prompt-only eval should be recorded, not returned as gate error: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || updated.LLMEval.OK || updated.LLMEval.Prompt == "" || !strings.Contains(updated.LLMEval.Error, "external LLM evaluation was removed") {
		t.Fatalf("prompt-only eval should produce structured llm_eval result: %+v", updated)
	}
}

func TestSelfVerifyLLMEvalResultClassifiesForegroundReadOnlyGate(t *testing.T) {
	result := model.SelfAugmentResult{OK: true, TerminationEligible: true, Summary: model.SelfAugmentSummary{MinimumGoalScore: 100, TerminationEligible: true}}
	updated, _ := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", TargetScore: 95})
	if updated.LLMEval == nil {
		t.Fatal("expected llm_eval result")
	}
	if updated.LLMEval.ExecutionClass != "foreground_blocking" || !updated.LLMEval.ReadOnly {
		t.Fatalf("expected foreground read-only LLM gate classification, got %+v", updated.LLMEval)
	}
}

func TestSelfVerifyLLMEvalGateFailsOnBlocker(t *testing.T) {
	result := model.SelfAugmentResult{OK: true, TerminationEligible: true, Summary: model.SelfAugmentSummary{MinimumGoalScore: 100, TerminationEligible: true}}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "gate", TargetScore: 95})
	if err == nil || !strings.Contains(err.Error(), "LLM evaluation gate failed") {
		t.Fatalf("expected gate failure, got err=%v result=%+v", err, updated)
	}
	if updated.OK || updated.TerminationEligible || updated.Summary.TerminationEligible || updated.LLMEval == nil || updated.LLMEval.OK {
		t.Fatalf("gate failure must mark self-verify not OK: %+v", updated)
	}
}

func envLookupForSelfVerifyTest(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
