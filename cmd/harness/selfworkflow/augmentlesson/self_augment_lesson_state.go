package augmentlesson

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-harness/cmd/harness/selfworkflow/model"
)

type Deps struct {
	HarnessRoot     func() string
	PrintJSON       func(any) error
	SelectCandidate func() *model.SelfAugmentCandidate
}

const (
	selfAugmentLessonStateKeyPrefix  = "self-augment-lesson-"
	selfAugmentLessonStateMaxAge     = 30 * 24 * time.Hour
	selfAugmentLessonStateMaxRecords = 10000
)

const selfAugmentLessonCandidateIDMaxLen = 96

// validateLessonCandidateID는 자유 dogfood 후보 슬러그를 포함한 lesson 후보
// 아이디를 검증한다. 커리큘럼 후보 아이디(kebab-case)와 같은 문자 규칙만
// 요구해 스키마 없는 state를 오염시키는 값을 막는다.
func validateLessonCandidateID(candidateID string) error {
	if len(candidateID) > selfAugmentLessonCandidateIDMaxLen {
		return fmt.Errorf("lesson candidate id must be at most %d characters", selfAugmentLessonCandidateIDMaxLen)
	}
	if strings.ContainsFunc(candidateID, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return false
		default:
			return true
		}
	}) {
		return fmt.Errorf("lesson candidate id must be kebab-case (letters, digits, hyphen): %q", candidateID)
	}
	return nil
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
		return model.SelfAugmentLessonResult{OK: false, Kind: model.SelfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: model.SelfAugmentationKoreanName, HarnessRoot: root}, fmt.Errorf("candidate is required: pass --candidate ID (an existing curriculum candidate or a slug for dogfooding findings such as issueops-whoami-record-flags) because no selected open candidate is available")
	}
	// 후보 아이디는 커리큘럼 후보 또는 자유 dogfood 슬러그를 허용한다. 플래너가
	// 모든 후보를 already_satisfied로 소진한 상태에서도 lesson 캡처 경로가
	// 막히면 안 된다(SELF_AUGMENTATION.md 학습 캡처 계약). 슬러그 규칙은
	// StateKeySlug와 같은 문자 규칙으로 검증한다.
	if err := validateLessonCandidateID(candidateID); err != nil {
		return model.SelfAugmentLessonResult{OK: false, Kind: model.SelfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: model.SelfAugmentationKoreanName, CandidateID: candidateID, HarnessRoot: root}, err
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
		key = fmt.Sprintf("%s%s-%s-%09d", selfAugmentLessonStateKeyPrefix, StateKeySlug(candidateID), now.Format("20060102T150405Z"), now.Nanosecond())
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
	state, err := StateWrite(key, string(b))
	if err != nil {
		result.OK = false
		result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: StateDir(), Error: err.Error()}
		return result, err
	}
	result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	_, _ = StatePrunePrefix(selfAugmentLessonStateKeyPrefix, selfAugmentLessonStateMaxAge, selfAugmentLessonStateMaxRecords, true)
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
