package llmeval

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/core/externalllm"
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
		t.Fatalf("HARNESS_SELF_VERIFY_LLM_EVAL=gate should enable gate mode, got %+v", config)
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

func TestSelfVerifyLLMEvalAdvisorySuccess(t *testing.T) {
	withFakeSelfVerifyZAI(t, selfVerifyFakeZAIResponse{Content: `{"ok":true,"score":99,"summary":"looks safe","risks":["watch flakes"],"recommended_next_actions":["ship"]}`})
	result := model.SelfAugmentResult{OK: true, TerminationEligible: true, Summary: model.SelfAugmentSummary{MinimumGoalScore: 100}}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory llm eval should not fail self-verify: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || !updated.LLMEval.OK || updated.LLMEval.Score != 99 {
		t.Fatalf("unexpected advisory llm eval result: %+v", updated)
	}
	if len(updated.LLMEval.Risks) != 1 || len(updated.LLMEval.RecommendedNextActions) != 1 || updated.LLMEval.EvidencePacketBytes == 0 {
		t.Fatalf("llm eval should keep structured review fields and packet size: %+v", updated.LLMEval)
	}
}

func TestSelfVerifyLLMEvalExtractsNoisyLLMJSON(t *testing.T) {
	withFakeSelfVerifyZAI(t, selfVerifyFakeZAIResponse{Content: "ULTRAWORK MODE ENABLED!\n" + `{"ok":true,"score":99,"summary":"looks safe","blockers":[],"risks":[],"recommended_next_actions":[]}`})
	result := model.SelfAugmentResult{OK: true, TerminationEligible: true, Summary: model.SelfAugmentSummary{MinimumGoalScore: 100}}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory noisy llm eval should not fail self-verify: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || !updated.LLMEval.OK || updated.LLMEval.Score != 99 || updated.LLMEval.Error != "" {
		t.Fatalf("noisy LLM output should extract strict JSON object: %+v", updated)
	}
}

func TestSelfVerifyLLMEvalMalformedOutputIsStructured(t *testing.T) {
	withFakeSelfVerifyZAI(t, selfVerifyFakeZAIResponse{Content: `not-json`})
	result := model.SelfAugmentResult{OK: true, TerminationEligible: true}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory malformed llm eval should be recorded, not returned as gate error: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || updated.LLMEval.OK || !strings.Contains(updated.LLMEval.Error, "parse LLM JSON") {
		t.Fatalf("malformed LLM output should produce structured llm_eval error: %+v", updated)
	}
	if len(updated.LLMEval.Error) > 512 {
		t.Fatalf("llm eval error should be bounded, got %d bytes", len(updated.LLMEval.Error))
	}
}

func TestSelfVerifyLLMEvalUnknownFieldIsStructured(t *testing.T) {
	withFakeSelfVerifyZAI(t, selfVerifyFakeZAIResponse{Content: `{"ok":true,"score":99,"unexpected":true}`})
	result := model.SelfAugmentResult{OK: true, TerminationEligible: true}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory unknown field should be recorded, not returned as gate error: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || updated.LLMEval.OK || !strings.Contains(updated.LLMEval.Error, "unknown field") {
		t.Fatalf("unknown LLM field should produce structured llm_eval error: %+v", updated)
	}
}

func TestSelfVerifyLLMEvalRequestFailureIsStructured(t *testing.T) {
	withFakeSelfVerifyZAI(t, selfVerifyFakeZAIResponse{Status: http.StatusInternalServerError, Body: `{"error":{"message":"model unavailable"}}`})
	result := model.SelfAugmentResult{OK: true, TerminationEligible: true}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory command failure should be recorded, not returned as gate error: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || updated.LLMEval.OK || !strings.Contains(updated.LLMEval.Error, "Z.AI LLM call failed") {
		t.Fatalf("request failure should produce structured llm_eval error: %+v", updated)
	}
}

func TestSelfVerifyLLMEvalResultClassifiesForegroundReadOnlyGate(t *testing.T) {
	withFakeSelfVerifyZAI(t, selfVerifyFakeZAIResponse{Content: `{"ok":true,"score":100,"summary":"pass","blockers":[],"risks":[],"recommended_next_actions":[]}`})
	result := model.SelfAugmentResult{OK: true, TerminationEligible: true, Summary: model.SelfAugmentSummary{MinimumGoalScore: 100, TerminationEligible: true}}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "gate", TargetScore: 95})
	if err != nil {
		t.Fatal(err)
	}
	if updated.LLMEval == nil {
		t.Fatal("expected llm_eval result")
	}
	if updated.LLMEval.ExecutionClass != "foreground_blocking" || !updated.LLMEval.ReadOnly {
		t.Fatalf("expected foreground read-only LLM gate classification, got %+v", updated.LLMEval)
	}
}

func TestSelfVerifyLLMEvalGateFailsOnBlocker(t *testing.T) {
	withFakeSelfVerifyZAI(t, selfVerifyFakeZAIResponse{Content: `{"ok":false,"score":40,"summary":"blocked","blockers":["missing QA"]}`})
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

type selfVerifyFakeZAIResponse struct {
	Status  int
	Body    string
	Content string
}

func withFakeSelfVerifyZAI(t *testing.T, responses ...selfVerifyFakeZAIResponse) {
	t.Helper()
	if len(responses) == 0 {
		t.Fatal("missing fake Z.AI responses")
	}
	t.Setenv("Z_AI_API_KEY", "test-key")
	// Isolate the state dir: the usage recorder writes an observation record
	// per LLM call and must not touch the developer's real state.
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := responses[index]
		if index < len(responses)-1 {
			index++
		}
		if response.Status != 0 {
			w.WriteHeader(response.Status)
		}
		if response.Body != "" {
			_, _ = w.Write([]byte(response.Body))
			return
		}
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, response.Content)
	}))
	t.Cleanup(server.Close)
	previous := externalllm.SetBaseURL(server.URL)
	t.Cleanup(func() { externalllm.SetBaseURL(previous) })
}
