package stateio

import (
	"encoding/json"
	"time"

	"agent-harness/internal/adapter/core"
)

func SaveSelfAugmentPlan(result *SelfAugmentPlanResult, key string) error {
	if key == "" {
		key = "self-augment-latest"
	}
	snapshot := SelfAugmentPlanStateSnapshot{
		SchemaVersion:         1,
		Kind:                  selfAugmentationPlanKind,
		LoopKind:              result.LoopKind,
		KoreanName:            result.KoreanName,
		OK:                    result.OK,
		Cycles:                result.Cycles,
		TargetScore:           result.TargetScore,
		HarnessRoot:           result.HarnessRoot,
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		SelectedCandidate:     result.SelectedCandidate,
		CandidateCount:        len(result.Candidates),
		OpenCandidateIDs:      SelfAugmentCandidateIDsByStatus(result.Candidates, selfAugmentCandidateStatusOpen),
		SatisfiedCandidateIDs: SelfAugmentCandidateIDsByStatus(result.Candidates, selfAugmentCandidateStatusSatisfied),
		Goals:                 result.Goals,
		SelectedFormulas:      result.SelectedFormulas,
		ResearchInfluences:    result.ResearchInfluences,
	}
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

func SelfAugmentCandidateIDsByStatus(candidates []SelfAugmentCandidate, status string) []string {
	ids := []string{}
	for _, candidate := range candidates {
		if candidate.Status == status {
			ids = append(ids, candidate.ID)
		}
	}
	return ids
}
