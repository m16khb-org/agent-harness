package issueops

import (
	"errors"
	"fmt"
	"testing"

	"agent-harness/internal/adapter/outbound/sqlstore"
	statecontract "agent-harness/internal/contract/state"
)

func TestReadIssueOpsRejectsRecordInvariantViolations(t *testing.T) {
	stateRoot := t.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		body string
	}{
		{name: "empty_phase", body: `"phase":""`},
		{name: "unknown_phase", body: `"phase":"unknown"`},
		{
			name: "phase_ledger_identity_mismatch",
			body: `"phase":"problem","phase_ledger":{"problem":{"phase":"plan"}}`,
		},
		{
			name: "unknown_plan_prep_status",
			body: `"phase":"problem","plan_prep":{"prior_decisions":{"status":"unknown"}}`,
		},
		{
			name: "unknown_cleanup_failure_step",
			body: `"phase":"done","cleanup_finish_failure":{"step":"secret-like-arbitrary-value"}`,
		},
		{name: "unknown_intent_class", body: `"phase":"problem","intent":{"intent_class":"unknown"}`},
		{name: "unknown_feedback_classification", body: `"phase":"feedback","feedback":[{"classification":"unknown"}]`},
		{name: "unknown_feedback_resolution", body: `"phase":"feedback","feedback":[{"resolution":"unknown"}]`},
		{name: "unknown_regress_phase", body: `"phase":"plan","regress_events":[{"from_phase":"unknown"}]`},
		{name: "unknown_routing_phase", body: `"phase":"plan","routing_trace":[{"phase":"unknown"}]`},
		{name: "unknown_branch_provider", body: `"phase":"plan","branch_prepare":{"provider":"bitbucket"}}`},
		{name: "unknown_remote_artifact_kind", body: `"phase":"pr","remote_artifact":{"provider":"github","kind":"mr"}}`},
		{name: "unknown_child_verdict", body: `"phase":"implement","child_cycles":[{"validation_verdict":"unknown"}]`},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			id := fmt.Sprintf("io-invalid%02d", index)
			raw := fmt.Sprintf(`{"schema_version":1,"id":%q,%s}`, id, test.body)
			if err := database.Put(issueOpsBucket, id, []byte(raw)); err != nil {
				t.Fatal(err)
			}

			_, err := ReadIssueOps(stateRoot, id)

			if !errors.Is(err, statecontract.ErrInvalidState) {
				t.Fatalf("ReadIssueOps error = %v, want invalid state", err)
			}
		})
	}
}

func TestReadIssueOpsRejectsInvalidStateMatrix(t *testing.T) {
	stateRoot := t.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		id   string
		raw  string
	}{
		{name: "missing_schema", id: "io-missing", raw: `{"id":"io-missing","phase":"problem"}`},
		{name: "zero_schema", id: "io-zero", raw: `{"schema_version":0,"id":"io-zero","phase":"problem"}`},
		{name: "future_schema", id: "io-future", raw: `{"schema_version":2,"id":"io-future","phase":"problem"}`},
		{name: "malformed_json", id: "io-malformed", raw: `{`},
		{name: "unknown_field", id: "io-unknown", raw: `{"schema_version":1,"id":"io-unknown","phase":"problem","unknown":true}`},
		{name: "id_mismatch", id: "io-requested", raw: `{"schema_version":1,"id":"io-other","phase":"problem"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := database.Put(issueOpsBucket, test.id, []byte(test.raw)); err != nil {
				t.Fatal(err)
			}

			_, err := ReadIssueOps(stateRoot, test.id)

			if !errors.Is(err, statecontract.ErrInvalidState) ||
				err.Error() != statecontract.ErrInvalidState.Error() {
				t.Fatalf("ReadIssueOps error = %v, want invalid state", err)
			}
		})
	}
}
