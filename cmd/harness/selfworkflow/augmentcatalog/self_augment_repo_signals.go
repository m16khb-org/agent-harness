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
			signals.HasRepoLocalSandbox = dirContainsTerm(root, filepath.Join("internal", "adapter", "policy"), "path_outside_workspace") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "policy", "policy_test.go"), "TestCommandPolicyDeniesPathArgsOutsideWorkspace") &&
				(dirContainsTerm(root, filepath.Join("cmd", "harness", "validationcli"), "policy deny outside path arg") ||
					dirContainsTerm(root, filepath.Join("cmd", "harness", "validationcli", "commandpolicy"), "policy deny outside path arg"))
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasPerformanceBaseline = dirContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow"), "SlowStepRegressions") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow", "historycompare", "self_augment_compare_test.go"), "TestCompareSelfAugmentSummariesDetectsSlowStepRegression") &&
				docsContainTerm(root, "slow_step:*")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasSelfAugmentSignalTable = fileContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow", "augmentcatalog", "self_augment_repo_signals.go"), "func repoSignalRules() []repoSignalRule") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "selfworkflow", "augmentcatalog", "self_augment_repo_signals.go"), "for _, rule := range repoSignalRules()")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasQualityInspectCLI = fileContainsTerm(root, filepath.Join("cmd", "harness", "qualitycli", "quality_inspect.go"), "quality inspect") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "harnessapp", "root_command_facade.go"), `"quality":`)
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasQualityInspectSignals = qualityInspectContainsTerm(root, "branch_candidate_functions") &&
				qualityInspectContainsTerm(root, "audit_p1_p2_items") &&
				qualityInspectContainsTerm(root, "low_coverage_packages")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasMCPResourceCoverage = fileContainsTerm(root, filepath.Join("internal", "domain", "mcp", "resource_catalog_test.go"), "TestResourcesExposeStableDescriptors") &&
				fileContainsTerm(root, filepath.Join("internal", "domain", "mcp", "catalog_test.go"), "TestResourceMapsPreserveDescriptorShape") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "mcpcli", "resources", "resources_test.go"), "TestHandleResourceReadReportsInvalidUnknownAndReadErrors") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "mcpcli", "resources", "resources_test.go"), "TestHandleResourceReadUsesCatalogSkillNameWhenConfigSkillNameIsEmpty") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "mcpcli", "resources", "context_determinism_test.go"), "TestResourcesContextIsByteDeterministic")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasHostJudgementCoverage = fileContainsTerm(root, filepath.Join("internal", "domain", "judgement", "structured_test.go"), "TestDecodeStructuredJSONObjectRejectsMalformedOutputs") &&
				fileContainsTerm(root, filepath.Join("internal", "domain", "judgement", "structured_test.go"), "TestDecodeStructuredJSONObjectBoundsLargeErrorOutput")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasIssueOpsLinkingBoundaryCoverage = fileContainsTerm(root, filepath.Join("internal", "adapter", "issueops", "linking", "link_test.go"), "TestLinkIssueRejectsInvalidURL") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "issueops", "linking", "link_test.go"), "TestLinkPlanRejectsBoundaryViolations") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "issueops", "linking", "link_test.go"), "plan_path does not exist") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "issueops", "linking", "link_test.go"), "plan_path must be inside linked worktree") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "issueops", "linking", "link_test.go"), "TestValidateIssueURL")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasStateWriteLocking = fileContainsTerm(root, filepath.Join("internal", "application", "state", "service.go"), "func (service *Service) Write(key, content string)") &&
				fileContainsTerm(root, filepath.Join("internal", "application", "state", "service.go"), "store.WithSpan(context.Background()") &&
				fileContainsTerm(root, filepath.Join("internal", "application", "state", "service.go"), "service.writeRecord(store, dir, key, record)") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "outbound", "state", "state_io.go"), "return service().Write(key, content)") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "outbound", "state", "state_test.go"), "TestStateWriteWaitsForKeyLock")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			// kubectl 판정은 도메인으로, staged check는 디스크를 읽으므로 core에
			// 남았다. 커버리지 신호도 두 위치를 각각 확인한다.
			kubectlTest := filepath.Join("internal", "domain", "commandguard", "lifecycle_command_kubectl_test.go")
			stagedTest := filepath.Join("internal", "adapter", "commandguard", "lifecycle_command_staged_checks_test.go")
			signals.HasCommandguardBoundaryCoverage = fileContainsTerm(root, kubectlTest, "TestGitOpsKubectlDecisionBlocksMutatingCommands") &&
				fileContainsTerm(root, kubectlTest, "TestGitOpsKubectlDecisionHandlesBoundaryTokens") &&
				fileContainsTerm(root, kubectlTest, "separate dry-run flag allows apply") &&
				fileContainsTerm(root, kubectlTest, "shell separator stops rollout subverb") &&
				fileContainsTerm(root, stagedTest, "non-app/lib directories should not count as broad repo dirs")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasWorkerStuckRunningDetection = fileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "store.go"), "func DetectStuckWorkerJobs") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "store.go"), "WorkerStatusFailed") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "worker_test.go"), "TestWorkerDetectStuckJobsMarksDeadPIDAsFailed") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "worker_test.go"), "TestWorkerDetectStuckJobsSkipsAlivePID") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "workercli", "worker.go"), `"cleanup-stuck"`) &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "workercli", "worker_queue_cli.go"), "runWorkerCleanupStuck") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "workercli", "worker_test.go"), "TestRunWorkerCleanupStuckMarksDeadPIDJobsFailed")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasDaemonConnectionLimit = fileContainsTerm(root, filepath.Join("cmd", "harness", "daemoncli", "daemon_server.go"), "const maxConnections") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "daemoncli", "daemon_server.go"), "newDaemonAdmission(maxConnections)") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "daemoncli", "daemon_admission.go"), "case a.slots <- struct{}{}") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "daemoncli", "daemon_admission.go"), "writeDaemonAdmissionError") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "daemoncli", "daemon_server_loop_test.go"), "TestRunDaemonAcceptLoopRejectsWhenConnectionLimitReached") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "daemoncli", "daemon_server_loop_test.go"), "TestRunDaemonAcceptLoopExpires64IdleSessionsAndAdmitsInitialize")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasGeniusMermaidLint = dirContainsTerm(root, filepath.Join("cmd", "harness", "validationcli"), "lintMermaidBlocks") &&
				fileContainsTerm(root, filepath.Join("cmd", "harness", "validationcli", "validation_mcp_mermaid_native_wrappers_test.go"), "TestLintMermaidBlocksEnforcesGeniusThinkRules") &&
				!fileContainsTerm(root, filepath.Join(".agent-harness", "ARCHITECTURE.md"), `\n`)
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasInstallDryRunMode = dirContainsTerm(root, filepath.Join("cmd", "harness", "installcli"), "dry-run") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallDryRunDoesNotWrite") &&
				docsContainTerm(root, "install --dry-run")
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
			signals.HasCommandAuditLog = fileContainsTerm(root, filepath.Join("internal", "adapter", "audit", "audit.go"), "AuditCommandPolicy") &&
				dirContainsTerm(root, filepath.Join("cmd", "harness", "policycli"), "policy audit")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasWorkerMVP = fileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "worker.go"), "EnqueueWorkerJob") &&
				dirContainsTerm(root, filepath.Join("cmd", "harness", "workercli"), "runWorkerEnqueue")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasReleaseReproPack = fileContainsTerm(root, filepath.Join("scripts", "release-repro-smoke.sh"), "install --dry-run --project-local --json") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-reproducibility.md"), "Release Checklist") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "TESTING.md"), "release install reproducibility smoke")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			hasReleaseGuideHeading := readmeContainsTerm(root, "Release User Guide: Install, Update, Rollback") ||
				readmeContainsTerm(root, "## Release and rollback") ||
				readmeContainsTerm(root, "## 릴리스와 롤백")
			hasRollbackCommand := readmeContainsTerm(root, "git reset --hard <known-good-sha>")
			signals.HasReleaseUserReadme = hasReleaseGuideHeading &&
				readmeContainsTerm(root, "agent-harness update") &&
				readmeContainsTerm(root, "scripts/release-repro-smoke.sh") &&
				hasRollbackCommand &&
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
				(readmeContainsTerm(root, "Current distribution decision") || readmeContainsTerm(root, "현재 배포 결정"))
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasReleaseDogfoodNotes = fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-dogfood-notes.md"), "Codex MCP transcript") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-dogfood-notes.md"), "Claude MCP transcript") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-dogfood-notes.md"), "inspect/docs/state workflow") &&
				fileContainsTerm(root, filepath.Join(".agent-harness", "operations", "release-reproducibility.md"), "Release Dogfood Notes")
		}},
	}
}

func readmeContainsTerm(root, term string) bool {
	return fileContainsTerm(root, "README.md", term) || fileContainsTerm(root, "README.en.md", term)
}

func hasMCPAdapterCatalog(root string) bool {
	return dirContainsTerm(root, filepath.Join("internal", "domain", "mcp"), "AdapterOwnedTools") &&
		(dirContainsTerm(root, filepath.Join("cmd", "harness"), "mcpadapter.AdapterOwnedTools") ||
			dirContainsTerm(root, filepath.Join("cmd", "harness", "mcpcli"), "mcpadapter.AdapterOwnedTools") ||
			dirContainsTerm(root, filepath.Join("cmd", "harness", "contractcli"), "mcpadapter.AdapterOwnedTools"))
}

func qualityInspectContainsTerm(root, term string) bool {
	return dirContainsTerm(root, filepath.Join("cmd", "harness", "qualitycli"), term) ||
		dirContainsTerm(root, filepath.Join("internal", "core", "qualityinspect"), term)
}
