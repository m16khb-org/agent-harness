package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core"
)

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
		HasSelfVerifyCLI:            fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), `case "self-verify"`) && dirContainsTerm(root, filepath.Join("cmd", "harness"), "selfVerificationKoreanName"),
		HasSelfAugmentPlanner:       dirContainsTerm(root, filepath.Join("cmd", "harness"), "planSelfAugmentation"),
		HasSelfAugmentStateCapture:  dirContainsTerm(root, filepath.Join("cmd", "harness"), "saveSelfAugmentPlan") && docsContainTerm(root, "--save-state"),
		HasSelfAugmentLessonCapture: dirContainsTerm(root, filepath.Join("cmd", "harness"), "saveSelfAugmentLesson"),
		HasAdapterContractMatrix:    fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallAdapterContractMatrix") && fileContainsTerm(root, filepath.Join("internal", "adapter", "testdata", "native_install_contract_matrix.golden.json"), "project-local-opt-in"),
		HasRiskQATier:               dirContainsTerm(root, filepath.Join("cmd", "harness"), "validateRiskQATier") && dirContainsTerm(root, filepath.Join("cmd", "harness"), "risk_qa"),
		HasGoalScoreSummary:         dirContainsTerm(root, filepath.Join("cmd", "harness"), "GoalScores") && dirContainsTerm(root, filepath.Join("cmd", "harness"), "MinimumGoalScore"),
		HasRepoLocalSandbox:         fileContainsTerm(root, filepath.Join("internal", "core", "policy.go"), "path_outside_workspace") && fileContainsTerm(root, filepath.Join("internal", "core", "policy_test.go"), "TestCommandPolicyDeniesPathArgsOutsideWorkspace") && dirContainsTerm(root, filepath.Join("cmd", "harness"), "policy deny outside path arg"),
		HasPerformanceBaseline:      dirContainsTerm(root, filepath.Join("cmd", "harness"), "SlowStepRegressions") && fileContainsTerm(root, filepath.Join("cmd", "harness", "self_augment_compare_test.go"), "TestCompareSelfAugmentSummariesDetectsSlowStepRegression") && docsContainTerm(root, "slow_step:*"),
		HasGeniusMermaidLint:        dirContainsTerm(root, filepath.Join("cmd", "harness"), "lintMermaidBlocks") && fileContainsTerm(root, filepath.Join("cmd", "harness", "self_augment_history_test.go"), "TestLintMermaidBlocksEnforcesGeniusThinkRules") && !fileContainsTerm(root, filepath.Join(".agent-harness", "ARCHITECTURE.md"), `\n`),
		HasInstallDryRunMode:        fileContainsTerm(root, filepath.Join("cmd", "harness", "install_native.go"), "dry-run") && fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallDryRunDoesNotWrite") && docsContainTerm(root, "install-native --dry-run"),
		HasCLIAdapterSplit:          fileContainsTerm(root, filepath.Join("internal", "adapter", "cli", "usage.go"), "func Usage") && fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), "cliadapter.Usage"),
		HasMCPAdapterCatalog:        dirContainsTerm(root, filepath.Join("internal", "adapter", "mcp"), "AdapterOwnedTools") && dirContainsTerm(root, filepath.Join("cmd", "harness"), "mcpadapter.AdapterOwnedTools"),
		HasCompatibilityContract:    fileContainsTerm(root, filepath.Join("cmd", "harness", "contract.go"), "CompatibilityContract") && fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), `case "contract"`),
		HasCandidateRefill:          dirContainsTerm(root, filepath.Join("cmd", "harness"), "candidate-refill-curriculum") && dirContainsTerm(root, filepath.Join("cmd", "harness"), "release-repro-pack"),
		HasCommandAuditLog:          fileContainsTerm(root, filepath.Join("internal", "core", "audit.go"), "AuditCommandPolicy") && dirContainsTerm(root, filepath.Join("cmd", "harness"), "policy audit"),
		HasWorkerMVP:                fileContainsTerm(root, filepath.Join("internal", "core", "worker.go"), "EnqueueWorkerJob") && fileContainsTerm(root, filepath.Join("cmd", "harness", "worker.go"), "runWorkerEnqueue"),
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
			"./bin/agent-harness self-verify --target-score=95 --json",
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
