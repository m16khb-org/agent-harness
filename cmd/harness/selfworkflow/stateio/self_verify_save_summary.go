package stateio

import (
	"encoding/json"
	"time"

	"agent-harness/internal/adapter/core"
)

func SaveSelfVerificationSummary(result *SelfAugmentResult, key string) error {
	if key == "" {
		key = "self-verify-latest"
	}
	snapshot := NewSelfVerificationSummarySnapshot(*result, time.Now().UTC())
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return err
	}
	state, err := core.StateWrite(key, string(b))
	if err != nil {
		result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: core.StateDir(), Error: err.Error()}
		return err
	}
	result.StateCheckpoint = &SelfAugmentStateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	return nil
}

func SaveSelfAugmentSummary(result *SelfAugmentResult, key string) error {
	return SaveSelfVerificationSummary(result, key)
}

func NewSelfVerificationSummarySnapshot(result SelfAugmentResult, generatedAt time.Time) SelfAugmentStateSnapshot {
	return SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          selfVerificationSummaryKind,
		LoopKind:      result.LoopKind,
		KoreanName:    result.KoreanName,
		OK:            result.OK,
		Iterations:    result.Iterations,
		BaseSeed:      result.BaseSeed,
		TargetScore:   result.TargetScore,
		ElapsedMS:     result.ElapsedMS,
		HarnessRoot:   result.HarnessRoot,
		GeneratedAt:   generatedAt.Format(time.RFC3339Nano),
		Summary:       result.Summary,
	}
}
