package issueops

import (
	"encoding/json"
	"fmt"
	"reflect"

	"agent-harness/internal/core/issueops/handoff"
)

// CurrentOwnershipAttempt resolves mutation authority only through the durable
// active-attempt pointer. Historical position never implies live authority.
func CurrentOwnershipAttempt(record IssueOpsRecord) *IssueOpsOwnershipAttempt {
	if record.Ownership == nil || record.Ownership.ActiveAttempt <= 0 {
		return nil
	}
	for index := range record.Ownership.Attempts {
		if record.Ownership.Attempts[index].Number == record.Ownership.ActiveAttempt {
			return &record.Ownership.Attempts[index]
		}
	}
	return nil
}

// LastOwnershipAttempt returns audit history only. Callers must not treat it as
// live mutation authority when ActiveAttempt is zero or points elsewhere.
func LastOwnershipAttempt(record IssueOpsRecord) *IssueOpsOwnershipAttempt {
	if record.Ownership == nil || len(record.Ownership.Attempts) == 0 {
		return nil
	}
	return &record.Ownership.Attempts[len(record.Ownership.Attempts)-1]
}

func CloneIssueOpsOwnershipLedger(value *IssueOpsOwnershipLedger) *IssueOpsOwnershipLedger {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var cloned IssueOpsOwnershipLedger
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return nil
	}
	return &cloned
}

// AppendIssueOpsOwnershipAttempt performs an in-memory compare-and-swap over
// the ledger and appends one deep-copied successor without rewriting history.
func AppendIssueOpsOwnershipAttempt(record IssueOpsRecord, expected *IssueOpsOwnershipLedger, successor IssueOpsOwnershipAttempt) (IssueOpsRecord, error) {
	if !reflect.DeepEqual(record.Ownership, expected) {
		return record, fmt.Errorf("ownership ledger changed before successor append")
	}
	ledger := CloneIssueOpsOwnershipLedger(record.Ownership)
	if ledger == nil {
		return record, fmt.Errorf("ownership ledger is required before successor append")
	}
	encoded, err := json.Marshal(successor)
	if err != nil {
		return record, fmt.Errorf("clone successor ownership attempt: %w", err)
	}
	var cloned IssueOpsOwnershipAttempt
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return record, fmt.Errorf("clone successor ownership attempt: %w", err)
	}
	if len(ledger.Attempts) == 0 || cloned.Number <= ledger.Attempts[len(ledger.Attempts)-1].Number {
		return record, fmt.Errorf("successor ownership attempt number must increase monotonically")
	}
	ledger.Attempts = append(ledger.Attempts, cloned)
	ledger.ActiveAttempt = cloned.Number
	ledger.PendingRestart = nil
	updated := record
	updated.Ownership = ledger
	updated.CycleState = IssueOpsCycleActive
	if err := ValidateIssueOpsOwnershipLedger(updated); err != nil {
		return record, err
	}
	return updated, nil
}

func ValidateIssueOpsOwnershipLedger(record IssueOpsRecord) error {
	return handoff.ValidateOwnershipLedger(record)
}
