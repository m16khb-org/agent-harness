package issueops

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/internal/core/sqlstore"
)

func TestReadRejectsRemovedHandoffProtocolVersionWithoutRewriting(t *testing.T) {
	stateRoot := t.TempDir()
	id := "io-removed-protocol"
	raw := []byte(`{"ok":true,"schema_version":7,"id":"io-removed-protocol","repo":"/repo/example","branch":"16-demo","phase":"implement","execution_handoff":{"protocol_version":2,"state":"owner_active"}}`)
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", id, raw); err != nil {
		t.Fatal(err)
	}
	got, err := ReadIssueOps(stateRoot, id)
	if err == nil || !strings.Contains(err.Error(), "protocol_version was removed") || got.OK {
		t.Fatalf("removed handoff protocol must fail closed: record=%#v err=%v", got, err)
	}
	after, ok, err := db.Get("issueops", id)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if !bytes.Equal(after, raw) {
		t.Fatal("rejected protocol record was rewritten")
	}
}

func TestReadRejectsPreOwnershipSchemaHandoffWithoutCompatibilityProjection(t *testing.T) {
	stateRoot := t.TempDir()
	id := "io-removed-handoff-shape"
	raw := []byte(`{"ok":true,"schema_version":7,"id":"io-removed-handoff-shape","repo":"/repo/example","branch":"16-demo","phase":"implement","execution_handoff":{"state":"submitted","coordinator_session":{"host":"codex","session_id":"old"}}}`)
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, id, raw); err != nil {
		t.Fatal(err)
	}

	got, err := ReadIssueOps(stateRoot, id)
	if err == nil || !strings.Contains(err.Error(), "predates the current ownership contract") || got.OK {
		t.Fatalf("removed handoff shape must be rejected: record=%#v err=%v", got, err)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, id); !bytes.Equal(after, raw) {
		t.Fatalf("rejected handoff record was rewritten\n got: %s\nwant: %s", after, raw)
	}
}

func TestCurrentHandoffRoundTripContainsNoProtocolVersion(t *testing.T) {
	stateRoot, record, _ := ownershipActiveRecorderRecord(t)
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(persisted.ExecutionHandoff)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "protocol_version") {
		t.Fatalf("current handoff persisted a protocol version: %s", encoded)
	}
}

func rawIssueOpsBytesForTest(t *testing.T, stateRoot, id string) []byte {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	b, ok, err := db.Get(issueOpsBucket, id)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("missing IssueOps row %q", id)
	}
	return append([]byte(nil), b...)
}
