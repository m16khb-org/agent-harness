package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"agent-harness/internal/core/issueops/handoff"
)

type IssueOpsV8MigrationClassification struct {
	OK              bool               `json:"ok"`
	ID              string             `json:"id"`
	Classification  string             `json:"classification"`
	CycleState      IssueOpsCycleState `json:"cycle_state"`
	Phase           IssueOpsPhase      `json:"phase"`
	ActiveAttempt   int                `json:"active_attempt,omitempty"`
	RawSHA256       string             `json:"raw_sha256"`
	CanonicalSHA256 string             `json:"canonical_sha256"`
}

func ClassifyIssueOpsV8Migration(raw []byte) (IssueOpsV8MigrationClassification, error) {
	var record IssueOpsRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return IssueOpsV8MigrationClassification{}, fmt.Errorf("decode issueops v8 migration input: %w", err)
	}
	if record.SchemaVersion != 8 {
		return IssueOpsV8MigrationClassification{}, fmt.Errorf("unsupported issueops schema_version %d for v8 migration", record.SchemaVersion)
	}
	migrated, err := MigrateIssueOpsV8Record(record)
	if err != nil {
		return IssueOpsV8MigrationClassification{}, err
	}
	canonical, err := json.Marshal(record)
	if err != nil {
		return IssueOpsV8MigrationClassification{}, err
	}
	classification := "active_without_owner"
	if record.ExecutionHandoff != nil {
		switch migrated.CycleState {
		case IssueOpsCyclePaused:
			classification = "paused_cancelled_owner"
		case IssueOpsCycleClosed:
			classification = "closed_owner"
		default:
			classification = "active_owner"
		}
	} else if migrated.CycleState == IssueOpsCycleClosed {
		classification = "closed_without_owner"
	}
	active := 0
	if migrated.Ownership != nil {
		active = migrated.Ownership.ActiveAttempt
	}
	return IssueOpsV8MigrationClassification{
		OK: true, ID: record.ID, Classification: classification, CycleState: migrated.CycleState, Phase: migrated.Phase, ActiveAttempt: active,
		RawSHA256: issueOpsMigrationSHA256(raw), CanonicalSHA256: issueOpsMigrationSHA256(canonical),
	}, nil
}

func MigrateIssueOpsV8Record(record IssueOpsRecord) (IssueOpsRecord, error) {
	if record.SchemaVersion != 8 {
		return record, fmt.Errorf("issueops schema_version 8 is required for migration")
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return record, err
	}
	var migrated IssueOpsRecord
	if err := json.Unmarshal(encoded, &migrated); err != nil {
		return record, err
	}
	migrated.SchemaVersion = IssueOpsCurrentSchemaVersion
	migrated.CycleState = IssueOpsCycleActive
	migrated.Ownership = nil

	workspace, owner := migrated.ExecutionWorkspace, migrated.ExecutionHandoff
	if (workspace == nil) != (owner == nil) {
		return record, fmt.Errorf("v8 ownership migration requires both workspace and handoff")
	}
	if owner == nil {
		if migrated.Phase == IssueOpsPhaseDone {
			migrated.CycleState = IssueOpsCycleClosed
		}
		migrated.ExecutionWorkspace = nil
		migrated.ExecutionHandoff = nil
		if err := ValidateIssueOpsOwnershipLedger(migrated); err != nil {
			return record, err
		}
		return migrated, nil
	}
	if owner.Attempt <= 0 || owner.WorkspaceEpoch == "" || workspace.WorkspaceEpoch == "" || owner.WorkspaceEpoch != workspace.WorkspaceEpoch {
		return record, fmt.Errorf("v8 ownership attempt and workspace epoch do not agree")
	}
	legacy := migrated
	legacy.SchemaVersion = 8
	legacy.CycleState = ""
	legacy.Ownership = nil
	if err := handoff.ValidateEnvelope(legacy); err != nil {
		return record, fmt.Errorf("v8 ownership envelope is invalid: %w", err)
	}
	startedAt := firstIssueOpsMigrationTimestamp(owner.PreparedAt, workspace.PreparedAt, migrated.CreatedAt)
	if startedAt == "" {
		return record, fmt.Errorf("v8 ownership attempt has no deterministic start timestamp")
	}
	attempt := IssueOpsOwnershipAttempt{Number: owner.Attempt, Workspace: workspace, Handoff: owner, StartedAt: startedAt}
	migrated.Ownership = &IssueOpsOwnershipLedger{Attempts: []IssueOpsOwnershipAttempt{attempt}}
	migrated.ExecutionWorkspace = nil
	migrated.ExecutionHandoff = nil

	pausedCancelled := migrated.Phase == IssueOpsPhaseDone && owner.State == handoff.StateClosed && owner.ClosedDisposition == handoff.DispositionCancelled && migrated.ForceReleasedAt != "" && migrated.RemoteArtifact == nil
	switch {
	case pausedCancelled:
		phase, ok := highestEnteredIssueOpsMigrationPhase(migrated.PhaseLedger)
		if !ok {
			return record, fmt.Errorf("paused v8 migration requires a non-done entered phase in phase_ledger")
		}
		migrated.Phase = phase
		migrated.CycleState = IssueOpsCyclePaused
		migrated.Ownership.Attempts[0].ClosedAt = firstIssueOpsMigrationTimestamp(owner.UpdatedAt, migrated.ForceReleasedAt)
	case migrated.Phase == IssueOpsPhaseDone:
		if owner.State != handoff.StateClosed {
			return record, fmt.Errorf("done v8 ownership record has nonterminal handoff state")
		}
		migrated.CycleState = IssueOpsCycleClosed
		migrated.Ownership.Attempts[0].ClosedAt = firstIssueOpsMigrationTimestamp(owner.UpdatedAt, owner.CompletedAt, migrated.UpdatedAt)
	default:
		if owner.State == handoff.StateClosed {
			return record, fmt.Errorf("non-done v8 ownership record has closed handoff without paused classification")
		}
		migrated.CycleState = IssueOpsCycleActive
		migrated.Ownership.ActiveAttempt = owner.Attempt
	}
	if err := ValidateIssueOpsOwnershipLedger(migrated); err != nil {
		return record, err
	}
	return migrated, nil
}

func PreviewIssueOpsV9Migration(stateRoot, id string) (IssueOpsV8MigrationClassification, error) {
	raw, err := readRawIssueOpsBytes(stateRoot, id)
	if err != nil {
		return IssueOpsV8MigrationClassification{}, err
	}
	return ClassifyIssueOpsV8Migration(raw)
}

func ConfirmIssueOpsV9Migration(stateRoot, id string) (IssueOpsV8MigrationClassification, error) {
	preview, err := PreviewIssueOpsV9Migration(stateRoot, id)
	if err != nil {
		return IssueOpsV8MigrationClassification{}, err
	}
	var confirmed IssueOpsV8MigrationClassification
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		raw, readErr := readRawIssueOpsBytes(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		current, classifyErr := ClassifyIssueOpsV8Migration(raw)
		if classifyErr != nil {
			return classifyErr
		}
		if current.RawSHA256 != preview.RawSHA256 || current.CanonicalSHA256 != preview.CanonicalSHA256 {
			return fmt.Errorf("issueops v8 record changed after migration preview")
		}
		var v8 IssueOpsRecord
		if err := json.Unmarshal(raw, &v8); err != nil {
			return err
		}
		migrated, migrateErr := MigrateIssueOpsV8Record(v8)
		if migrateErr != nil {
			return migrateErr
		}
		if _, writeErr := writeIssueOps(stateRoot, migrated); writeErr != nil {
			return writeErr
		}
		confirmed = current
		return nil
	})
	return confirmed, err
}

func highestEnteredIssueOpsMigrationPhase(ledger IssueOpsPhaseLedger) (IssueOpsPhase, bool) {
	var selected IssueOpsPhase
	for _, phase := range IssueOpsPhases {
		if phase == IssueOpsPhaseDone {
			continue
		}
		entry, ok := ledger[phase]
		if ok && strings.TrimSpace(entry.EnteredAt) != "" {
			selected = phase
		}
	}
	return selected, selected != ""
}

func firstIssueOpsMigrationTimestamp(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func issueOpsMigrationSHA256(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
