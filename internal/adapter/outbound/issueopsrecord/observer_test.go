package issueopsrecord

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestJSONLineObserverEmitsOnlyActionableRedactedSpanEvents(t *testing.T) {
	var output bytes.Buffer
	observer := NewJSONLineObserver(&output, 100*time.Millisecond)

	observer.Observe(SpanObservation{
		Operation: "routing.update",
		Outcome:   "success",
		WaitMS:    1,
		HoldMS:    2,
	})
	if output.Len() != 0 {
		t.Fatalf("fast uncontended span emitted noise: %q", output.String())
	}

	observer.Observe(SpanObservation{
		Operation: "routing.update",
		Outcome:   "success",
		Contended: true,
		WaitMS:    4,
		HoldMS:    2,
	})
	line := strings.TrimSpace(output.String())
	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatal(err)
	}
	if event["event"] != "issueops_record_span" ||
		event["operation"] != "routing.update" ||
		event["contended"] != true {
		t.Fatalf("unexpected span event: %v", event)
	}
	for _, forbidden := range []string{"root", "id", "bucket", "payload"} {
		if _, found := event[forbidden]; found {
			t.Fatalf("span event leaked %s: %v", forbidden, event)
		}
	}
}

func TestStoreScopesSpanObservationByCapability(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-observe01"
	data, err := json.Marshal(issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Phase:         issueopscontract.IssueOpsPhaseProblem,
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(Bucket(), id, data); err != nil {
		t.Fatal(err)
	}

	var observations []SpanObservation
	store := Store{
		Scope: "decision",
		Observer: ObserverFunc(func(observation SpanObservation) {
			observations = append(observations, observation)
		}),
	}
	if _, err := store.Update(
		context.Background(),
		stateRoot,
		id,
		func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
			record.Branch = "observed"
			return record, true, nil
		},
	); err != nil {
		t.Fatal(err)
	}

	if len(observations) != 1 || observations[0].Operation != "decision.update" {
		t.Fatalf("scoped observations = %+v", observations)
	}
}
