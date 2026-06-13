package augmentcatalog

import (
	"path/filepath"
	"strings"
)

type repoSignalRule struct {
	apply func(root string, signals *SelfAugmentRepoSignals)
}

func CollectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) SelfAugmentRepoSignals {
	signals := SelfAugmentRepoSignals{
		DocsIndexed:         docsIndexed,
		Skills:              append([]string{}, skills...),
		HasGeniusThink:      strings.TrimSpace(geniusText) != "",
		HasSelfAugmentSkill: containsString(skills, "self-augment"),
	}
	for _, rule := range repoSignalRules() {
		rule.apply(root, &signals)
	}
	return signals
}

func repoSignalRules() []repoSignalRule {
	return []repoSignalRule{
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasSelfVerificationDocs = docsContainTerm(root, "Self-verification") && docsContainTerm(root, "Self-augmentation")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasSelfVerifyCLI = fileContainsTerm(root, filepath.Join("cmd", "harness", "harnessapp", "root_command_facade.go"), `"self-verify":`) &&
				dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "selfVerificationKoreanName")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasSelfAugmentPlanner = dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "planSelfAugmentation")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasSelfAugmentStateCapture = dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "saveSelfAugmentPlan") &&
				docsContainTerm(root, "--save-state")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasSelfAugmentLessonCapture = dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "saveSelfAugmentLesson")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasAdapterContractMatrix = fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallAdapterContractMatrix") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "testdata", "native_install_contract_matrix.golden.json"), "project-local-opt-in")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasRiskQATier = dirContainsTerm(root, filepath.Join("cmd", "harness", "riskqa"), "Validate") &&
				dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "risk_qa")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasGoalScoreSummary = dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "GoalScores") &&
				dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "MinimumGoalScore")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasRepoLocalSandbox = dirContainsTerm(root, filepath.Join("internal", "core", "policy"), "path_outside_workspace") &&
				fileContainsTerm(root, filepath.Join("internal", "core", "policy", "policy_test.go"), "TestCommandPolicyDeniesPathArgsOutsideWorkspace") &&
				(dirContainsTerm(root, filepath.Join("cmd", "harness", "validationcli"), "policy deny outside path arg") ||
					dirContainsTerm(root, filepath.Join("cmd", "harness", "validationcli", "commandpolicy"), "policy deny outside path arg"))
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasPerformanceBaseline = dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "SlowStepRegressions") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow", "historycompare", "self_augment_compare_test.go"), "TestCompareSelfAugmentSummariesDetectsSlowStepRegression") &&
				docsContainTerm(root, "slow_step:*")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasGeniusMermaidLint = dirContainsTerm(root, filepath.Join("cmd", "harness", "validationcli"), "lintMermaidBlocks") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "validationcli", "validation_mcp_mermaid_native_wrappers_test.go"), "TestLintMermaidBlocksEnforcesGeniusThinkRules") &&
				!fileContainsTerm(root, filepath.Join(".agent-harness", "ARCHITECTURE.md"), `\n`)
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasInstallDryRunMode = dirContainsTerm(root, filepath.Join("cmd", "harness", "installcli"), "dry-run") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallDryRunDoesNotWrite") &&
				docsContainTerm(root, "install-native --dry-run")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasCLIAdapterSplit = fileContainsTerm(root, filepath.Join("internal", "adapter", "cli", "usage.go"), "func Usage") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "harnessapp", "app.go"), "cliadapter.Usage")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasMCPAdapterCatalog = hasMCPAdapterCatalog(root)
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasCompatibilityContract = (fileContainsTerm(root, filepath.Join("cmd", "harness", "contract.go"), "CompatibilityContract") ||
				fileContainsTerm(root, filepath.Join("cmd", "harness", "harnessapp", "misc_facade.go"), "CompatibilityContract")) &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "harnessapp", "root_command_facade.go"), `"contract":`)
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasCandidateRefill = dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "candidate-refill-curriculum") &&
				dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "release-repro-pack")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasCommandAuditLog = fileContainsTerm(root, filepath.Join("internal", "core", "audit", "audit.go"), "AuditCommandPolicy") &&
				dirContainsTerm(root, filepath.Join("cmd", "harness", "policycli"), "policy audit")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasWorkerMVP = fileContainsTerm(root, filepath.Join("internal", "core", "worker", "worker.go"), "EnqueueWorkerJob") &&
				dirContainsTerm(root, filepath.Join("cmd", "harness", "workercli"), "runWorkerEnqueue")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasReleaseReproPack = fileContainsTerm(root, filepath.Join("scripts", "release-repro-smoke.sh"), "install-native --dry-run --project-local --json") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-reproducibility.md"), "Release Checklist") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "TESTING.md"), "release install reproducibility smoke")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasReleaseUserReadme = fileContainsTerm(root, "README.md", "Release User Guide: Install, Update, Rollback") &&
				fileContainsTerm(root, "README.md", "agent-harness update") &&
				fileContainsTerm(root, "README.md", "scripts/release-repro-smoke.sh") &&
				fileContainsTerm(root, "README.md", "git reset --hard <known-good-sha>") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-reproducibility.md"), "Release User Guide: Install, Update, Rollback")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasCrossPlatformBuildMatrix = fileContainsTerm(root, filepath.Join("scripts", "release-build-matrix.sh"), "darwin/arm64 darwin/amd64 linux/amd64 linux/arm64") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-reproducibility.md"), "Cross-Platform Build Matrix") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "TESTING.md"), "cross-platform release build matrix smoke")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasDistributionDecision = fileContainsTerm(root, filepath.Join(".agent-harness", "ADR.md"), "2026-06-13 — Distribution decision gate") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-reproducibility.md"), "Current decision: prefer tarball/manual archive") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-reproducibility.md"), "Rollback criteria") &&
				fileContainsTerm(root, "README.md", "Current distribution decision")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasReleaseDogfoodNotes = fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-dogfood-notes.md"), "Codex MCP transcript") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-dogfood-notes.md"), "Claude MCP transcript") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-dogfood-notes.md"), "inspect/docs/state workflow") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-reproducibility.md"), "Release Dogfood Notes")
		}},
	}
}

func hasMCPAdapterCatalog(root string) bool {
	return dirContainsTerm(root, filepath.Join("internal", "adapter", "mcp"), "AdapterOwnedTools") &&
		(dirContainsTerm(root, filepath.Join("cmd", "harness"), "mcpadapter.AdapterOwnedTools") ||
			dirContainsTerm(root, filepath.Join("cmd", "harness", "mcpcli"), "mcpadapter.AdapterOwnedTools") ||
			dirContainsTerm(root, filepath.Join("cmd", "harness", "contractcli"), "mcpadapter.AdapterOwnedTools"))
}
