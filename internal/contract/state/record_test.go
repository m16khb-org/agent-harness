package state_test

import (
	"encoding/json"
	"testing"

	"issueops/internal/contract/state"
)

func TestRecordEnvelopeRequiresExplicitCurrentV1(t *testing.T) {
	record := state.RecordEnvelope{SchemaVersion: state.SchemaVersion, Key: "key", Content: "value", UpdatedAt: "2026-08-02T00:00:00Z", Bytes: 5}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"schema_version":1,"key":"key","content":"value","updated_at":"2026-08-02T00:00:00Z","bytes":5}` {
		t.Fatalf("unexpected current-v1 envelope: %s", encoded)
	}
}
