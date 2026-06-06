package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestSelfVerifyProgressReporterEmitsJSONL(t *testing.T) {
	var buf bytes.Buffer
	reporter, err := newSelfVerifyProgressReporter("jsonl", &buf)
	if err != nil {
		t.Fatalf("newSelfVerifyProgressReporter: %v", err)
	}
	if reporter == nil {
		t.Fatal("expected progress reporter")
	}
	reporter.emit(SelfVerifyProgressEvent{
		Event:      "loop_start",
		LoopKind:   "self_verification",
		Iterations: 10,
		Seed:       100,
	})
	reporter.emitStepEnd("self_verification", 1, 10, 100, 1, 13, StepResult{Label: "go test", OK: true, DurationMS: 25})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected progress lines: %q", buf.String())
	}
	var event SelfVerifyProgressEvent
	if err := json.Unmarshal([]byte(lines[1]), &event); err != nil {
		t.Fatalf("progress line is not JSON: %v\n%s", err, lines[1])
	}
	if event.Event != "step_end" || event.Step != "go test" || event.OK == nil || !*event.OK || event.LastSuccess != "go test" {
		t.Fatalf("unexpected step progress event: %+v", event)
	}
}

func TestSelfVerifyProgressReporterRejectsUnknownMode(t *testing.T) {
	if reporter, err := newSelfVerifyProgressReporter("xml", io.Discard); err == nil || reporter != nil {
		t.Fatalf("expected unsupported progress mode error, got reporter=%+v err=%v", reporter, err)
	}
}

func TestTailWithBudgetMarksTruncation(t *testing.T) {
	out, truncated, original := tailWithBudget(strings.Repeat("x", 100), 40)
	if !truncated || original != 100 {
		t.Fatalf("expected truncation metadata, got truncated=%v original=%d", truncated, original)
	}
	if len(out) > 40 || !strings.Contains(out, "truncated") {
		t.Fatalf("unexpected bounded output: len=%d out=%q", len(out), out)
	}
}

func TestFindUnredactedSecretLikeFlagsRealTokens(t *testing.T) {
	findings := findUnredactedSecretLike("OPENAI_API_KEY=sk-123456789012345678901234\n")
	if len(findings) == 0 {
		t.Fatal("expected unredacted secret finding")
	}
	if !strings.Contains(findings[0], "openai_token") {
		t.Fatalf("unexpected finding: %+v", findings)
	}
}

func TestFindUnredactedSecretLikeAllowsRedactedFixtures(t *testing.T) {
	findings := findUnredactedSecretLike("TOKEN=redacted\npassword=example\n")
	if len(findings) != 0 {
		t.Fatalf("redacted fixtures should be allowed: %+v", findings)
	}
}
