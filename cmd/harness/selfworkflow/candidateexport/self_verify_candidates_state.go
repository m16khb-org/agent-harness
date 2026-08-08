package candidateexport

import (
	"encoding/json"
	"time"
)

func SaveSelfVerificationCandidateExport(result *SelfVerificationCandidateExportResult, key string) error {
	if key == "" {
		key = "self-verify-candidates-latest"
	}
	snapshot := SelfVerificationCandidateExportStateSnapshot{
		SchemaVersion:         1,
		Kind:                  SelfVerificationCandidateExportKind,
		LoopKind:              result.LoopKind,
		KoreanName:            result.KoreanName,
		OK:                    result.OK,
		HarnessRoot:           result.HarnessRoot,
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		SourcePath:            result.SourcePath,
		CandidateCount:        result.CandidateCount,
		OpenCandidateIDs:      result.OpenCandidateIDs,
		SatisfiedCandidateIDs: result.SatisfiedCandidateIDs,
		SelectedCandidate:     result.SelectedCandidate,
		Candidates:            result.Candidates,
	}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return err
	}
	state, err := StateWrite(key, string(b))
	if err != nil {
		result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: StateDir(), Error: err.Error()}
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
