package state_test

import (
	"errors"
	"io/fs"
	"testing"

	statecontract "agent-harness/internal/contract/state"
	"agent-harness/internal/core/sqlstore"
	corestate "agent-harness/internal/core/state"
)

func TestStateInvalidMatrix(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	db, err := sqlstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		raw  string
	}{
		{name: "missing_schema", raw: `{"key":"missing_schema","content":"x","bytes":1}`},
		{name: "schema_zero", raw: `{"schema_version":0,"key":"schema_zero","content":"x","bytes":1}`},
		{name: "future_schema", raw: `{"schema_version":2,"key":"future_schema","content":"x","bytes":1}`},
		{name: "malformed_json", raw: `{`},
		{name: "legacy_field", raw: `{"schema_version":1,"key":"legacy_field","content":"x","bytes":1,"legacy_authority":"x"}`},
		{name: "key_mismatch", raw: `{"schema_version":1,"key":"other","content":"x","bytes":1}`},
		{name: "byte_mismatch", raw: `{"schema_version":1,"key":"byte_mismatch","content":"x","bytes":2}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Put("state", tc.name, []byte(tc.raw)); err != nil {
				t.Fatal(err)
			}
			_, err := corestate.StateRead(tc.name)
			if !errors.Is(err, statecontract.ErrInvalidState) || err.Error() != "invalid state" {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestStateAbsentAndCurrentV1(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	db, err := sqlstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("state", "current_v1", []byte(`{"schema_version":1,"key":"current_v1","content":"x","updated_at":"2026-08-02T00:00:00Z","bytes":1}`)); err != nil {
		t.Fatal(err)
	}
	result, err := corestate.StateRead("current_v1")
	if err != nil || result.Record.Content != "x" || result.Record.SchemaVersion != statecontract.SchemaVersion {
		t.Fatalf("current v1 result=%+v err=%v", result, err)
	}

	_, err = corestate.StateRead("absent")
	if !errors.Is(err, fs.ErrNotExist) || errors.Is(err, statecontract.ErrInvalidState) {
		t.Fatalf("absent error=%v", err)
	}
}
