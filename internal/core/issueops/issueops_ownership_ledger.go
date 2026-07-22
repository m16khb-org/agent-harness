package issueops

import (
	"encoding/json"
	"fmt"
	"reflect"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

// CurrentOwnershipAttempt resolves mutation authority only through the durable
// active-attempt pointer. Historical position never implies live authority.
func CurrentOwnershipAttempt(record IssueOpsRecord) *IssueOpsOwnershipAttempt {
	return model.CurrentOwnershipAttempt(record)
}

// LastOwnershipAttempt returns audit history only. Callers must not treat it as
// live mutation authority when ActiveAttempt is zero or points elsewhere.
func LastOwnershipAttempt(record IssueOpsRecord) *IssueOpsOwnershipAttempt {
	return model.LastOwnershipAttempt(record)
}

func currentIssueOpsWorkspace(record IssueOpsRecord) *IssueOpsExecutionWorkspace {
	return model.CurrentExecutionWorkspace(record)
}

func currentIssueOpsHandoff(record IssueOpsRecord) *IssueOpsExecutionHandoff {
	return model.CurrentExecutionHandoff(record)
}

func lastIssueOpsHandoff(record IssueOpsRecord) *IssueOpsExecutionHandoff {
	attempt := LastOwnershipAttempt(record)
	if attempt == nil {
		return nil
	}
	return attempt.Handoff
}

func retainedCleanupHandoff(record IssueOpsRecord) *IssueOpsExecutionHandoff {
	if record.CycleState != IssueOpsCycleClosed {
		return nil
	}
	h := lastIssueOpsHandoff(record)
	if h == nil || h.Completion == nil {
		return nil
	}
	return h
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
