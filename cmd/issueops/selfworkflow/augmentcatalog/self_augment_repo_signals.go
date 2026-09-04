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
			signals.HasSelfVerifyCLI = fileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "root_command_facade.go"), `"self-verify":`) &&
				dirContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow"), "selfVerificationKoreanName")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasSelfAugmentPlanner = dirContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow"), "planSelfAugmentation")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasSelfAugmentStateCapture = dirContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow"), "saveSelfAugmentPlan") &&
				docsContainTerm(root, "--save-state")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasSelfAugmentLessonCapture = dirContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow"), "saveSelfAugmentLesson")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasAdapterContractMatrix = fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallAdapterContractMatrix") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "testdata", "native_install_contract_matrix.golden.json"), "project-local-opt-in")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasRiskQATier = dirContainsTerm(root, filepath.Join("cmd", "issueops", "riskqa"), "Validate") &&
				dirContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow"), "risk_qa")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasGoalScoreSummary = dirContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow"), "GoalScores") &&
				dirContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow"), "MinimumGoalScore")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasRepoLocalSandbox = dirContainsTerm(root, filepath.Join("internal", "adapter", "policy"), "path_outside_workspace") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "policy", "policy_test.go"), "TestCommandPolicyDeniesPathArgsOutsideWorkspace") &&
				(dirContainsTerm(root, filepath.Join("cmd", "issueops", "validationcli"), "policy deny outside path arg") ||
					dirContainsTerm(root, filepath.Join("cmd", "issueops", "validationcli", "commandpolicy"), "policy deny outside path arg"))
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasPerformanceBaseline = dirContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow"), "SlowStepRegressions") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow", "historycompare", "self_augment_compare_test.go"), "TestCompareSelfAugmentSummariesDetectsSlowStepRegression") &&
				docsContainTerm(root, "slow_step:*")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasSelfAugmentSignalTable = fileContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow", "augmentcatalog", "self_augment_repo_signals.go"), "func repoSignalRules() []repoSignalRule") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow", "augmentcatalog", "self_augment_repo_signals.go"), "for _, rule := range repoSignalRules()")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasQualityInspectCLI = fileContainsTerm(root, filepath.Join("cmd", "issueops", "qualitycli", "quality_inspect.go"), "quality inspect") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "root_command_facade.go"), `"quality":`)
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasQualityInspectSignals = qualityInspectContainsTerm(root, "branch_candidate_functions") &&
				qualityInspectContainsTerm(root, "audit_p1_p2_items") &&
				qualityInspectContainsTerm(root, "low_coverage_packages")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasMCPResourceCoverage = fileContainsTerm(root, filepath.Join("internal", "domain", "mcp", "resource_catalog_test.go"), "TestResourcesExposeStableDescriptors") &&
				fileContainsTerm(root, filepath.Join("internal", "domain", "mcp", "catalog_test.go"), "TestResourceMapsPreserveDescriptorShape") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "mcpcli", "resources", "resources_test.go"), "TestHandleResourceReadReportsInvalidUnknownAndReadErrors") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "mcpcli", "resources", "resources_test.go"), "TestHandleResourceReadUsesCatalogSkillNameWhenConfigSkillNameIsEmpty") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "mcpcli", "resources", "context_determinism_test.go"), "TestResourcesContextIsByteDeterministic")
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
			signals.HasWorkerStuckRunningDetection = fileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "store.go"), "func DetectStuckWorkerJobs") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "store.go"), "WorkerStatusFailed") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "worker_test.go"), "TestWorkerDetectStuckJobsMarksDeadPIDAsFailed") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "worker_test.go"), "TestWorkerDetectStuckJobsSkipsAlivePID") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "workercli", "worker.go"), `"cleanup-stuck"`) &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "workercli", "worker_queue_cli.go"), "runWorkerCleanupStuck") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "workercli", "worker_test.go"), "TestRunWorkerCleanupStuckMarksDeadPIDJobsFailed")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			hasDaemonConnectionCap := fileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_server.go"), "const maxConnections") ||
				(fileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_server.go"), "defaultMaxConnections") &&
					fileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_server.go"), "maxConnections = daemonMaxConnections"))
			signals.HasDaemonConnectionLimit = hasDaemonConnectionCap &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_server.go"), "newDaemonAdmission(maxConnections)") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_admission.go"), "case a.slots <- struct{}{}") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_admission.go"), "writeDaemonAdmissionError") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_server_loop_test.go"), "TestRunDaemonAcceptLoopRejectsWhenConnectionLimitReached") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_server_loop_test.go"), "TestRunDaemonAcceptLoopExpires64IdleSessionsAndAdmitsInitialize")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasIssueOpsInboundAdapterCoverage = fileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopsdecision", "decision_test.go"), "TestHandlersAddDelegatesToServiceAndAppliesDecision") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopsinventory", "list_test.go"), "TestListHandlerDelegatesScanAndProjectsResult") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopsretention", "prune_test.go"), "TestPruneHandlerReportsDryRunByDefault") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopsrouting", "routing_test.go"), "TestRoutingHandlersDelegateRecordAndScore") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopsstatus", "status_test.go"), "TestStatusHandlerProjectsStoredRecord") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopslease", "handlers_test.go"), "TestPublicClaimErrorMapsDenialsToStableMessages")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasToolConformanceTransportCoverage = fileContainsTerm(root, filepath.Join("internal", "contract", "toolconformance", "types_test.go"), "TestClassificationsCoverAllContractCases") &&
				fileContainsTerm(root, filepath.Join("internal", "contract", "toolconformance", "types_test.go"), "TestBenchmarkReportJSONRoundTripPreservesTypedEnums") &&
				fileContainsTerm(root, filepath.Join("internal", "contract", "issueops", "execution_sync_base_test.go"), "TestValidateWriteLeaseStatusMatrix") &&
				fileContainsTerm(root, filepath.Join("internal", "contract", "issueops", "execution_sync_base_test.go"), "TestBaseSyncRequiredErrorCarriesReseedFreeNextCommand")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasGeniusMermaidLint = dirContainsTerm(root, filepath.Join("cmd", "issueops", "validationcli"), "lintMermaidBlocks") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "validationcli", "validation_mcp_mermaid_native_wrappers_test.go"), "TestLintMermaidBlocksEnforcesGeniusThinkRules") &&
				!fileContainsTerm(root, filepath.Join(".issueops", "ARCHITECTURE.md"), `\n`)
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasInstallDryRunMode = dirContainsTerm(root, filepath.Join("cmd", "issueops", "installcli"), "dry-run") &&
				fileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallDryRunDoesNotWrite") &&
				docsContainTerm(root, "install --dry-run")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasCLIAdapterSplit = fileContainsTerm(root, filepath.Join("internal", "domain", "cli", "usage.go"), "func Usage") &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "app.go"), "cliadapter.Usage")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasMCPAdapterCatalog = hasMCPAdapterCatalog(root)
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasCompatibilityContract = (fileContainsTerm(root, filepath.Join("cmd", "issueops", "contract.go"), "CompatibilityContract") ||
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "misc_facade.go"), "CompatibilityContract")) &&
				fileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "root_command_facade.go"), `"contract":`)
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasCandidateRefill = dirContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow"), "candidate-refill-curriculum") &&
				dirContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow"), "release-repro-pack")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasCommandAuditLog = fileContainsTerm(root, filepath.Join("internal", "adapter", "audit", "audit.go"), "AuditCommandPolicy") &&
				dirContainsTerm(root, filepath.Join("cmd", "issueops", "policycli"), "policy audit")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasWorkerMVP = fileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "worker.go"), "EnqueueWorkerJob") &&
				dirContainsTerm(root, filepath.Join("cmd", "issueops", "workercli"), "runWorkerEnqueue")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasReleaseReproPack = fileContainsTerm(root, filepath.Join("scripts", "release-repro-smoke.sh"), "install --dry-run --project-local --json") &&
				fileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Release Checklist") &&
				docsContainTerm(root, "release install reproducibility smoke")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			hasReleaseGuideHeading := readmeContainsTerm(root, "Release User Guide: Install, Update, Rollback") ||
				readmeContainsTerm(root, "## Release and rollback") ||
				readmeContainsTerm(root, "## 릴리스와 롤백")
			hasInstallCommand := readmeContainsTerm(root, "./install.sh")
			hasUpdateCommand := readmeContainsTerm(root, "issueops update") ||
				readmeContainsTerm(root, "io update")
			hasRollbackReference := readmeContainsTerm(root, ".issueops/operations/release-reproducibility.md")
			signals.HasReleaseUserReadme = hasReleaseGuideHeading &&
				hasInstallCommand &&
				hasUpdateCommand &&
				hasRollbackReference &&
				!readmeContainsTerm(root, "git reset --hard") &&
				fileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Release User Guide: Install, Update, Rollback")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasCrossPlatformBuildMatrix = fileContainsTerm(root, filepath.Join("scripts", "release-build-matrix.sh"), "darwin/arm64 darwin/amd64 linux/amd64 linux/arm64") &&
				fileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Cross-Platform Build Matrix") &&
				docsContainTerm(root, "cross-platform release build matrix smoke")
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasDistributionDecision = docsContainTerm(root, "2026-06-13 — Distribution decision gate") &&
				fileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Current decision: prefer tarball/manual archive") &&
				fileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Rollback criteria") &&
				(readmeContainsTerm(root, "Current distribution decision") || readmeContainsTerm(root, "현재 배포 결정"))
		}},
		{func(root string, signals *SelfAugmentRepoSignals) {
			signals.HasReleaseDogfoodNotes = fileContainsTerm(root, filepath.Join(".issueops", "operations", "release-dogfood-notes.md"), "Codex MCP transcript") &&
				fileContainsTerm(root, filepath.Join(".issueops", "operations", "release-dogfood-notes.md"), "Claude MCP transcript") &&
				fileContainsTerm(root, filepath.Join(".issueops", "operations", "release-dogfood-notes.md"), "inspect/docs/state workflow") &&
				fileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Release Dogfood Notes")
		}},
	}
}

func readmeContainsTerm(root, term string) bool {
	return fileContainsTerm(root, "README.md", term) || fileContainsTerm(root, "README.en.md", term)
}

func hasMCPAdapterCatalog(root string) bool {
	return dirContainsTerm(root, filepath.Join("internal", "domain", "mcp"), "AdapterOwnedTools") &&
		(dirContainsTerm(root, filepath.Join("cmd", "issueops"), "mcpadapter.AdapterOwnedTools") ||
			dirContainsTerm(root, filepath.Join("cmd", "issueops", "mcpcli"), "mcpadapter.AdapterOwnedTools") ||
			dirContainsTerm(root, filepath.Join("cmd", "issueops", "contractcli"), "mcpadapter.AdapterOwnedTools"))
}

func qualityInspectContainsTerm(root, term string) bool {
	return dirContainsTerm(root, filepath.Join("cmd", "issueops", "qualitycli"), term) ||
		dirContainsTerm(root, filepath.Join("internal", "core", "qualityinspect"), term)
}
