package issueopslease

import (
	"errors"
	"testing"

	statecontract "issueops/internal/contract/state"
)

func TestIssueOpsLeaseInvalidMatrix(t *testing.T) {
	cases := []struct {
		name string
		id   string
		raw  string
	}{
		{name: "missing_schema", id: "io-missing", raw: `{"id":"io-missing"}`},
		{name: "schema_zero", id: "io-zero", raw: `{"schema_version":0,"id":"io-zero"}`},
		{name: "future_schema", id: "io-future", raw: `{"schema_version":2,"id":"io-future"}`},
		{name: "malformed_json", id: "io-malformed", raw: `{`},
		{name: "legacy_authority", id: "io-legacy", raw: `{"schema_version":1,"id":"io-legacy","execution_handoff":{"legacy":true}}`},
		{name: "id_mismatch", id: "io-expected", raw: `{"schema_version":1,"id":"io-other"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(tc.id, []byte(tc.raw))
			if !errors.Is(err, statecontract.ErrInvalidState) || err.Error() != "invalid state" {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestIssueOpsLeaseCurrentV1(t *testing.T) {
	const id = "io-current"
	record, err := Decode(id, []byte(`{"schema_version":1,"id":"io-current"}`))
	if err != nil || record.SchemaVersion != SchemaVersion || record.ID != id {
		t.Fatalf("record=%+v err=%v", record, err)
	}
	encoded, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(id, encoded)
	if err != nil || decoded.SchemaVersion != SchemaVersion || decoded.ID != id {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
}

func TestIssueOpsLeaseEncodeRejectsSchemaZero(t *testing.T) {
	_, err := Encode(Record{ID: "io-zero"})
	if !errors.Is(err, statecontract.ErrInvalidState) || err.Error() != "invalid state" {
		t.Fatalf("error=%v", err)
	}
}
