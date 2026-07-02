package augmentlesson

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/core"
)

type Deps struct {
	HarnessRoot     func() string
	PrintJSON       func(any) error
	SelectCandidate func() *model.SelfAugmentCandidate
}

func SaveSelfAugmentLesson(req model.SelfAugmentLessonRequest, deps Deps) (model.SelfAugmentLessonResult, error) {
	root := "."
	if deps.HarnessRoot != nil {
		root = deps.HarnessRoot()
	}
	candidateID := strings.TrimSpace(req.CandidateID)
	if candidateID == "" && deps.SelectCandidate != nil {
		if candidate := deps.SelectCandidate(); candidate != nil {
			candidateID = candidate.ID
		}
	}
	if candidateID == "" {
		return model.SelfAugmentLessonResult{OK: false, Kind: model.SelfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: model.SelfAugmentationKoreanName, HarnessRoot: root}, fmt.Errorf("candidate is required when no selected candidate is available")
	}
	lesson := strings.TrimSpace(req.Lesson)
	if lesson == "" {
		return model.SelfAugmentLessonResult{OK: false, Kind: model.SelfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: model.SelfAugmentationKoreanName, CandidateID: candidateID, HarnessRoot: root}, fmt.Errorf("lesson is required")
	}
	nextAction := strings.TrimSpace(req.NextAction)
	if nextAction == "" {
		return model.SelfAugmentLessonResult{OK: false, Kind: model.SelfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: model.SelfAugmentationKoreanName, CandidateID: candidateID, Lesson: lesson, HarnessRoot: root}, fmt.Errorf("next-action is required")
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
	result := model.SelfAugmentLessonResult{
		OK:          true,
		Kind:        model.SelfAugmentationLessonKind,
		LoopKind:    "self_augmentation",
		KoreanName:  model.SelfAugmentationKoreanName,
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
		// Nanosecond suffix: second-granularity keys collide when lessons are
		// recorded back-to-back in one loop, silently dropping earlier lessons.
		key = fmt.Sprintf("self-augment-lesson-%s-%s-%09d", StateKeySlug(candidateID), now.Format("20060102T150405Z"), now.Nanosecond())
	}
	snapshot := model.SelfAugmentLessonStateSnapshot{
		SchemaVersion: 1,
		Kind:          model.SelfAugmentationLessonKind,
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
		result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return result, err
	}
	state, err := core.StateWrite(key, string(b))
	if err != nil {
		result.OK = false
		result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: core.StateDir(), Error: err.Error()}
		return result, err
	}
	result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	return result, nil
}

func StateKeySlug(s string) string {
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
