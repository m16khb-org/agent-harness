package main

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
	case "cli-mcp-adapter-split":
		if signals.HasCLIAdapterSplit && signals.HasMCPAdapterCatalog {
			evidence = []string{"CLI usage lives in internal/adapter/cli", "MCP adapter-owned tool descriptors live in internal/adapter/mcp"}
		}
	case "dto-compatibility-contract":
		if signals.HasCompatibilityContract {
			evidence = []string{"harness contract schema/check exposes CLI/MCP compatibility contract"}
		}
	case "candidate-refill-curriculum":
		if signals.HasCandidateRefill {
			evidence = []string{"self-augment catalog includes second-wave candidates and release-repro-pack open follow-up"}
		}
	case "policy-audit-redaction":
		if signals.HasCommandAuditLog {
			evidence = []string{"policy audit writes append-only redacted JSONL records without executing commands"}
		}
	case "worker-mvp-no-shell":
		if signals.HasWorkerMVP {
			evidence = []string{"worker MVP persists queued/cancelled job lifecycle records and never executes shell commands"}
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
