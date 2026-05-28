package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core"
)

type SelfAugmentPlanRequest struct {
	Cycles      int     `json:"cycles"`
	TargetScore float64 `json:"target_score"`
}

const (
	selfAugmentationPlanKind   = "self_augmentation_plan"
	selfAugmentationLessonKind = "self_augmentation_lesson"
)

type SelfAugmentPlanResult struct {
	OK                  bool                        `json:"ok"`
	LoopKind            string                      `json:"loop_kind"`
	KoreanName          string                      `json:"korean_name"`
	Cycles              int                         `json:"cycles"`
	TargetScore         float64                     `json:"target_score"`
	TerminationEligible bool                        `json:"termination_eligible"`
	HarnessRoot         string                      `json:"harness_root"`
	GeneratedAt         string                      `json:"generated_at"`
	GeniusThinkPath     string                      `json:"genius_think_path"`
	UsesGeniusThink     bool                        `json:"uses_genius_think"`
	SelectedFormulas    []string                    `json:"selected_formulas"`
	ResearchInfluences  []SelfAugmentInfluence      `json:"research_influences"`
	Goals               []SelfAugmentGoal           `json:"goals"`
	Candidates          []SelfAugmentCandidate      `json:"candidates"`
	SelectedCandidate   *SelfAugmentCandidate       `json:"selected_candidate,omitempty"`
	ExecutionProtocol   []string                    `json:"execution_protocol"`
	VerificationGate    []string                    `json:"verification_gate"`
	Warnings            []string                    `json:"warnings"`
	RepoSignals         SelfAugmentRepoSignals      `json:"repo_signals"`
	StateCheckpoint     *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
}

type SelfAugmentInfluence struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Adopted string `json:"adopted"`
}

type SelfAugmentGoal struct {
	Name        string   `json:"name"`
	KoreanName  string   `json:"korean_name"`
	Score       float64  `json:"score"`
	TargetScore float64  `json:"target_score"`
	Passed      bool     `json:"passed"`
	Description string   `json:"description"`
	Evidence    []string `json:"evidence"`
}

type SelfAugmentCandidate struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Category             string   `json:"category"`
	Status               string   `json:"status"`
	Score                float64  `json:"score"`
	Impact               float64  `json:"impact"`
	Feasibility          float64  `json:"feasibility"`
	Novelty              float64  `json:"novelty"`
	Risk                 float64  `json:"risk"`
	WhyNow               []string `json:"why_now"`
	ExpectedGain         []string `json:"expected_gain"`
	VerifyWith           []string `json:"verify_with"`
	SatisfactionEvidence []string `json:"satisfaction_evidence,omitempty"`
}

type SelfAugmentRepoSignals struct {
	DocsIndexed                 int      `json:"docs_indexed"`
	Skills                      []string `json:"skills"`
	HasGeniusThink              bool     `json:"has_genius_think"`
	HasSelfAugmentSkill         bool     `json:"has_self_augment_skill"`
	HasSelfVerificationDocs     bool     `json:"has_self_verification_docs"`
	HasSelfVerifyCLI            bool     `json:"has_self_verify_cli"`
	HasSelfAugmentPlanner       bool     `json:"has_self_augment_planner"`
	HasSelfAugmentStateCapture  bool     `json:"has_self_augment_state_capture"`
	HasSelfAugmentLessonCapture bool     `json:"has_self_augment_lesson_capture"`
	HasAdapterContractMatrix    bool     `json:"has_adapter_contract_matrix"`
	HasRiskQATier               bool     `json:"has_risk_qa_tier"`
	HasGoalScoreSummary         bool     `json:"has_goal_score_summary"`
	HasRepoLocalSandbox         bool     `json:"has_repo_local_sandbox"`
	HasPerformanceBaseline      bool     `json:"has_performance_baseline"`
	HasGeniusMermaidLint        bool     `json:"has_genius_mermaid_lint"`
	HasInstallDryRunMode        bool     `json:"has_install_dry_run_mode"`
}

type SelfAugmentLessonRequest struct {
	CandidateID string `json:"candidate_id"`
	Lesson      string `json:"lesson"`
	NextAction  string `json:"next_action"`
	Source      string `json:"source"`
	Severity    string `json:"severity"`
	StateKey    string `json:"state_key"`
}

type SelfAugmentLessonResult struct {
	OK              bool                        `json:"ok"`
	Kind            string                      `json:"kind"`
	LoopKind        string                      `json:"loop_kind"`
	KoreanName      string                      `json:"korean_name"`
	CandidateID     string                      `json:"candidate_id"`
	Lesson          string                      `json:"lesson"`
	NextAction      string                      `json:"next_action"`
	Source          string                      `json:"source"`
	Severity        string                      `json:"severity"`
	HarnessRoot     string                      `json:"harness_root"`
	GeneratedAt     string                      `json:"generated_at"`
	StateCheckpoint *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
}

type SelfAugmentLessonStateSnapshot struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	LoopKind      string `json:"loop_kind"`
	KoreanName    string `json:"korean_name"`
	OK            bool   `json:"ok"`
	CandidateID   string `json:"candidate_id"`
	Lesson        string `json:"lesson"`
	NextAction    string `json:"next_action"`
	Source        string `json:"source"`
	Severity      string `json:"severity"`
	HarnessRoot   string `json:"harness_root"`
	GeneratedAt   string `json:"generated_at"`
}

type SelfAugmentPlanStateSnapshot struct {
	SchemaVersion         int                    `json:"schema_version"`
	Kind                  string                 `json:"kind"`
	LoopKind              string                 `json:"loop_kind"`
	KoreanName            string                 `json:"korean_name"`
	OK                    bool                   `json:"ok"`
	Cycles                int                    `json:"cycles"`
	TargetScore           float64                `json:"target_score"`
	HarnessRoot           string                 `json:"harness_root"`
	GeneratedAt           string                 `json:"generated_at"`
	SelectedCandidate     *SelfAugmentCandidate  `json:"selected_candidate,omitempty"`
	CandidateCount        int                    `json:"candidate_count"`
	OpenCandidateIDs      []string               `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string               `json:"satisfied_candidate_ids"`
	Goals                 []SelfAugmentGoal      `json:"goals"`
	SelectedFormulas      []string               `json:"selected_formulas"`
	ResearchInfluences    []SelfAugmentInfluence `json:"research_influences"`
}

func runSelfAugment(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "lesson":
			return runSelfAugmentLesson(args[1:])
		case "verify":
			return runSelfVerify(args[1:])
		case "history", "compare", "promote":
			return runSelfVerify(args)
		}
	}
	fs := flag.NewFlagSet("self-augment", flag.ContinueOnError)
	cycles := fs.Int("cycles", 1, "number of autonomous improvement cycles to plan")
	targetScore := fs.Float64("target-score", defaultLoopTargetScoreExclusive, "exclusive per-goal score threshold; every concrete goal must score above this value before termination")
	saveState := fs.Bool("save-state", false, "save compact self-augmentation plan to harness state")
	stateKey := fs.String("state-key", "self-augment-latest", "state key for --save-state")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cycles < 1 {
		return fmt.Errorf("cycles must be positive")
	}
	if *targetScore < 0 || *targetScore >= 100 {
		return fmt.Errorf("target-score must be >= 0 and < 100")
	}
	result := planSelfAugmentation(SelfAugmentPlanRequest{Cycles: *cycles, TargetScore: *targetScore})
	if *saveState {
		if err := saveSelfAugmentPlan(&result, *stateKey); err != nil {
			return err
		}
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("%s plan: %d candidate(s), selected=%s, termination_eligible=%v\n", result.KoreanName, len(result.Candidates), selectedCandidateID(result.SelectedCandidate), result.TerminationEligible)
	for _, goal := range result.Goals {
		fmt.Printf("- %s score=%.1f target>%.1f passed=%v\n", goal.KoreanName, goal.Score, goal.TargetScore, goal.Passed)
	}
	if result.SelectedCandidate != nil {
		fmt.Printf("selected: %s — %s\n", result.SelectedCandidate.ID, result.SelectedCandidate.Title)
	}
	return nil
}

func planSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	root := harnessRoot()
	geniusPath := filepath.Join(root, "GENIUS_THINK.md")
	geniusText := ""
	warnings := []string{}
	if b, err := os.ReadFile(geniusPath); err == nil {
		geniusText = string(b)
	} else {
		warnings = append(warnings, "GENIUS_THINK.md not found; augmentation can still run but loses the local genius-thinking heuristic")
	}
	docs := core.DocsIndex(root, version)
	skills, err := core.ListSkillNames(root)
	if err != nil {
		warnings = append(warnings, "list skills: "+err.Error())
	}
	signals := SelfAugmentRepoSignals{
		DocsIndexed:                 len(docs.Docs),
		Skills:                      append([]string{}, skills...),
		HasGeniusThink:              strings.TrimSpace(geniusText) != "",
		HasSelfAugmentSkill:         containsString(skills, "self-augment"),
		HasSelfVerificationDocs:     docsContainTerm(root, "Self-verification") && docsContainTerm(root, "Self-augmentation"),
		HasSelfVerifyCLI:            fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), `case "self-verify"`) && fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), "selfVerificationKoreanName"),
		HasSelfAugmentPlanner:       fileContainsTerm(root, filepath.Join("cmd", "harness", "self_augment_loop.go"), "planSelfAugmentation"),
		HasSelfAugmentStateCapture:  fileContainsTerm(root, filepath.Join("cmd", "harness", "self_augment_loop.go"), "saveSelfAugmentPlan") && docsContainTerm(root, "--save-state"),
		HasSelfAugmentLessonCapture: fileContainsTerm(root, filepath.Join("cmd", "harness", "self_augment_loop.go"), "saveSelfAugmentLesson"),
		HasAdapterContractMatrix:    fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallAdapterContractMatrix") && fileContainsTerm(root, filepath.Join("internal", "adapter", "testdata", "native_install_contract_matrix.golden.json"), "project-local-opt-in"),
		HasRiskQATier:               fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), "validateRiskQATier") && fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), "risk_qa"),
		HasGoalScoreSummary:         fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), "GoalScores") && fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), "MinimumGoalScore"),
		HasRepoLocalSandbox:         fileContainsTerm(root, filepath.Join("internal", "core", "policy.go"), "path_outside_workspace") && fileContainsTerm(root, filepath.Join("internal", "core", "policy_test.go"), "TestCommandPolicyDeniesPathArgsOutsideWorkspace") && fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), "policy deny outside path arg"),
		HasPerformanceBaseline:      fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), "SlowStepRegressions") && fileContainsTerm(root, filepath.Join("cmd", "harness", "self_augment_summary_test.go"), "TestCompareSelfAugmentSummariesDetectsSlowStepRegression") && docsContainTerm(root, "slow_step:*"),
		HasGeniusMermaidLint:        fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), "lintMermaidBlocks") && fileContainsTerm(root, filepath.Join("cmd", "harness", "self_augment_summary_test.go"), "TestLintMermaidBlocksEnforcesGeniusThinkRules") && !fileContainsTerm(root, filepath.Join(".agent-harness", "ARCHITECTURE.md"), `\n`),
		HasInstallDryRunMode:        fileContainsTerm(root, filepath.Join("cmd", "harness", "install_native.go"), "dry-run") && fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallDryRunDoesNotWrite") && docsContainTerm(root, "install-native --dry-run"),
	}
	goals := []SelfAugmentGoal{
		{
			Name: "curriculum_selection", KoreanName: "개선 목표 선별", TargetScore: req.TargetScore,
			Score:       scoreBool(signals.HasGeniusThink && docs.OK),
			Description: "GENIUS_THINK.md와 repo evidence로 10개 이상 후보를 만들고 가치·위험·실현 가능성을 수치화한다.",
			Evidence:    []string{"GENIUS_THINK.md", "docs index", "skill inventory"},
		},
		{
			Name: "implementation_delta", KoreanName: "개선 구현", TargetScore: req.TargetScore,
			Score:       scoreBool(false),
			Description: "선택 후보를 실제 코드/문서/스킬 diff로 구현한다. 단순 보고서만으로는 통과하지 않는다.",
			Evidence:    []string{"git diff", "targeted implementation"},
		},
		{
			Name: "verification_qa", KoreanName: "검증·QA", TargetScore: req.TargetScore,
			Score:       scoreBool(false),
			Description: "Targeted tests, QA smoke checks, and the self-verification loop must pass.",
			Evidence:    []string{"go test", "QA gate", "harness self-verify"},
		},
		{
			Name: "learning_capture", KoreanName: "학습 기록", TargetScore: req.TargetScore,
			Score:       scoreBool(false),
			Description: "실패/성공 원인과 다음 개선점을 state/docs 중 적절한 위치에 남긴다.",
			Evidence:    []string{"harness state", ".agent-harness"},
		},
	}
	for i := range goals {
		goals[i].Passed = goals[i].Score > goals[i].TargetScore
	}
	candidates := selfAugmentCandidates(signals)
	sort.Slice(candidates, func(i, j int) bool {
		leftOpen := candidates[i].Status == selfAugmentCandidateStatusOpen
		rightOpen := candidates[j].Status == selfAugmentCandidateStatusOpen
		if leftOpen != rightOpen {
			return leftOpen
		}
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].ID < candidates[j].ID
	})
	var selected *SelfAugmentCandidate
	for _, candidate := range candidates {
		if candidate.Status == selfAugmentCandidateStatusOpen {
			copyCandidate := candidate
			selected = &copyCandidate
			break
		}
	}
	return SelfAugmentPlanResult{
		OK:                  true,
		LoopKind:            "self_augmentation",
		KoreanName:          selfAugmentationKoreanName,
		Cycles:              req.Cycles,
		TargetScore:         req.TargetScore,
		TerminationEligible: allSelfAugmentGoalsPassed(goals),
		HarnessRoot:         root,
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		GeniusThinkPath:     geniusPath,
		UsesGeniusThink:     signals.HasGeniusThink,
		SelectedFormulas:    selectGeniusFormulas(geniusText),
		ResearchInfluences:  selfAugmentResearchInfluences(),
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
			"./bin/harness self-verify --iterations=10 --target-score=95 --json",
		},
		Warnings:    warnings,
		RepoSignals: signals,
	}
}

func runSelfAugmentLesson(args []string) error {
	fs := flag.NewFlagSet("self-augment lesson", flag.ContinueOnError)
	candidateID := fs.String("candidate", "", "candidate id this lesson belongs to; defaults to the current selected open candidate")
	lesson := fs.String("lesson", "", "Reflexion-style lesson learned from a failure, QA issue, or design concern")
	nextAction := fs.String("next-action", "", "specific next action that should use this lesson")
	source := fs.String("source", "self-augment", "source that produced the lesson")
	severity := fs.String("severity", "info", "lesson severity: info, warning, or error")
	stateKey := fs.String("state-key", "", "state key; defaults to self-augment-lesson-<candidate>-<timestamp>")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := saveSelfAugmentLesson(SelfAugmentLessonRequest{
		CandidateID: *candidateID,
		Lesson:      *lesson,
		NextAction:  *nextAction,
		Source:      *source,
		Severity:    *severity,
		StateKey:    *stateKey,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("self-augment lesson saved: candidate=%s key=%s\n", result.CandidateID, result.StateCheckpoint.Key)
	return nil
}

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

func scoreBool(ok bool) float64 {
	if ok {
		return 100
	}
	return 0
}

func allSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool {
	if len(goals) == 0 {
		return false
	}
	for _, goal := range goals {
		if !goal.Passed {
			return false
		}
	}
	return true
}

func selectedCandidateID(candidate *SelfAugmentCandidate) string {
	if candidate == nil {
		return ""
	}
	return candidate.ID
}

func docsContainTerm(root, term string) bool {
	for _, path := range core.ListDocs(root) {
		b, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(b), term) {
			return true
		}
	}
	return false
}

func fileContainsTerm(root, relPath, term string) bool {
	b, err := os.ReadFile(filepath.Join(root, relPath))
	return err == nil && strings.Contains(string(b), term)
}

func selectGeniusFormulas(text string) []string {
	if strings.TrimSpace(text) == "" {
		return []string{}
	}
	formulas := []string{
		"문제 재정의 알고리즘",
		"혁신적 솔루션 생성 공식",
		"사고의 진화 방정식",
		"복잡성 해결 매트릭스",
	}
	selected := []string{}
	for _, formula := range formulas {
		if strings.Contains(text, formula) {
			selected = append(selected, formula)
		}
	}
	return selected
}

func selfAugmentResearchInfluences() []SelfAugmentInfluence {
	return []SelfAugmentInfluence{
		{Name: "Reflexion", Source: "https://arxiv.org/abs/2303.11366", Adopted: "scalar/test feedback is converted into reusable verbal lessons between cycles"},
		{Name: "Self-Refine", Source: "https://arxiv.org/abs/2303.17651", Adopted: "generate-feedback-refine is used inside candidate design and implementation retries"},
		{Name: "Voyager", Source: "https://arxiv.org/abs/2305.16291", Adopted: "automatic curriculum and skill-library thinking guide open-ended improvement selection"},
		{Name: "SWE-agent", Source: "https://arxiv.org/abs/2405.15793", Adopted: "agent-computer interface discipline: repository navigation, file edits, and tests are explicit loop surfaces"},
		{Name: "AgentBench", Source: "https://arxiv.org/abs/2308.03688", Adopted: "multi-dimensional goal scoring replaces vague done/not-done loop exits"},
		{Name: "SWE-bench", Source: "https://arxiv.org/abs/2310.06770", Adopted: "real repository issue resolution is the model for improvement candidates"},
		{Name: "LangGraph", Source: "https://docs.langchain.com/oss/python/langgraph/overview", Adopted: "durable state, human oversight, and recovery are kept as design constraints"},
		{Name: "AutoGen", Source: "https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/human-in-the-loop.html", Adopted: "termination conditions and max-turn safeguards inspire explicit score gates"},
		{Name: "DSPy optimizers", Source: "https://github.com/stanfordnlp/dspy/blob/main/docs/docs/learn/optimization/optimizers.md", Adopted: "metric-first optimization shapes candidate scoring and regression checks"},
		{Name: "OpenAI Evals", Source: "https://github.com/openai/evals", Adopted: "evaluation artifacts must be reusable and rights-safe before promotion"},
	}
}

const (
	selfAugmentCandidateStatusOpen      = "open"
	selfAugmentCandidateStatusSatisfied = "already_satisfied"
)

func selfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	base := []SelfAugmentCandidate{
		{
			ID: "loop-taxonomy-score-gates", Title: "Separate self-verification and self-augmentation loops and enforce >95 exit gates", Category: "quality",
			Impact: 99, Feasibility: 98, Novelty: 93, Risk: 8,
			WhyNow:       []string{"현재 self-augment가 실제로는 검증 루프 역할을 한다", "사용자가 테스트와 QA 포함 및 95점 게이트를 요구했다"},
			ExpectedGain: []string{"루프 이름/책임 혼동 제거", "검증 없는 종료 방지", "CLI/MCP/native skill 계약 일치"},
			VerifyWith:   []string{"go test ./...", "MCP/CLI golden", "harness self-verify --iterations=10 --target-score=95"},
		},
		{
			ID: "agent-skill-executor", Title: "Provide the self-augmentation loop as a native skill executor that creates real improvement diffs", Category: "feature",
			Impact: 97, Feasibility: 96, Novelty: 92, Risk: 12,
			WhyNow:       []string{"Go CLI는 LLM 판단/코드 편집 주체가 아니므로 skill 표면이 필요하다", "Codex/Claude 공용 하네스 목적과 맞다"},
			ExpectedGain: []string{"후보화→선택→구현→검증을 agent가 실제 수행", "GENIUS_THINK.md와 연구 앵커를 반복적으로 활용"},
			VerifyWith:   []string{"skill quick_validate", "install-native", "self-verify QA gate"},
		},
		{
			ID: "durable-augmentation-memory", Title: "Accumulate self-augmentation candidates, decisions, and failure lessons in state", Category: "memory",
			Impact: 93, Feasibility: 92, Novelty: 86, Risk: 20,
			WhyNow:       []string{"Reflexion식 언어 피드백을 다음 실행에 재사용하려면 durable memory가 필요하다"},
			ExpectedGain: []string{"동일 실패 반복 감소", "레포별 개선 히스토리 누적"},
			VerifyWith:   []string{"state roundtrip", "history/compare contract"},
		},
		{
			ID: "qa-dashboard-summary", Title: "Expand self-verification summaries into QA dashboard scorecards", Category: "observability",
			Impact: 91, Feasibility: 95, Novelty: 72, Risk: 10,
			WhyNow:       []string{"느린 step과 실패 위치만으로는 목표별 종료 판단이 부족하다"},
			ExpectedGain: []string{"목표별 점수와 증거 label을 바로 확인", "CI/agent가 gate를 기계적으로 판정"},
			VerifyWith:   []string{"response_contract golden", "self-verify --json schema inspection"},
		},
		{
			ID: "reflexion-state-memory", Title: "Store self-augmentation failure lessons in state", Category: "memory",
			Impact: 89, Feasibility: 91, Novelty: 84, Risk: 16,
			WhyNow:       []string{"반복 실패를 다음 cycle에서 활용하려면 언어 피드백 저장소가 필요하다"},
			ExpectedGain: []string{"실패 원인 재발 감소", "레포별 개선 이력 검색 가능"},
			VerifyWith:   []string{"state_write/read"},
		},
		{
			ID: "qa-race-tier", Title: "Conditionally attach risk-based race/static QA tier to the self-verification loop", Category: "qa",
			Impact: 88, Feasibility: 88, Novelty: 79, Risk: 22,
			WhyNow:       []string{"모든 반복에서 race를 돌리면 과하지만 concurrency/policy 변경에는 필요하다"},
			ExpectedGain: []string{"검증 비용과 신뢰도 균형", "고위험 변경에 대한 추가 방어"},
			VerifyWith:   []string{"go test -race ./... -count=1", "targeted package tests"},
		},
		{
			ID: "adapter-contract-matrix", Title: "Codex/Claude adapter 계약을 matrix fixture로 고정", Category: "test",
			Impact: 87, Feasibility: 90, Novelty: 75, Risk: 14,
			WhyNow:       []string{"설치 방식과 host adapter가 늘면서 core/adapter 계약 회귀 가능성이 커졌다"},
			ExpectedGain: []string{"SOLID 구조의 포트 계약 보존", "host별 출력 drift 조기 탐지"},
			VerifyWith:   []string{"internal/core install tests", "adapter golden fixtures"},
		},
		{
			ID: "install-dry-run-mode", Title: "install-native에 dry-run planning mode 추가", Category: "safety",
			Impact: 85, Feasibility: 86, Novelty: 70, Risk: 18,
			WhyNow:       []string{"user/global 설치는 안전하지만 쓰기 전 preview가 있으면 신뢰도가 오른다"},
			ExpectedGain: []string{"destructive concern 감소", "다른 레포 설치 전 diff 예측"},
			VerifyWith:   []string{"install tests", "temporary HOME smoke"},
		},
		{
			ID: "genius-mermaid-lint", Title: "GENIUS_THINK Mermaid 규칙을 docs QA lint로 승격", Category: "docs",
			Impact: 78, Feasibility: 94, Novelty: 67, Risk: 8,
			WhyNow:       []string{"문서 생성 시 Mermaid 파싱 오류를 반복 방지할 수 있다"},
			ExpectedGain: []string{"문서 품질 안정화", "한글 Mermaid 노드 규칙 자동 확인"},
			VerifyWith:   []string{"docs QA gate", "markdown fixture lint"},
		},
		{
			ID: "performance-baseline", Title: "self-verify slowest_steps를 성능 baseline regression으로 승격", Category: "performance",
			Impact: 83, Feasibility: 87, Novelty: 74, Risk: 16,
			WhyNow:       []string{"현재 elapsed regression은 summary 단위라 병목 위치 추적이 약하다"},
			ExpectedGain: []string{"느린 step regression 탐지", "성능 개선 후보 자동 생성"},
			VerifyWith:   []string{"self-verify compare fixtures", "slowest_steps golden"},
		},
		{
			ID: "repo-local-augmentation-sandbox", Title: "Harden workspace-boundary sandboxing for self-augmenting other repositories", Category: "safety",
			Impact: 90, Feasibility: 82, Novelty: 83, Risk: 24,
			WhyNow:       []string{"하네스가 여러 레포에서 쓰이면 repo별 권한·상태 경계가 핵심이다"},
			ExpectedGain: []string{"레포별 독립 self-augment 실행", "root 밖 접근 회귀 방지"},
			VerifyWith:   []string{"command_policy_check", "temp repo integration tests"},
		},
	}
	for i := range base {
		base[i].Status = selfAugmentCandidateStatusOpen
		base[i].Score = selfAugmentCandidateScore(base[i])
		markSatisfiedSelfAugmentCandidate(&base[i], signals)
	}
	return base
}

func markSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	var evidence []string
	switch candidate.ID {
	case "loop-taxonomy-score-gates":
		if signals.HasSelfVerifyCLI && signals.HasSelfAugmentPlanner && signals.HasSelfVerificationDocs && signals.HasGoalScoreSummary {
			evidence = []string{"self-verify CLI exists", "self-augment planner exists", "loop docs distinguish both Korean names", "goal score summary is implemented"}
		}
	case "agent-skill-executor":
		if signals.HasSelfAugmentSkill {
			evidence = []string{"skills/self-augment exists in shared skill inventory"}
		}
	case "durable-augmentation-memory":
		if signals.HasSelfAugmentStateCapture {
			evidence = []string{"self-augment --save-state persists selected candidate curriculum to harness state"}
		}
	case "reflexion-state-memory":
		if signals.HasSelfAugmentLessonCapture {
			evidence = []string{"self-augment lesson stores Reflexion lessons in harness state"}
		}
	case "qa-dashboard-summary":
		if signals.HasGoalScoreSummary {
			evidence = []string{"self-verify summary includes goal_scores and minimum_goal_score"}
		}
	case "qa-race-tier":
		if signals.HasRiskQATier {
			evidence = []string{"self-verify includes a risk QA tier that conditionally runs go test -race and go vet for sensitive Go changes"}
		}
	case "adapter-contract-matrix":
		if signals.HasAdapterContractMatrix {
			evidence = []string{"internal/adapter install contract matrix locks Codex/Claude user-global and project-local installation behavior with a golden fixture"}
		}
	case "repo-local-augmentation-sandbox":
		if signals.HasRepoLocalSandbox {
			evidence = []string{"command policy rejects path-like argv outside workspace_root and self-verify command policy smoke covers the boundary"}
		}
	case "performance-baseline":
		if signals.HasPerformanceBaseline {
			evidence = []string{"self-verify compare promotes label-level slowest_steps deltas into slow_step regressions with unit coverage"}
		}
	case "genius-mermaid-lint":
		if signals.HasGeniusMermaidLint {
			evidence = []string{"QA gate lints Mermaid fences using GENIUS_THINK quote/<br/> rules and repo diagrams were normalized"}
		}
	case "install-dry-run-mode":
		if signals.HasInstallDryRunMode {
			evidence = []string{"install-native supports --dry-run planning with no filesystem writes and adapter-level coverage"}
		}
	}
	if len(evidence) == 0 {
		return
	}
	candidate.Status = selfAugmentCandidateStatusSatisfied
	candidate.SatisfactionEvidence = evidence
	candidate.Score = 0
	candidate.WhyNow = append(candidate.WhyNow, "Already satisfied; do not select in the next self-augmentation cycle")
}

func selfAugmentCandidateScore(c SelfAugmentCandidate) float64 {
	score := c.Impact*0.38 + c.Feasibility*0.30 + c.Novelty*0.20 + (100-c.Risk)*0.12
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}
