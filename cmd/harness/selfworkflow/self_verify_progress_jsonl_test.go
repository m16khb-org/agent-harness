package selfworkflow

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestSelfVerifyProgressReporterEmitsJSONL(t *testing.T) {
	var buf bytes.Buffer
	reporter, err := NewSelfVerifyProgressReporter("jsonl", &buf)
	if err != nil {
		t.Fatalf("NewSelfVerifyProgressReporter: %v", err)
	}
	if reporter == nil {
		t.Fatal("expected progress reporter")
	}
	reporter.Emit(SelfVerifyProgressEvent{
		Event:      "loop_start",
		LoopKind:   "self_verification",
		Iterations: 10,
		Seed:       100,
	})
	reporter.EmitStepEnd("self_verification", 1, 10, 100, 1, 13, StepResult{Label: "go test", OK: true, DurationMS: 25})

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
	if reporter, err := NewSelfVerifyProgressReporter("xml", io.Discard); err == nil || reporter != nil {
		t.Fatalf("expected unsupported progress mode error, got reporter=%+v err=%v", reporter, err)
	}
}
