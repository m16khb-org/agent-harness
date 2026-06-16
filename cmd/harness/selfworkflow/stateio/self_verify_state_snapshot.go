package stateio

import (
	"encoding/json"
	"fmt"
	"time"

	"agent-harness/internal/core"
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
	return snapshot, nil
}

func IsSelfVerificationSummaryKind(kind string) bool {
	return kind == selfVerificationSummaryKind || kind == legacySelfAugmentSummaryKind
}

func WriteSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
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
