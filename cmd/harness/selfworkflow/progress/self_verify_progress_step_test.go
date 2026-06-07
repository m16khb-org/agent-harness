package progress

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/cmd/harness/commandstep"
)

func TestStepEndFailureKeepsLastSuccess(t *testing.T) {
	var buf bytes.Buffer
	reporter, err := NewSelfVerifyProgressReporter("jsonl", &buf)
	if err != nil {
		t.Fatalf("NewSelfVerifyProgressReporter: %v", err)
	}
	reporter.EmitStepEnd("self_verification", 1, 1, 100, 1, 2, commandstep.StepResult{Label: "go test", OK: true, DurationMS: 10})
	reporter.EmitStepEnd("self_verification", 1, 1, 100, 2, 2, commandstep.StepResult{Label: "go build", OK: false, Error: "build failed"})

	events := decodeProgressEventsForStepTest(t, buf.String())
	if len(events) != 2 {
		t.Fatalf("expected two step_end events, got %d: %s", len(events), buf.String())
	}
	failed := events[1]
	if failed.Event != "step_end" || failed.Step != "go build" || failed.OK == nil || *failed.OK || failed.LastSuccess != "go test" || failed.Error != "build failed" {
		t.Fatalf("unexpected failed step_end event: %+v", failed)
	}
}

func decodeProgressEventsForStepTest(t *testing.T, out string) []SelfVerifyProgressEvent {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	events := make([]SelfVerifyProgressEvent, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event SelfVerifyProgressEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode progress event: %v\n%s", err, line)
		}
		events = append(events, event)
	}
	return events
}
