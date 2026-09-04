package issueopsapp

import (
	"bytes"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/issueopsrecord"
)

func TestIssueOpsRecordObserverEmitsScopedActionableEvent(t *testing.T) {
	var output bytes.Buffer
	observer := issueOpsRecordObserver(&output)
	store := issueOpsRecordStore("routing", observer)
	store.Observer.Observe(issueopsrecord.SpanObservation{
		Operation: "routing.update",
		Outcome:   "success",
		Contended: true,
		WaitMS:    2,
		HoldMS:    1,
	})

	line := output.String()
	if !strings.Contains(line, `"event":"issueops_record_span"`) ||
		!strings.Contains(line, `"operation":"routing.update"`) {
		t.Fatalf("actionable span event missing: %q", line)
	}
}
