package stateio

import (
	"encoding/json"
	"fmt"
	"time"

	"agent-harness/internal/core"
	"agent-harness/internal/core/failurecause"
)

func ReadSelfAugmentStateSnapshot(key string) (SelfAugmentStateSnapshot, error) {
	state, err := core.StateRead(key)
	if err != nil {
		return SelfAugmentStateSnapshot{}, err
	}
	var snapshot SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		return SelfAugmentStateSnapshot{}, err
	}
	if !IsSelfVerificationSummaryKind(snapshot.Kind) {
		return SelfAugmentStateSnapshot{}, fmt.Errorf("state key %q contains kind %q, want %s", key, snapshot.Kind, selfVerificationSummaryKind)
	}
	if snapshot.SchemaVersion != 1 {
		return SelfAugmentStateSnapshot{}, fmt.Errorf("state key %q has unsupported self-verification summary schema %d", key, snapshot.SchemaVersion)
	}
	NormalizeSelfAugmentSnapshotFailureCause(&snapshot)
	return snapshot, nil
}

func IsSelfVerificationSummaryKind(kind string) bool {
	return kind == selfVerificationSummaryKind || kind == legacySelfAugmentSummaryKind
}
func NormalizeSelfAugmentSnapshotFailureCause(snapshot *SelfAugmentStateSnapshot) {
	classified := failurecause.Classify(snapshot.Summary.FailedSteps > 0, snapshot.Summary.FailureCauseEvidence)
	snapshot.Summary.FailureCause = classified.Cause
	snapshot.Summary.FailureCauseReason = classified.Reason
	snapshot.Summary.FailureCauseEvidence = classified.Evidence
}

func WriteSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	NormalizeSelfAugmentSnapshotFailureCause(&snapshot)
	key, err := core.NormalizeStateKey(key)
	if err != nil {
		return err
	}
	content, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	record := core.StateRecord{
		SchemaVersion: core.StateCurrentSchemaVersion,
		Key:           key,
		Content:       string(content),
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Bytes:         len(content),
	}
	// SA1: persist via the locked + atomic (temp+rename) state writer instead of a
	// raw os.WriteFile, matching core.writeStateRecord's durability. Byte-identical
	// on-disk output (same path, MarshalIndent, trailing newline), now crash-safe
	// and serialized against concurrent writers.
	_, err = core.WriteStateRecord(dir, key, record)
	return err
}
