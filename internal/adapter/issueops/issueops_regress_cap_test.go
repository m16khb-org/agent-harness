package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func TestRegressIssueOpsForReplanRecordsRegressEvent(t *testing.T) {
	stateRoot, id := recordAtPhaseForRegressTest(t, IssueOpsPhasePlan)

	out, err := RegressIssueOpsForReplan(stateRoot, id, "first stop: scope too broad")
	if err != nil {
		t.Fatalf("regress: %v", err)
	}
	if len(out.RegressEvents) != 1 {
		t.Fatalf("regress must append one audit event, got %#v", out.RegressEvents)
	}
	event := out.RegressEvents[0]
	if event.Reason != "first stop: scope too broad" {
		t.Errorf("event reason = %q, want the stop reason", event.Reason)
	}
	if event.FromPhase != IssueOpsPhasePlan {
		t.Errorf("event from_phase = %q, want %q", event.FromPhase, IssueOpsPhasePlan)
	}
	if event.At == "" {
		t.Error("event must carry a timestamp")
	}
}

func TestRegressIssueOpsForReplanCapsRepeatedRegressions(t *testing.T) {
	// One below the cap: the regress is still allowed and appends its event.
	stateRoot, id := seedRegressEvents(t, issueOpsRegressCap-1)
	out, err := RegressIssueOpsForReplan(stateRoot, id, "still within cap")
	if err != nil {
		t.Fatalf("regress below cap must be allowed: %v", err)
	}
	if len(out.RegressEvents) != issueOpsRegressCap {
		t.Fatalf("regress events = %d, want %d", len(out.RegressEvents), issueOpsRegressCap)
	}

	// At the cap: fail-closed refusal that demands a human decision, no rewind.
	stateRoot2, id2 := seedRegressEvents(t, issueOpsRegressCap)
	if _, err := RegressIssueOpsForReplan(stateRoot2, id2, "one stop too many"); err == nil ||
		!strings.Contains(err.Error(), "human decision") {
		t.Fatalf("regress at cap must be refused with a human-decision escalation, got %v", err)
	}
	rec2, err := ReadIssueOps(stateRoot2, id2)
	if err != nil {
		t.Fatal(err)
	}
	if rec2.Phase != IssueOpsPhasePlan {
		t.Fatalf("refused regress must not rewind the phase, got %s", rec2.Phase)
	}
	if len(rec2.RegressEvents) != issueOpsRegressCap {
		t.Fatalf("refused regress must not append events, got %d", len(rec2.RegressEvents))
	}
}

func TestRegressCapErrorReportsActualEventCount(t *testing.T) {
	stateRoot, id := seedRegressEvents(t, issueOpsRegressCap+2)

	_, err := RegressIssueOpsForReplan(stateRoot, id, "too many stops")
	if err == nil {
		t.Fatal("expected cap error")
	}
	if !strings.Contains(err.Error(), "already went through 5 stop") {
		t.Fatalf("cap error should report actual event count, got %v", err)
	}
}

func seedRegressEvents(t *testing.T, count int) (string, string) {
	t.Helper()
	stateRoot, id := recordAtPhaseForRegressTest(t, IssueOpsPhasePlan)
	rec, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	for range count {
		rec.RegressEvents = append(rec.RegressEvents, issueops.IssueOpsRegressEvent{
			Reason:    "earlier stop",
			FromPhase: IssueOpsPhasePlan,
			At:        "2026-07-02T00:00:00Z",
		})
	}
	if _, err := touchAndWriteIssueOps(stateRoot, rec); err != nil {
		t.Fatal(err)
	}
	return stateRoot, id
}
