package augmentplan

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"agent-harness/cmd/harness/selfworkflow/augmentcatalog"
	"agent-harness/cmd/harness/selfworkflow/model"
	install "agent-harness/internal/adapter/install"
)

type Request = model.SelfAugmentPlanRequest
type Result = model.SelfAugmentPlanResult

func Plan(req Request, root, version string) Result {
	geniusPath := filepath.Join(root, "GENIUS_THINK.md")
	geniusText := ""
	warnings := []string{}
	if b, err := os.ReadFile(geniusPath); err == nil {
		geniusText = string(b)
	} else {
		warnings = append(warnings, "GENIUS_THINK.md not found; augmentation can still run but loses the local genius-thinking heuristic")
	}
	docs := DocsIndex(root, version)
	skills, err := install.ListSkillNames(root)
	if err != nil {
		warnings = append(warnings, "list skills: "+err.Error())
	}
	signals := augmentcatalog.CollectSelfAugmentRepoSignals(root, len(docs.Docs), skills, geniusText)
	goals := []model.SelfAugmentGoal{
		{
			Name: "curriculum_selection", KoreanName: "개선 목표 선별", TargetScore: req.TargetScore,
			Score:       augmentcatalog.ScoreBool(signals.HasGeniusThink && docs.OK),
			Description: "GENIUS_THINK.md와 repo evidence로 10개 이상 후보를 만들고 가치·위험·실현 가능성을 수치화한다.",
			Evidence:    []string{"GENIUS_THINK.md", "docs index", "skill inventory"},
		},
		{
			Name: "implementation_delta", KoreanName: "개선 구현", TargetScore: req.TargetScore,
			Score:       augmentcatalog.ScoreBool(false),
			Description: "선택 후보를 실제 코드/문서/스킬 diff로 구현한다. 단순 보고서만으로는 통과하지 않는다.",
			Evidence:    []string{"git diff", "targeted implementation"},
		},
		{
			Name: "verification_qa", KoreanName: "검증·QA", TargetScore: req.TargetScore,
			Score:       augmentcatalog.ScoreBool(false),
			Description: "Targeted tests, QA smoke checks, and the self-verification loop must pass.",
			Evidence:    []string{"go test", "QA gate", "harness self-verify"},
		},
		{
			Name: "learning_capture", KoreanName: "학습 기록", TargetScore: req.TargetScore,
			Score:       augmentcatalog.ScoreBool(false),
			Description: "실패/성공 원인과 다음 개선점을 state/docs 중 적절한 위치에 남긴다.",
			Evidence:    []string{"harness state", ".agent-harness"},
		},
	}
	for i := range goals {
		goals[i].Passed = goals[i].Score > goals[i].TargetScore
	}
	candidates := augmentcatalog.SelfAugmentCandidates(signals)
	lessonCounts, lessonWarnings := severeLessonCounts()
	warnings = append(warnings, lessonWarnings...)
	warnings = append(warnings, applyLessonPenalties(candidates, lessonCounts)...)
	sort.Slice(candidates, func(i, j int) bool {
		leftOpen := candidates[i].Status == augmentcatalog.SelfAugmentCandidateStatusOpen
		rightOpen := candidates[j].Status == augmentcatalog.SelfAugmentCandidateStatusOpen
		if leftOpen != rightOpen {
			return leftOpen
		}
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].ID < candidates[j].ID
	})
	var selected *model.SelfAugmentCandidate
	for _, candidate := range candidates {
		if candidate.Status == augmentcatalog.SelfAugmentCandidateStatusOpen {
			copyCandidate := candidate
			selected = &copyCandidate
			break
		}
	}
	return Result{
		OK:                  true,
		LoopKind:            "self_augmentation",
		KoreanName:          model.SelfAugmentationKoreanName,
		Cycles:              req.Cycles,
		TargetScore:         req.TargetScore,
		TerminationEligible: augmentcatalog.AllSelfAugmentGoalsPassed(goals),
		HarnessRoot:         root,
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		GeniusThinkPath:     geniusPath,
		UsesGeniusThink:     signals.HasGeniusThink,
		SelectedFormulas:    augmentcatalog.SelectGeniusFormulas(geniusText),
		ResearchInfluences:  augmentcatalog.SelfAugmentResearchInfluences(),
		Goals:               goals,
		Candidates:          candidates,
		SelectedCandidate:   selected,
		ExecutionProtocol: []string{
			"baseline: run the self-verification loop and collect goal scores",
			"curriculum: generate at least 10 improvement candidates using GENIUS_THINK.md formulas and repo evidence",
			"selection: choose the highest score safe candidate, not the easiest cosmetic change",
			"implementation: edit code/docs/skills with small reversible diffs",
			"feedback: convert failures into Reflexion-style verbal lessons and retry within the cycle budget",
			"termination: stop only when every self-augmentation goal and self-verification goal scores above target_score",
		},
		VerificationGate: []string{
			"targeted tests for changed behavior",
			"QA gate including docs, skills, native integration, and MCP/state smoke",
			"go test ./... -count=1",
			"go test -race ./... -count=1 when touched Go behavior is concurrency or policy sensitive",
			"./bin/agent-harness self-verify --target-score=95 --json",
		},
		Warnings:    warnings,
		RepoSignals: signals,
	}
}
