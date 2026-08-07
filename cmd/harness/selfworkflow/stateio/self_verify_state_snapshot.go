package stateio

import (
	"encoding/json"
	"fmt"
	"time"

	statecontract "agent-harness/internal/contract/state"

	"agent-harness/internal/core"
	"agent-harness/internal/domain/failurecause"
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
	if snapshot.Kind != selfVerificationSummaryKind {
		return SelfAugmentStateSnapshot{}, fmt.Errorf("state key %q contains kind %q, want %s", key, snapshot.Kind, selfVerificationSummaryKind)
	}
	if snapshot.SchemaVersion != 1 {
		return SelfAugmentStateSnapshot{}, fmt.Errorf("state key %q has unsupported self-verification summary schema %d", key, snapshot.SchemaVersion)
	}
	NormalizeSelfAugmentSnapshotFailureCause(&snapshot)
	return snapshot, nil
}

func IsSelfVerificationSummaryKind(kind string) bool {
	return kind == selfVerificationSummaryKind
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
	record := statecontract.RecordEnvelope{
		SchemaVersion: core.StateCurrentSchemaVersion,
		Key:           key,
		Content:       string(content),
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Bytes:         len(content),
	}
	// SA1: raw os.WriteFile 대신 lock + atomic(temp+rename) state writer로 저장해
	// core.writeStateRecord의 내구성과 맞춘다. 온디스크 출력은 경로·MarshalIndent·
	// trailing newline이 모두 같아 byte-identical하며, crash-safe하고 동시 writer에
	// 대해 직렬화된다.
	_, err = core.WriteStateRecord(dir, key, record)
	return err
}
