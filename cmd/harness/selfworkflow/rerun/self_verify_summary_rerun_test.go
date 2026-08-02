package rerun

import (
	"strings"
	"testing"
)

func TestSelfVerifyStepRerunCommandCoversOperationalLabels(t *testing.T) {
	tests := map[string]string{
		"go test":                "go test ./... -count=1",
		"contract golden tests":  "contractgolden",
		"risk QA tier":           "go vet ./...",
		"go build":               "go build -o bin/agent-harness ./cmd/harness",
		"inspect smoke":          "inspect --json",
		"docs index smoke":       "docs --json",
		"candidate export":       "self-verify candidates",
		"step budget baseline":   "self-verify compare",
		"install dry-run smoke":  "install --dry-run",
		"command policy smoke":   "policy check",
		"command audit smoke":    "policy audit",
		"contract check":         "contract check",
		"worker lifecycle smoke": "worker enqueue",
		"MCP smoke":              "mcp",
		"state roundtrip":        "state write",
		"parallel isolation":     "self-verify --full",
		"daemon resilience":      "daemon start",
		"preflight fuzz":         "preflight --json",
		"native integration":     "install-native.sh",
		"redaction audit":        "go test ./cmd/harness -run Test -count=1",
		"QA gate":                "go test ./cmd/harness -run Test -count=1",
		"harness invariants":     "go test ./cmd/harness -run Test -count=1",
		"unknown helper branch":  "",
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

func TestSelfVerifyRerunCommandsAndScoreFormatting(t *testing.T) {
	commands := SelfVerifyRerunCommands("go test", 1, 100, 95.5)
	if len(commands) != 2 {
		t.Fatalf("expected specific and full rerun commands, got %#v", commands)
	}
	if !strings.Contains(commands[1], "--iterations=10") || !strings.Contains(commands[1], "--target-score=95.5") {
		t.Fatalf("full rerun command did not clamp iterations or format target: %q", commands[1])
	}
	commands = SelfVerifyRerunCommands("unknown", 12, 200, 95)
	if len(commands) != 1 || !strings.Contains(commands[0], "--iterations=12") || !strings.Contains(commands[0], "--target-score=95") {
		t.Fatalf("unexpected fallback rerun command: %#v", commands)
	}
	if FormatScore(100) != "100" || FormatScore(99.25) != "99.25" {
		t.Fatal("FormatScore should preserve integer and fractional forms")
	}
}
