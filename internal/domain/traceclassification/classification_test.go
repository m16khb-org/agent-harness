package classification

import "testing"

func TestProposedKnobForStep(t *testing.T) {
	tests := []struct {
		step     string
		contains string
	}{
		{"contract golden test", "tighten CLI/MCP contract"},
		{"secret redaction", "extend redaction audit"},
		{"daemon resilience", "add daemon stale-lock"},
		{"policy check", "add deterministic policy"},
		{"go test failure", "reduce failing test to a fixture"},
		{"build error", "keep build failure fix"},
		{"unknown step", "classify the repeated trace pattern"},
	}
	for _, tt := range tests {
		got := ProposedKnobForStep(tt.step)
		if got == "" {
			t.Errorf("ProposedKnobForStep(%q) returned empty", tt.step)
		}
		if !stringsContain(got, tt.contains) {
			t.Errorf("ProposedKnobForStep(%q) = %q, want containing %q", tt.step, got, tt.contains)
		}
	}
}

func TestOverfitRiskForClass(t *testing.T) {
	tests := []struct {
		class    string
		contains string
	}{
		{"deterministic", "medium:"},
		{"intermittent", "high:"},
		{"single_failure_observation", "high:"},
		{"unknown", "medium:"},
	}
	for _, tt := range tests {
		got := OverfitRiskForClass(tt.class)
		if got == "" {
			t.Errorf("OverfitRiskForClass(%q) returned empty", tt.class)
		}
		if !stringsContain(got, tt.contains) {
			t.Errorf("OverfitRiskForClass(%q) = %q, want containing %q", tt.class, got, tt.contains)
		}
	}
}

func TestDefaultVerificationCommand(t *testing.T) {
	tests := []struct {
		step     string
		contains string
	}{
		{"contract golden", "go test ./cmd/issueops -run Golden"},
		{"policy guard", "go test ./internal/core"},
		{"build fix", "go build -o bin/issueops"},
		{"unknown", "go test ./... -count=1"},
	}
	for _, tt := range tests {
		got := DefaultVerificationCommand(tt.step)
		if got == "" {
			t.Errorf("DefaultVerificationCommand(%q) returned empty", tt.step)
		}
		if !stringsContain(got, tt.contains) {
			t.Errorf("DefaultVerificationCommand(%q) = %q, want containing %q", tt.step, got, tt.contains)
		}
	}
}

func stringsContain(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
