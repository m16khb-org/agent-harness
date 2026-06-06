package selfworkflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelfVerifyLLMEvalDefaultOmittedFromJSON(t *testing.T) {
	result := SelfAugmentResult{OK: true, LoopKind: "self_verification"}
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
		selfVerifyLLMEvalEnv: "gate",
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
		selfVerifyLLMEvalEnv: "strict",
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
		selfVerifyLLMEvalEnv: "strict",
	}))
	if err == nil || !strings.Contains(err.Error(), selfVerifyLLMEvalEnv) {
		t.Fatalf("expected env validation error naming %s, got %v", selfVerifyLLMEvalEnv, err)
	}
}

func TestSelfVerifyLLMEvalAdvisorySuccess(t *testing.T) {
	fake := writeFakeAgyForSelfVerifyTest(t, `{"ok":true,"score":99,"summary":"looks safe","risks":["watch flakes"],"recommended_next_actions":["ship"]}`)
	result := SelfAugmentResult{OK: true, TerminationEligible: true, Summary: SelfAugmentSummary{MinimumGoalScore: 100}}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", AgyCommand: fake, TargetScore: 95})
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

func TestSelfVerifyLLMEvalExtractsNoisyAgyJSON(t *testing.T) {
	fake := writeFakeAgyForSelfVerifyTest(t, "ULTRAWORK MODE ENABLED!\n"+`{"ok":true,"score":99,"summary":"looks safe","blockers":[],"risks":[],"recommended_next_actions":[]}`)
	result := SelfAugmentResult{OK: true, TerminationEligible: true, Summary: SelfAugmentSummary{MinimumGoalScore: 100}}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", AgyCommand: fake, TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory noisy llm eval should not fail self-verify: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || !updated.LLMEval.OK || updated.LLMEval.Score != 99 || updated.LLMEval.Error != "" {
		t.Fatalf("noisy agy output should extract strict JSON object: %+v", updated)
	}
}

func TestSelfVerifyLLMEvalMalformedOutputIsStructured(t *testing.T) {
	fake := writeFakeAgyForSelfVerifyTest(t, `not-json`)
	result := SelfAugmentResult{OK: true, TerminationEligible: true}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", AgyCommand: fake, TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory malformed llm eval should be recorded, not returned as gate error: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || updated.LLMEval.OK || !strings.Contains(updated.LLMEval.Error, "parse agy JSON") {
		t.Fatalf("malformed agy output should produce structured llm_eval error: %+v", updated)
	}
	if len(updated.LLMEval.Error) > 512 {
		t.Fatalf("llm eval error should be bounded, got %d bytes", len(updated.LLMEval.Error))
	}
}

func TestSelfVerifyLLMEvalUnknownFieldIsStructured(t *testing.T) {
	fake := writeFakeAgyForSelfVerifyTest(t, `{"ok":true,"score":99,"unexpected":true}`)
	result := SelfAugmentResult{OK: true, TerminationEligible: true}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", AgyCommand: fake, TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory unknown field should be recorded, not returned as gate error: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || updated.LLMEval.OK || !strings.Contains(updated.LLMEval.Error, "unknown field") {
		t.Fatalf("unknown agy field should produce structured llm_eval error: %+v", updated)
	}
}

func TestSelfVerifyLLMEvalCommandFailureIsStructured(t *testing.T) {
	fake := writeFailingFakeAgyForSelfVerifyTest(t)
	result := SelfAugmentResult{OK: true, TerminationEligible: true}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "advisory", AgyCommand: fake, TargetScore: 95})
	if err != nil {
		t.Fatalf("advisory command failure should be recorded, not returned as gate error: %v", err)
	}
	if !updated.OK || updated.LLMEval == nil || updated.LLMEval.OK || !strings.Contains(updated.LLMEval.Error, "agy -p failed") {
		t.Fatalf("command failure should produce structured llm_eval error: %+v", updated)
	}
}

func TestSelfVerifyLLMEvalResultClassifiesForegroundReadOnlyGate(t *testing.T) {
	fake := writeFakeAgyForSelfVerifyTest(t, `{"ok":true,"score":100,"summary":"pass","blockers":[],"risks":[],"recommended_next_actions":[]}`)
	result := SelfAugmentResult{OK: true, TerminationEligible: true, Summary: SelfAugmentSummary{MinimumGoalScore: 100, TerminationEligible: true}}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "gate", AgyCommand: fake, TargetScore: 95})
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
	fake := writeFakeAgyForSelfVerifyTest(t, `{"ok":false,"score":40,"summary":"blocked","blockers":["missing QA"]}`)
	result := SelfAugmentResult{OK: true, TerminationEligible: true, Summary: SelfAugmentSummary{MinimumGoalScore: 100, TerminationEligible: true}}
	updated, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{Enabled: true, Mode: "gate", AgyCommand: fake, TargetScore: 95})
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

func writeFakeAgyForSelfVerifyTest(t *testing.T, output string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agy.sh")
	content := "#!/bin/sh\nif [ \"$1\" != \"--dangerously-skip-permissions\" ] || [ \"$2\" != \"-p\" ]; then echo missing agy flags >&2; exit 2; fi\nprintf '%s\\n' '" + strings.ReplaceAll(output, "'", "'\\''") + "'\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFailingFakeAgyForSelfVerifyTest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agy-fail.sh")
	content := "#!/bin/sh\nif [ \"$1\" != \"--dangerously-skip-permissions\" ] || [ \"$2\" != \"-p\" ]; then echo missing agy flags >&2; exit 2; fi\necho model unavailable >&2\nexit 7\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
