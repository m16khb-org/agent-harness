package selfworkflow

import (
	"strings"
	"testing"
)

func TestSelfOrchestrationHelpersReturnStableFallbacks(t *testing.T) {
	if got := selectedCandidateID(nil); got != "" {
		t.Fatalf("selectedCandidateID(nil)=%q, want empty string", got)
	}
	if got := SelectedSelfVerificationCandidateID(nil); got != "none" {
		t.Fatalf("SelectedSelfVerificationCandidateID(nil)=%q, want none", got)
	}

	augment := SelfAugmentCandidate{ID: "augment-next"}
	verify := SelfVerificationCandidate{ID: "verify-next"}
	if got := selectedCandidateID(&augment); got != "augment-next" {
		t.Fatalf("selectedCandidateID returned %q", got)
	}
	if got := SelectedSelfVerificationCandidateID(&verify); got != "verify-next" {
		t.Fatalf("SelectedSelfVerificationCandidateID returned %q", got)
	}
}

func TestStateKeySlugNormalizesUnsafeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "trims and lowercases", in: "  Ship This Lesson  ", want: "ship-this-lesson"},
		{name: "collapses punctuation", in: "A/B:C___D", want: "a-b-c-d"},
		{name: "fallback for empty slug", in: "!!!", want: "lesson"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StateKeySlug(tt.in); got != tt.want {
				t.Fatalf("StateKeySlug(%q)=%q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSelfVerifyLLMEvalEnvParsesDisabledAliasesAndRejectsUnknown(t *testing.T) {
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
	if _, _, err := ParseSelfVerifyLLMEvalEnv("maybe"); err == nil || !strings.Contains(err.Error(), selfVerifyLLMEvalEnv) {
		t.Fatalf("expected named env parse error, got %v", err)
	}
}

func TestDecodeSelfVerifyLLMEvalStrictRejectsExtraJSONValue(t *testing.T) {
	var eval SelfVerifyLLMEvalResult
	err := DecodeSelfVerifyLLMEvalStrict([]byte(`{"ok":true,"mode":"advisory","execution_class":"foreground_blocking","read_only":true,"score":99,"blockers":[],"risks":[],"recommended_next_actions":[],"evidence_packet_bytes":10} {"ok":false}`), &eval)
	if err == nil || !strings.Contains(err.Error(), "unexpected extra JSON value") {
		t.Fatalf("expected extra JSON value error, got %v", err)
	}
}
