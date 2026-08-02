package state

import statecontract "agent-harness/internal/contract/state"

func ValidateRecord(expectedKey string, record statecontract.RecordEnvelope) error {
	if record.SchemaVersion != statecontract.SchemaVersion || record.Key != expectedKey || record.Bytes != len([]byte(record.Content)) {
		return statecontract.Invalid("record_invariant")
	}
	return nil
}
