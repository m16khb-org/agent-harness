package augmentcatalog

func MarkSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	var evidence []string
	for _, rule := range candidateSatisfactionRules() {
		if rule.id == candidate.ID {
			evidence = rule.evidence(signals)
			break
		}
	}
	if len(evidence) == 0 {
		return
	}
	candidate.Status = SelfAugmentCandidateStatusSatisfied
	candidate.SatisfactionEvidence = evidence
	candidate.Score = 0
	candidate.WhyNow = append(candidate.WhyNow, "Already satisfied; do not select in the next self-augmentation cycle")
}

type candidateSatisfactionRule struct {
	id       string
	evidence func(SelfAugmentRepoSignals) []string
}

func candidateSatisfactionRules() []candidateSatisfactionRule {
	return []candidateSatisfactionRule{
		{"loop-taxonomy-score-gates", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasSelfVerifyCLI && signals.HasSelfAugmentPlanner && signals.HasSelfVerificationDocs && signals.HasGoalScoreSummary, "self-verify CLI exists", "self-augment planner exists", "loop docs distinguish both Korean names", "goal score summary is implemented")
		}},
		{"agent-skill-executor", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasSelfAugmentSkill, "skills/self-augment exists in shared skill inventory")
		}},
		{"durable-augmentation-memory", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasSelfAugmentStateCapture, "self-augment --save-state persists selected candidate curriculum to harness state")
		}},
		{"reflexion-state-memory", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasSelfAugmentLessonCapture, "self-augment lesson stores Reflexion lessons in harness state")
		}},
		{"qa-dashboard-summary", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasGoalScoreSummary, "self-verify summary includes goal_scores and minimum_goal_score")
		}},
		{"qa-race-tier", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasRiskQATier, "self-verify includes a risk QA tier that conditionally runs go test -race and go vet for sensitive Go changes")
		}},
		{"adapter-contract-matrix", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasAdapterContractMatrix, "internal/adapter install contract matrix locks Codex/Claude user-global and project-local installation behavior with a golden fixture")
		}},
		{"repo-local-augmentation-sandbox", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasRepoLocalSandbox, "command policy rejects path-like argv outside workspace_root and self-verify command policy smoke covers the boundary")
		}},
		{"performance-baseline", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasPerformanceBaseline, "self-verify compare promotes label-level slowest_steps deltas into slow_step regressions with unit coverage")
		}},
		{"self-augment-signal-table", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasSelfAugmentSignalTable, "self-augment repository signal collection is table-driven through repoSignalRules")
		}},
		{"quality-signal-harvester", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasQualityInspectCLI && signals.HasQualityInspectSignals, "quality inspect CLI exists", "quality inspect emits coverage, branch complexity, and audit risk signals")
		}},
		{"coverage-mcp-resources", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasMCPResourceCoverage, "MCP resources cover stable descriptors, deterministic context, read errors, and catalog skill fallback")
		}},
		{"coverage-host-judgement", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasHostJudgementCoverage, "host-agent judgement decoder covers malformed structured output and bounded error output")
		}},
		{"coverage-issueops-linking", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasIssueOpsLinkingBoundaryCoverage, "issueops linking covers invalid issue URLs, missing plan files, and plan/worktree boundary violations")
		}},
		{"state-write-locking", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasStateWriteLocking, "StateWrite delegates to the application service, which serializes writes through the store span with regression coverage")
		}},
		{"coverage-commandguard", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasCommandguardBoundaryCoverage, "commandguard covers kubectl mutation denial, dry-run, shell separator, and broad Biome boundary paths")
		}},
		{"worker-stuck-running-detection", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasWorkerStuckRunningDetection, "worker cleanup-stuck detects running jobs with dead PIDs and marks them failed through CLI/core coverage")
		}},
		{"daemon-connection-limit", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasDaemonConnectionLimit, "daemon accept loop enforces maxConnections with rejection and graceful-drain coverage")
		}},
		{"genius-mermaid-lint", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasGeniusMermaidLint, "QA gate lints Mermaid fences using GENIUS_THINK quote/<br/> rules and repo diagrams were normalized")
		}},
		{"install-dry-run-mode", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasInstallDryRunMode, "install supports --dry-run planning with no filesystem writes and adapter-level coverage")
		}},
		{"cli-mcp-adapter-split", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasCLIAdapterSplit && signals.HasMCPAdapterCatalog, "CLI usage lives in internal/domain/cli", "MCP adapter-owned tool descriptors live in internal/domain/mcp")
		}},
		{"dto-compatibility-contract", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasCompatibilityContract, "harness contract schema/check exposes CLI/MCP compatibility contract")
		}},
		{"candidate-refill-curriculum", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasCandidateRefill, "self-augment catalog includes second-wave candidates and release-repro-pack open follow-up")
		}},
		{"policy-audit-redaction", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasCommandAuditLog, "policy audit writes append-only redacted JSONL records without executing commands")
		}},
		{"worker-mvp-no-shell", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasWorkerMVP, "worker MVP persists queued/cancelled job lifecycle records and never executes shell commands")
		}},
		{"release-repro-pack", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasReleaseReproPack, "release reproducibility checklist exists", "scripts/release-repro-smoke.sh verifies temp HOME/CODEX_HOME/HARNESS_ROOT install dry-run", "testing docs list the release install reproducibility smoke")
		}},
		{"release-user-readme", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasReleaseUserReadme, "README contains install and update commands", "README links canonical release reproducibility and rollback criteria", "README omits copyable destructive rollback commands")
		}},
		{"cross-platform-build-matrix", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasCrossPlatformBuildMatrix, "scripts/release-build-matrix.sh builds the supported darwin/linux amd64/arm64 matrix", "release reproducibility docs record the cross-platform matrix", "testing docs list the release build matrix smoke")
		}},
		{"distribution-decision-record", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasDistributionDecision, "ADR records the tarball/manual archive versus Homebrew decision gate", "release reproducibility docs define rollback criteria", "README exposes the current distribution decision")
		}},
		{"release-dogfood-notes", func(signals SelfAugmentRepoSignals) []string {
			return evidenceWhen(signals.HasReleaseDogfoodNotes, "release dogfood notes capture Codex MCP registration", "release dogfood notes capture Claude MCP registration", "shared inspect/docs/state workflow is recorded")
		}},
	}
}

func evidenceWhen(ok bool, evidence ...string) []string {
	if !ok {
		return nil
	}
	return evidence
}

func SelfAugmentCandidateScore(c SelfAugmentCandidate) float64 {
	score := c.Impact*0.38 + c.Feasibility*0.30 + c.Novelty*0.20 + (100-c.Risk)*0.12
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}
