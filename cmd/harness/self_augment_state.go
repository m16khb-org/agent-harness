package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core"
)

func saveSelfAugmentLesson(req SelfAugmentLessonRequest) (SelfAugmentLessonResult, error) {
	root := harnessRoot()
	candidateID := strings.TrimSpace(req.CandidateID)
	if candidateID == "" {
		plan := planSelfAugmentation(SelfAugmentPlanRequest{Cycles: 1, TargetScore: defaultLoopTargetScoreExclusive})
		if plan.SelectedCandidate != nil {
			candidateID = plan.SelectedCandidate.ID
		}
	}
	if candidateID == "" {
		return SelfAugmentLessonResult{OK: false, Kind: selfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: selfAugmentationKoreanName, HarnessRoot: root}, fmt.Errorf("candidate is required when no selected candidate is available")
	}
	lesson := strings.TrimSpace(req.Lesson)
	if lesson == "" {
		return SelfAugmentLessonResult{OK: false, Kind: selfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: selfAugmentationKoreanName, CandidateID: candidateID, HarnessRoot: root}, fmt.Errorf("lesson is required")
	}
	nextAction := strings.TrimSpace(req.NextAction)
	if nextAction == "" {
		return SelfAugmentLessonResult{OK: false, Kind: selfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: selfAugmentationKoreanName, CandidateID: candidateID, Lesson: lesson, HarnessRoot: root}, fmt.Errorf("next-action is required")
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "self-augment"
	}
	severity := strings.TrimSpace(req.Severity)
	if severity == "" {
		severity = "info"
	}
	now := time.Now().UTC()
	generatedAt := now.Format(time.RFC3339Nano)
	result := SelfAugmentLessonResult{
		OK:          true,
		Kind:        selfAugmentationLessonKind,
		LoopKind:    "self_augmentation",
		KoreanName:  selfAugmentationKoreanName,
		CandidateID: candidateID,
		Lesson:      lesson,
		NextAction:  nextAction,
		Source:      source,
		Severity:    severity,
		HarnessRoot: root,
		GeneratedAt: generatedAt,
	}
	key := strings.TrimSpace(req.StateKey)
	if key == "" {
		key = fmt.Sprintf("self-augment-lesson-%s-%s", stateKeySlug(candidateID), now.Format("20060102T150405Z"))
	}
	snapshot := SelfAugmentLessonStateSnapshot{
		SchemaVersion: 1,
		Kind:          selfAugmentationLessonKind,
		LoopKind:      result.LoopKind,
		KoreanName:    result.KoreanName,
		OK:            result.OK,
		CandidateID:   result.CandidateID,
		Lesson:        result.Lesson,
		NextAction:    result.NextAction,
		Source:        result.Source,
		Severity:      result.Severity,
		HarnessRoot:   result.HarnessRoot,
		GeneratedAt:   result.GeneratedAt,
	}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		result.OK = false
		result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return result, err
	}
	state, err := core.StateWrite(key, string(b))
	if err != nil {
		result.OK = false
		result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: core.StateDir(), Error: err.Error()}
		return result, err
	}
	result.StateCheckpoint = &SelfAugmentStateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	return result, nil
}

func stateKeySlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "lesson"
	}
	return out
}

func saveSelfAugmentPlan(result *SelfAugmentPlanResult, key string) error {
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
		OpenCandidateIDs:      selfAugmentCandidateIDsByStatus(result.Candidates, selfAugmentCandidateStatusOpen),
		SatisfiedCandidateIDs: selfAugmentCandidateIDsByStatus(result.Candidates, selfAugmentCandidateStatusSatisfied),
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

func selfAugmentCandidateIDsByStatus(candidates []SelfAugmentCandidate, status string) []string {
	ids := []string{}
	for _, candidate := range candidates {
		if candidate.Status == status {
			ids = append(ids, candidate.ID)
		}
	}
	return ids
}
