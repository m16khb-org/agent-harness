package rerun

import (
	"strings"
	"testing"
)

func TestSelfVerifyStepRerunCommandCoversOperationalLabels(t *testing.T) {
	tests := map[string]string{
		"go build":              "go build -o bin/agent-harness ./cmd/harness",
		"command audit smoke":   "policy audit",
		"candidate export":      "self-verify candidates",
		"step budget baseline":  "self-verify compare",
		"daemon resilience":     "daemon start",
		"redaction audit":       "go test ./cmd/harness -run Test -count=1",
		"unknown helper branch": "",
	}
	for label, want := range tests {
		t.Run(label, func(t *testing.T) {
			got, ok := SelfVerifyStepRerunCommand(label)
			if want == "" {
				if ok || got != "" {
					t.Fatalf("expected no rerun command for %q, got %q ok=%v", label, got, ok)
				}
				return
			}
			if !ok || !strings.Contains(got, want) {
				t.Fatalf("SelfVerifyStepRerunCommand(%q)=%q ok=%v, want substring %q", label, got, ok, want)
			}
		})
	}
}
