package verifyloop

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/progress"
)

func TestSelfVerifyRejectsZeroIterations(t *testing.T) {
	result, err := SelfVerify(0, 100, 95, false, Deps{})
	if err == nil || !strings.Contains(err.Error(), "requires at least 1 iteration") {
		t.Fatalf("expected zero-iteration error, got result=%#v err=%v", result, err)
	}
	if result.OK || result.LoopKind != "self_verification" || result.Iterations != 0 || result.Summary.TotalRuns != 0 {
		t.Fatalf("unexpected zero-iteration result: %#v", result)
	}
}

func TestSelfVerifyProgressRejectsZeroIterationsWithLoopEndEvent(t *testing.T) {
	var buf bytes.Buffer
	reporter, err := progress.NewSelfVerifyProgressReporter("jsonl", &buf)
	if err != nil {
		t.Fatalf("NewSelfVerifyProgressReporter: %v", err)
	}

	result, err := SelfVerifyWithProgress(0, 123, 95, false, reporter, Deps{})
	if err == nil || !strings.Contains(err.Error(), "requires at least 1 iteration") {
		t.Fatalf("expected zero-iteration error, got result=%#v err=%v", result, err)
	}

	events := decodeProgressEventsForLoopTest(t, buf.String())
	if len(events) != 2 {
		t.Fatalf("expected loop_start and loop_end events, got %d: %s", len(events), buf.String())
	}
	if events[0].Event != "loop_start" || events[0].Iterations != 0 || events[0].Seed != 123 {
		t.Fatalf("unexpected loop_start event: %+v", events[0])
	}
	if events[1].Event != "loop_end" || events[1].OK == nil || *events[1].OK || !strings.Contains(events[1].Error, "requires at least 1 iteration") {
		t.Fatalf("unexpected loop_end event: %+v", events[1])
	}
}

func decodeProgressEventsForLoopTest(t *testing.T, out string) []progress.SelfVerifyProgressEvent {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	events := make([]progress.SelfVerifyProgressEvent, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event progress.SelfVerifyProgressEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode progress event: %v\n%s", err, line)
		}
		events = append(events, event)
	}
	return events
}
