package issueops_test

import (
	"encoding/json"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func TestRecordUsesStablePhaseAndCurrentTypedSidecars(t *testing.T) {
	record := issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            "io-1",
		Repo:          "/repo",
		Phase:         issueops.IssueOpsPhaseImplement,
		Execution:     &issueops.Execution{Mode: issueops.ExecutionModeDirect},
		CreatedAt:     "created",
		UpdatedAt:     "updated",
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded["phase"]) != `"implement"` || string(decoded["execution"]) != `{"mode":"direct","workspace":{"source_root":"","root":"","branch":"","base_head":"","driver":"","linked_at":""},"lease":{"generation":0,"status":""}}` {
		t.Fatalf("current record drift: %s", encoded)
	}
}
