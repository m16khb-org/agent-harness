package loopresult

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/cmd/harness/selfworkflow/progress"
)

func TestNewBuildsSelfVerificationResultContract(t *testing.T) {
	got := New(10, 100, 95, "/repo")
	if got.LoopKind != "self_verification" || got.KoreanName != model.SelfVerificationKoreanName {
		t.Fatalf("unexpected loop identity: %#v", got)
	}
	if got.Iterations != 10 || got.BaseSeed != 100 || got.TargetScore != 95 || got.HarnessRoot != "/repo" {
		t.Fatalf("unexpected loop metadata: %#v", got)
	}
	if len(got.LoopContract) == 0 || !strings.Contains(strings.Join(got.LoopContract, "\n"), "target_score") {
		t.Fatalf("missing loop contract details: %#v", got.LoopContract)
	}
}

func TestEmitStartAndEndWriteProgressEvents(t *testing.T) {
	var buf bytes.Buffer
	reporter, err := progress.NewSelfVerifyProgressReporter("jsonl", &buf)
	if err != nil {
		t.Fatalf("new reporter: %v", err)
	}
	EmitStart(reporter, "self_verification", 2, 100)
	EmitEnd(reporter, "self_verification", 2, 100, false, "failed")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 progress lines, got %d: %q", len(lines), buf.String())
	}
	var start, end progress.SelfVerifyProgressEvent
	if err := json.Unmarshal([]byte(lines[0]), &start); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &end); err != nil {
		t.Fatalf("decode end: %v", err)
	}
	if start.Event != "loop_start" || start.Iterations != 2 || start.Seed != 100 {
		t.Fatalf("unexpected start event: %#v", start)
	}
	if end.Event != "loop_end" || end.OK == nil || *end.OK || end.Error != "failed" {
		t.Fatalf("unexpected end event: %#v", end)
	}
	EmitStart(nil, "ignored", 0, 0)
	EmitEnd(nil, "ignored", 0, 0, true, "")
}
