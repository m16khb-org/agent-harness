package augmentcatalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirContainsTermIgnoresTestOnlySignals(t *testing.T) {
	root := t.TempDir()
	relDir := filepath.Join("cmd", "harness")
	writeFileForRepoSignalTest(t, filepath.Join(root, relDir, "signal_test.go"), "package main\nconst marker = \"production-only-signal\"\n")

	if dirContainsTerm(root, relDir, "production-only-signal") {
		t.Fatalf("test-only source was accepted as production repo signal")
	}

	writeFileForRepoSignalTest(t, filepath.Join(root, relDir, "signal.go"), "package main\nconst marker = \"production-only-signal\"\n")
	if !dirContainsTerm(root, relDir, "production-only-signal") {
		t.Fatalf("production source was not accepted as repo signal")
	}
}

func TestCollectSelfAugmentRepoSignalsFindsMCPAdapterCatalogInContractCLI(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "adapter", "mcp", "catalog.go"), "package mcp\nfunc AdapterOwnedTools() {}\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "contractcli", "contract.go"), "package contractcli\nconst marker = \"mcpadapter.AdapterOwnedTools\"\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasMCPAdapterCatalog {
		t.Fatalf("contractcli MCP adapter catalog signal was not detected: %+v", signals)
	}
}

func TestSelfAugmentSignalTableIsSatisfiedByRepoSignalRules(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "selfworkflow", "augmentcatalog", "self_augment_repo_signals.go"), "package augmentcatalog\ntype repoSignalRule struct{}\nfunc repoSignalRules() []repoSignalRule { return nil }\nfunc CollectSelfAugmentRepoSignals() { for _, rule := range repoSignalRules() { _ = rule } }\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasSelfAugmentSignalTable {
		t.Fatalf("self-augment signal table signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "self-augment-signal-table", Status: SelfAugmentCandidateStatusOpen, Score: 83.8}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("self-augment signal-table candidate was not marked satisfied: %+v", candidate)
	}
}

func TestQualitySignalHarvesterIsSatisfiedByQualityInspectCLIAndSignals(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "qualitycli", "quality_inspect.go"), "package qualitycli\nfunc Inspect() {}\nconst marker = \"quality inspect\"\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "harnessapp", "root_command_facade.go"), "package harnessapp\nvar commands = map[string]any{\"quality\": nil}\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "core", "qualityinspect", "inspect.go"), "package qualityinspect\nconst marker = \"branch_candidate_functions audit_p1_p2_items low_coverage_packages\"\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasQualityInspectCLI || !signals.HasQualityInspectSignals {
		t.Fatalf("quality inspect signals were not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "quality-signal-harvester", Status: SelfAugmentCandidateStatusOpen, Score: 89.24}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("quality signal harvester candidate was not marked satisfied: %+v", candidate)
	}
}

func TestIssueOpsLinkingCoverageIsSatisfiedByBoundaryTests(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "core", "issueops", "linking", "link_test.go"), `package linking

func TestLinkIssueRejectsInvalidURL() {
	_ = "http(s) URL"
}

func TestLinkPlanRejectsBoundaryViolations() {
	_ = "plan_path does not exist"
	_ = "plan_path must be inside linked worktree"
}

func TestValidateIssueURL() {}
`)

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasIssueOpsLinkingBoundaryCoverage {
		t.Fatalf("issueops linking boundary coverage signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "coverage-issueops-linking", Status: SelfAugmentCandidateStatusOpen, Score: 77.4}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("issueops linking coverage candidate was not marked satisfied: %+v", candidate)
	}
}

func TestStateWriteLockingIsSatisfiedByLockedStateWrite(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "application", "state", "service.go"), `package state

func (service *Service) Write(key, content string) (StateResult, error) {
	err := store.WithSpan(context.Background(), func(context.Context) error {
		_, err := service.writeRecord(store, dir, key, record)
		return err
	})
	return result, err
}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "adapter", "outbound", "state", "state_io.go"), `package state

func StateWrite(key, content string) (StateResult, error) {
	return service().Write(key, content)
}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "adapter", "outbound", "state", "state_test.go"), `package state

func TestStateWriteWaitsForKeyLock() {}
`)

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasStateWriteLocking {
		t.Fatalf("state write locking signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "state-write-locking", Status: SelfAugmentCandidateStatusOpen, Score: 77.22}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("state write locking candidate was not marked satisfied: %+v", candidate)
	}
}

func TestCommandguardCoverageIsSatisfiedByBoundaryTests(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "domain", "commandguard", "lifecycle_command_kubectl_test.go"), `package commandguard

func TestGitOpsKubectlDecisionBlocksMutatingCommands() {}
func TestGitOpsKubectlDecisionHandlesBoundaryTokens() {
	_ = "separate dry-run flag allows apply"
	_ = "shell separator stops rollout subverb"
	_ = "rollout undo is blocked"
}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "core", "commandguard", "lifecycle_command_staged_checks_test.go"), `package commandguard

func TestStagedCheckDecisionWarnsForBroadBiomeCommands() {}
func TestPackageScriptAndBiomeHelpersHandleBoundaries() {
	_ = "non-app/lib directories should not count as broad repo dirs"
}
`)

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasCommandguardBoundaryCoverage {
		t.Fatalf("commandguard boundary coverage signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "coverage-commandguard", Status: SelfAugmentCandidateStatusOpen, Score: 77.08}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("commandguard coverage candidate was not marked satisfied: %+v", candidate)
	}
}

func TestWorkerStuckRunningDetectionIsSatisfiedByCoreAndCLI(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "core", "worker", "store.go"), `package worker

func DetectStuckWorkerJobs() (WorkerListResult, error) {
	current.Status = WorkerStatusFailed
	current.SafetyNotice = "worker job was stuck in running status with dead PID; auto-marked as failed"
	return result, nil
}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "core", "worker", "worker_test.go"), `package worker

func TestWorkerDetectStuckJobsMarksDeadPIDAsFailed() {}
func TestWorkerDetectStuckJobsSkipsAlivePID() {}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "core", "workflow_facade.go"), `package core

func DetectStuckWorkerJobs() (WorkerListResult, error) {
	return coreworker.DetectStuckWorkerJobs()
}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "workercli", "worker.go"), `package workercli

func runWorker(args []string) error {
	switch args[0] {
	case "cleanup-stuck":
		return runWorkerCleanupStuck(args[1:])
	}
	return nil
}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "workercli", "worker_queue_cli.go"), `package workercli

func runWorkerCleanupStuck(args []string) error {
	result, err := core.DetectStuckWorkerJobs()
	_ = result
	return err
}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "workercli", "worker_test.go"), `package workercli

func TestRunWorkerCleanupStuckMarksDeadPIDJobsFailed() {}
`)

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasWorkerStuckRunningDetection {
		t.Fatalf("worker stuck-running detection signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "worker-stuck-running-detection", Status: SelfAugmentCandidateStatusOpen, Score: 76.96}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("worker stuck-running candidate was not marked satisfied: %+v", candidate)
	}
}

func TestDaemonConnectionLimitIsSatisfiedByAcceptLoopGuard(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "daemoncli", "daemon_server.go"), `package daemoncli

const maxConnections = 64

func runDaemonServerWithDeps() {
	admission := newDaemonAdmission(maxConnections)
	_ = admission
}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "daemoncli", "daemon_admission.go"), `package daemoncli

const daemonStatusConnectionLimit = "daemon_connection_limit_reached"

type daemonAdmission struct { slots chan struct{} }

func (a *daemonAdmission) acquire() bool {
	select {
	case a.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func writeDaemonAdmissionError() {}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "daemoncli", "daemon_server_loop_test.go"), `package daemoncli

func TestRunDaemonAcceptLoopRejectsWhenConnectionLimitReached() {}
func TestRunDaemonAcceptLoopExpires64IdleSessionsAndAdmitsInitialize() {}
`)

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasDaemonConnectionLimit {
		t.Fatalf("daemon connection limit signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "daemon-connection-limit", Status: SelfAugmentCandidateStatusOpen, Score: 76.72}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("daemon connection limit candidate was not marked satisfied: %+v", candidate)
	}
}

func TestDraftWikiStaleLockIsSatisfiedByQueueLockRecovery(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "core", "draftwiki", "queue", "lock.go"), `package queue

const staleLockMaxAge = 5 * time.Minute

func AcquireLock(projectStateDir string) (func(), bool, error) {
	if isStale(path) {
		_ = os.Remove(path)
	}
	return func() {}, true, nil
}

func isStale(path string) bool {
	return !processAlive(pid)
}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "core", "draftwiki", "queue", "queue_test.go"), `package queue

func TestAcquireLockRecoversStaleDeadPIDLock() {}
func TestAcquireLockKeepsLiveCurrentLock() {}
func TestAcquireLockContention() {}
`)

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasDraftWikiStaleLockDetection {
		t.Fatalf("draft-wiki stale lock signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "draftwiki-stale-lock", Status: SelfAugmentCandidateStatusOpen, Score: 76.24}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("draft-wiki stale lock candidate was not marked satisfied: %+v", candidate)
	}
}

func TestMCPResourceCoverageIsSatisfiedByCatalogAndReadEdgeTests(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "adapter", "mcp", "resource_catalog_test.go"), `package mcp

func TestResourcesExposeStableDescriptors() {}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "adapter", "mcp", "catalog_test.go"), `package mcp

func TestResourceMapsPreserveDescriptorShape() {}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "mcpcli", "resources", "resources_test.go"), `package resources

func TestHandleResourceReadReportsInvalidUnknownAndReadErrors() {}
func TestHandleResourceReadUsesCatalogSkillNameWhenConfigSkillNameIsEmpty() {}
func TestContentShapeAndHarnessFileResourceMappings() {}
`)
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "mcpcli", "resources", "context_determinism_test.go"), `package resources

func TestResourcesContextIsByteDeterministic() {}
`)

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasMCPResourceCoverage {
		t.Fatalf("MCP resource coverage signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "coverage-mcp-resources", Status: SelfAugmentCandidateStatusOpen, Score: 76.16}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("MCP resource coverage candidate was not marked satisfied: %+v", candidate)
	}
}

func TestHostJudgementCoverageIsSatisfiedByMalformedResultTests(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "domain", "judgement", "structured_test.go"), `package judgement

func TestDecodeStructuredJSONObjectRejectsMalformedOutputs() {}
func TestDecodeStructuredJSONObjectBoundsLargeErrorOutput() {}
`)

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasHostJudgementCoverage {
		t.Fatalf("host judgement coverage signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "coverage-host-judgement", Status: SelfAugmentCandidateStatusOpen, Score: 76}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("host judgement coverage candidate was not marked satisfied: %+v", candidate)
	}
}

func TestReleaseReproPackIsSatisfiedByChecklistScriptAndTestingSignal(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "scripts", "release-repro-smoke.sh"), "agent-harness install --dry-run --project-local --json\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "# Release Reproducibility\n\n## Release Checklist\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "TESTING.md"), "- `scripts/release-repro-smoke.sh` clean-machine release install reproducibility smoke\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasReleaseReproPack {
		t.Fatalf("release repro pack signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "release-repro-pack", Status: SelfAugmentCandidateStatusOpen, Score: 79}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("release repro candidate was not marked satisfied: %+v", candidate)
	}
}

func TestReleaseUserReadmeIsSatisfiedByInstallUpdateRollbackGuide(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "README.md"), "# agent-harness\n\n## Release User Guide: Install, Update, Rollback\n\n```bash\nagent-harness update\nscripts/release-repro-smoke.sh\ngit reset --hard <known-good-sha>\n```\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "README section: Release User Guide: Install, Update, Rollback\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasReleaseUserReadme {
		t.Fatalf("release user README signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "release-user-readme", Status: SelfAugmentCandidateStatusOpen, Score: 80.28}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("release user README candidate was not marked satisfied: %+v", candidate)
	}
}

func TestReleaseUserReadmeSupportsKoreanCanonicalReadmeWithEnglishCompanion(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "README.md"), "# agent-harness\n\n[English](README.en.md)\n\n## 릴리스와 롤백\n\n```bash\nagent-harness update\nscripts/release-repro-smoke.sh\ngit reset --hard <known-good-sha>\n```\n\n[릴리스 재현성과 롤백](.agent-harness/operations/release-reproducibility.md)\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, "README.en.md"), "# agent-harness\n\n[한국어](README.md)\n\n## Release and rollback\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "README section: Release User Guide: Install, Update, Rollback\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasReleaseUserReadme {
		t.Fatalf("split Korean/English release README signal was not detected: %+v", signals)
	}
}

func TestReleaseUserReadmeRequiresRollbackCommand(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "README.md"), "## 릴리스와 롤백\n\nagent-harness update\nscripts/release-repro-smoke.sh\n\n[롤백](.agent-harness/operations/release-reproducibility.md)\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "README section: Release User Guide: Install, Update, Rollback\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if signals.HasReleaseUserReadme {
		t.Fatal("release README signal should require a concrete rollback command")
	}
}

func TestReadmeContainsTermReadsEnglishCompanion(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "README.md"), "# agent-harness\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, "README.en.md"), "companion-only-marker\n")

	if !readmeContainsTerm(root, "companion-only-marker") {
		t.Fatal("term from English companion README was not detected")
	}
}

func TestCrossPlatformBuildMatrixIsSatisfiedByScriptDocsAndTestingSignal(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "scripts", "release-build-matrix.sh"), "TARGETS=\"darwin/arm64 darwin/amd64 linux/amd64 linux/arm64\"\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "## Cross-Platform Build Matrix\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "TESTING.md"), "- `scripts/release-build-matrix.sh` cross-platform release build matrix smoke\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasCrossPlatformBuildMatrix {
		t.Fatalf("cross-platform build matrix signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "cross-platform-build-matrix", Status: SelfAugmentCandidateStatusOpen, Score: 78.76}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("cross-platform build matrix candidate was not marked satisfied: %+v", candidate)
	}
}

func TestDistributionDecisionRecordIsSatisfiedByADRReleaseDocsAndReadme(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "ADR.md"), "### 2026-06-13 — Distribution decision gate\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "Current decision: prefer tarball/manual archive\n\nRollback criteria\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, "README.md"), "Current distribution decision: use a tarball/manual archive for the first release\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasDistributionDecision {
		t.Fatalf("distribution decision signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "distribution-decision-record", Status: SelfAugmentCandidateStatusOpen, Score: 78.54}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("distribution decision candidate was not marked satisfied: %+v", candidate)
	}
}

func TestDistributionDecisionSupportsKoreanCanonicalReadme(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "ADR.md"), "### 2026-06-13 — Distribution decision gate\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "Current decision: prefer tarball/manual archive\n\nRollback criteria\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, "README.md"), "현재 배포 결정은 tarball/manual archive를 우선합니다.\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasDistributionDecision {
		t.Fatalf("Korean README distribution decision signal was not detected: %+v", signals)
	}
}

func TestReleaseDogfoodNotesIsSatisfiedByHostTranscripts(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-dogfood-notes.md"), "## Codex MCP transcript\n\n## Claude MCP transcript\n\n## inspect/docs/state workflow\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "## Release Dogfood Notes\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasReleaseDogfoodNotes {
		t.Fatalf("release dogfood notes signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "release-dogfood-notes", Status: SelfAugmentCandidateStatusOpen, Score: 78.44}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("release dogfood notes candidate was not marked satisfied: %+v", candidate)
	}
}

func TestSelectedCandidateIDReturnsStableFallback(t *testing.T) {
	if got := SelectedCandidateID(nil); got != "" {
		t.Fatalf("SelectedCandidateID(nil)=%q, want empty string", got)
	}
	candidate := SelfAugmentCandidate{ID: "augment-next"}
	if got := SelectedCandidateID(&candidate); got != "augment-next" {
		t.Fatalf("SelectedCandidateID returned %q", got)
	}
}

func writeFileForRepoSignalTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
