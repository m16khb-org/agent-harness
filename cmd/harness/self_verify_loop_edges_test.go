package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSelfVerifyRejectsZeroIterationsThroughWrapper(t *testing.T) {
	result, err := selfVerify(0, 100, 95, false)
	if err == nil || !strings.Contains(err.Error(), "requires at least 1 iteration") {
		t.Fatalf("expected zero-iteration error, got result=%#v err=%v", result, err)
	}
	if result.OK || result.LoopKind != "self_verification" || result.Iterations != 0 || result.Summary.TotalRuns != 0 {
		t.Fatalf("unexpected zero-iteration wrapper result: %#v", result)
	}
}

func TestSelfVerifyProgressRejectsZeroIterationsWithLoopEndEvent(t *testing.T) {
	var buf bytes.Buffer
	reporter, err := newSelfVerifyProgressReporter("jsonl", &buf)
	if err != nil {
		t.Fatalf("newSelfVerifyProgressReporter: %v", err)
	}

	result, err := selfVerifyWithProgress(0, 123, 95, false, reporter)
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

func TestCachedContractGoldenStepCoversCachedBranch(t *testing.T) {
	cached := cachedContractGoldenStep(StepResult{Label: "go test", OK: true})
	if !cached.OK || cached.Command != "covered by go test ./... -count=1" || !strings.Contains(cached.Stdout, "already executed") {
		t.Fatalf("unexpected cached contract golden step: %+v", cached)
	}
}

func TestSelfVerifyProgressStepEndFailureKeepsLastSuccess(t *testing.T) {
	var buf bytes.Buffer
	reporter, err := newSelfVerifyProgressReporter("jsonl", &buf)
	if err != nil {
		t.Fatalf("newSelfVerifyProgressReporter: %v", err)
	}
	reporter.emitStepEnd("self_verification", 1, 1, 100, 1, 2, StepResult{Label: "go test", OK: true, DurationMS: 10})
	reporter.emitStepEnd("self_verification", 1, 1, 100, 2, 2, StepResult{Label: "go build", OK: false, Error: "build failed"})

	events := decodeProgressEventsForLoopTest(t, buf.String())
	if len(events) != 2 {
		t.Fatalf("expected two step_end events, got %d: %s", len(events), buf.String())
	}
	failed := events[1]
	if failed.Event != "step_end" || failed.Step != "go build" || failed.OK == nil || *failed.OK || failed.LastSuccess != "go test" || failed.Error != "build failed" {
		t.Fatalf("unexpected failed step_end event: %+v", failed)
	}
}

func decodeProgressEventsForLoopTest(t *testing.T, out string) []SelfVerifyProgressEvent {
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
