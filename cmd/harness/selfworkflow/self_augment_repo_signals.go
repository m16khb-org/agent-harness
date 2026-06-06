package selfworkflow

import (
	"path/filepath"
	"strings"
)

func collectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) SelfAugmentRepoSignals {
	return SelfAugmentRepoSignals{
		DocsIndexed:                 docsIndexed,
		Skills:                      append([]string{}, skills...),
		HasGeniusThink:              strings.TrimSpace(geniusText) != "",
		HasSelfAugmentSkill:         containsString(skills, "self-augment"),
		HasSelfVerificationDocs:     docsContainTerm(root, "Self-verification") && docsContainTerm(root, "Self-augmentation"),
		HasSelfVerifyCLI:            dirContainsTerm(root, filepath.Join("cmd", "harness"), `case "self-verify"`) && dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "selfVerificationKoreanName"),
		HasSelfAugmentPlanner:       dirContainsTerm(root, filepath.Join("cmd", "harness"), "planSelfAugmentation"),
		HasSelfAugmentStateCapture:  dirContainsTerm(root, filepath.Join("cmd", "harness"), "saveSelfAugmentPlan") && docsContainTerm(root, "--save-state"),
		HasSelfAugmentLessonCapture: dirContainsTerm(root, filepath.Join("cmd", "harness"), "saveSelfAugmentLesson"),
		HasAdapterContractMatrix:    fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallAdapterContractMatrix") && fileContainsTerm(root, filepath.Join("internal", "adapter", "testdata", "native_install_contract_matrix.golden.json"), "project-local-opt-in"),
		HasRiskQATier:               dirContainsTerm(root, filepath.Join("cmd", "harness", "riskqa"), "Validate") && dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "risk_qa"),
		HasGoalScoreSummary:         dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "GoalScores") && dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "MinimumGoalScore"),
		HasRepoLocalSandbox:         dirContainsTerm(root, filepath.Join("internal", "core", "policy"), "path_outside_workspace") && fileContainsTerm(root, filepath.Join("internal", "core", "policy", "policy_test.go"), "TestCommandPolicyDeniesPathArgsOutsideWorkspace") && dirContainsTerm(root, filepath.Join("cmd", "harness", "validationcli"), "policy deny outside path arg"),
		HasPerformanceBaseline:      dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "SlowStepRegressions") && fileContainsTerm(root, filepath.Join("cmd", "harness", "self_augment_compare_test.go"), "TestCompareSelfAugmentSummariesDetectsSlowStepRegression") && docsContainTerm(root, "slow_step:*"),
		HasGeniusMermaidLint:        dirContainsTerm(root, filepath.Join("cmd", "harness"), "lintMermaidBlocks") && fileContainsTerm(root, filepath.Join("cmd", "harness", "self_augment_history_test.go"), "TestLintMermaidBlocksEnforcesGeniusThinkRules") && !fileContainsTerm(root, filepath.Join(".agent-harness", "ARCHITECTURE.md"), `\n`),
		HasInstallDryRunMode:        dirContainsTerm(root, filepath.Join("cmd", "harness", "installcli"), "dry-run") && fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallDryRunDoesNotWrite") && docsContainTerm(root, "install-native --dry-run"),
		HasCLIAdapterSplit:          fileContainsTerm(root, filepath.Join("internal", "adapter", "cli", "usage.go"), "func Usage") && fileContainsTerm(root, filepath.Join("cmd", "harness", "main.go"), "cliadapter.Usage"),
		HasMCPAdapterCatalog:        dirContainsTerm(root, filepath.Join("internal", "adapter", "mcp"), "AdapterOwnedTools") && dirContainsTerm(root, filepath.Join("cmd", "harness"), "mcpadapter.AdapterOwnedTools"),
		HasCompatibilityContract:    fileContainsTerm(root, filepath.Join("cmd", "harness", "contract.go"), "CompatibilityContract") && dirContainsTerm(root, filepath.Join("cmd", "harness"), `case "contract"`),
		HasCandidateRefill:          dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "candidate-refill-curriculum") && dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "release-repro-pack"),
		HasCommandAuditLog:          fileContainsTerm(root, filepath.Join("internal", "core", "audit", "audit.go"), "AuditCommandPolicy") && dirContainsTerm(root, filepath.Join("cmd", "harness", "policycli"), "policy audit"),
		HasWorkerMVP:                fileContainsTerm(root, filepath.Join("internal", "core", "worker", "worker.go"), "EnqueueWorkerJob") && dirContainsTerm(root, filepath.Join("cmd", "harness", "workercli"), "runWorkerEnqueue"),
	}
}
