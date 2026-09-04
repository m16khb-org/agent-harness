package state_test

import (
	"errors"
	"testing"

	statecontract "issueops/internal/contract/state"
	statedomain "issueops/internal/domain/state"
)

func TestValidateRecordAcceptsOnlyExactCurrentV1(t *testing.T) {
	valid := statecontract.RecordEnvelope{SchemaVersion: 1, Key: "key", Content: "value", UpdatedAt: "now", Bytes: 5}
	if err := statedomain.ValidateRecord("key", valid); err != nil {
		t.Fatalf("valid v1 rejected: %v", err)
	}
	for name, record := range map[string]statecontract.RecordEnvelope{
		"missing_schema": {Key: "key", Content: "value", UpdatedAt: "now", Bytes: 5},
		"future_schema":  {SchemaVersion: 2, Key: "key", Content: "value", UpdatedAt: "now", Bytes: 5},
		"key_mismatch":   {SchemaVersion: 1, Key: "other", Content: "value", UpdatedAt: "now", Bytes: 5},
		"byte_mismatch":  {SchemaVersion: 1, Key: "key", Content: "value", UpdatedAt: "now", Bytes: 4},
	} {
		t.Run(name, func(t *testing.T) {
			if err := statedomain.ValidateRecord("key", record); !errors.Is(err, statecontract.ErrInvalidState) || err.Error() != "invalid state" {
				t.Fatalf("invalid record returned %v", err)
			}
		})
	}
}
