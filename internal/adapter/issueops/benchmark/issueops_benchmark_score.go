package benchmark

import issueopscontract "agent-harness/internal/contract/issueops"

import "strings"

func ScoreIssueOpsBenchmarkArtifact(fixture issueopscontract.IssueOpsBenchmarkFixture, artifact issueopscontract.IssueOpsBenchmarkArtifact) IssueOpsBenchmarkScore {
	checks := map[string]struct {
		ok       bool
		evidence string
		failure  string
	}{
		"intent_understanding": {
			ok:       strings.TrimSpace(artifact.ProblemSummary) != "" || strings.TrimSpace(artifact.IssueDraft) != "",
			evidence: "problem summary or issue draft is present",
			failure:  "missing problem summary or issue draft",
		},
		"issue_quality": {
			ok:       hasAllIssueOpsConcepts(artifact.IssueDraft, issueOpsIssueSectionConcepts) && containsHangul(artifact.IssueDraft) && hasIssueOpsGuidelineRef(artifact) && issueOpsLabelDecisionEvidenceComplete(artifact),
			evidence: "issue draft includes required sections, Korean text, guideline reference, and label scoring decision evidence",
			failure:  "issue draft missing required sections, Korean text, guideline reference, or label scoring decision evidence",
		},
		"domain_contract_quality": {
			ok:       issueOpsDomainContractEvidenceComplete(artifact),
			evidence: "domain contract evidence separates invariants, exact mechanisms, equivalent behavior, and source lines",
			failure:  "domain contract evidence missing invariants, exact mechanisms, equivalent behavior, or source lines",
		},
		"plan_quality": {
			ok:       containsAnyFold(artifact.Plan, "go test", "verify", "verification", "test"),
			evidence: "plan includes verification commands or tests",
			failure:  "plan missing verification commands",
		},
		"api_doc_gate_quality": {
			ok:       issueOpsAPIDocGateEvidenceComplete(artifact),
			evidence: "API documentation gate evidence covers changed endpoints, public error responses, and static plus review checks",
			failure:  "API documentation gate evidence missing changed endpoints, public error responses, static check, or review check",
		},
		"live_evidence_quality": {
			ok:       issueOpsLiveEvidenceMatrixComplete(artifact),
			evidence: "live evidence matrix separates environments, repo/config evidence, runtime evidence, and remediation order",
			failure:  "live evidence matrix missing environments, repo/config evidence, runtime evidence, or remediation order",
		},
		"task_decomposition": {
			ok:       containsAllFold(artifact.TaskBreakdown, "owns") && containsAnyFold(artifact.TaskBreakdown, "worker", "task") && issueOpsHierarchyEvidenceComplete(artifact),
			evidence: "task breakdown assigns bounded ownership and uses provider-native hierarchy for large issues",
			failure:  "task breakdown missing bounded ownership or provider-native hierarchy",
		},
		"tdd_quality": {
			ok:       containsAllFold(artifact.TDDPlan, "failing", "test") && containsAnyFold(artifact.TDDPlan, "before", "first"),
			evidence: "TDD plan names failing tests before implementation",
			failure:  "TDD plan missing failing-test-first evidence",
		},
		"subagent_orchestration": {
			ok:       containsAllFold(artifact.SubagentPrompts, "not alone", "do not revert", "own") && containsAnyFold(artifact.SubagentPrompts, "expected output", "report") && workerPromptHasContextGate(artifact) && reviewPromptIsBounded(artifact),
			evidence: "subagent prompts include coordination, ownership, context verification, and bounded review guidance",
			failure:  "subagent prompts missing coordination, context verification, or bounded review guidance",
		},
		"review_feedback_accountability": {
			ok:       issueOpsReviewFeedbackEvidenceComplete(artifact),
			evidence: "review feedback evidence classifies claims, cites verification, records thread replies, and tracks resolution",
			failure:  "review feedback evidence missing claim classification, verification, thread replies, or resolution status",
		},
		"implementation_readiness": {
			ok:       strings.TrimSpace(artifact.BranchName) != "" && strings.TrimSpace(artifact.WorktreePath) != "",
			evidence: "branch and worktree are recorded",
			failure:  "branch or worktree evidence missing",
		},
		"pr_mr_quality": {
			ok:       hasAllIssueOpsConcepts(artifact.PRDraft, issueOpsPRSectionConcepts) && containsAnyFold(artifact.PRDraft, "issue:", "fixes", "closes", "이슈:") && containsHangul(artifact.PRDraft) && hasIssueOpsGuidelineRef(artifact),
			evidence: "PR/MR draft includes required sections, issue link, Korean text, reviewer notes, and guideline reference",
			failure:  "PR/MR draft missing required sections, issue link, Korean text, reviewer notes, or guideline reference",
		},
		"phase_control_quality": {
			ok:       containsAllFold(artifact.PhaseChoices, "proceed", "revise", "jump", "pause"),
			evidence: "phase choices include proceed, revise, jump, and pause",
			failure:  "phase choices missing required options",
		},
		"branch_worktree_gate_quality": {
			ok:       strings.HasPrefix(artifact.BranchName, "feature/") && strings.Contains(artifact.WorktreePath, ".worktrees/"),
			evidence: "issue branch and sibling worktree path are recorded",
			failure:  "issue branch or sibling worktree gate missing",
		},
		"isolation_compliance": {
			ok:       implementationInWorktree(artifact),
			evidence: "implementation location is inside the isolated worktree",
			failure:  "implementation location is outside the isolated worktree",
		},
		"completion_hygiene_quality": {
			ok:       issueOpsCompletionHygieneComplete(artifact),
			evidence: "completion hygiene records final diff, target branch, remote artifact refresh, single-commit policy, and cleanup status",
			failure:  "completion hygiene missing final diff, target branch, remote artifact refresh, single-commit policy, or cleanup status",
		},
		"worktree_cleanup_quality": {
			ok:       containsAnyFold(artifact.WorktreeCleanup, "clean") && containsAnyFold(artifact.WorktreeCleanup, "cleanup", "remove") && containsAnyFold(artifact.WorktreeCleanup, "choice", "offered", "present"),
			evidence: "worktree cleanup status and choices are recorded",
			failure:  "worktree cleanup status or choices missing",
		},
		"pioneer_skill_contribution": {
			ok:       issueOpsPioneerSkillEvidenceComplete(fixture, artifact),
			evidence: "pioneer skill evidence carries the targeted skill's distinctive-method signature (necessary keyword proxy, not live-routing proof)",
			failure:  "pioneer skill evidence missing the targeted skill's distinctive-method signature",
		},
		"skill_routing_fidelity": {
			ok:       issueOpsSkillRoutingFidelityComplete(fixture, artifact),
			evidence: "recorded routing trace covers every expected skill-at-phase (recorded-trace proxy, not live-routing proof)",
			failure:  "recorded routing trace is missing an expected skill-at-phase",
		},
	}

	score := IssueOpsBenchmarkScore{OK: true, FixtureID: fixture.ID}
	for _, dimension := range issueOpsBenchmarkDimensions {
		if reason := issueOpsDimensionNotApplicable(dimension, fixture); reason != "" {
			// Honest N/A: recorded but excluded from average/minimum/Passed so a
			// fixture that does not exercise the dimension neither gains a false
			// 100 nor loses points.
			score.DimensionScores = append(score.DimensionScores, IssueOpsDimensionScore{
				Dimension:     dimension,
				Score:         0,
				Evidence:      reason,
				NotApplicable: true,
			})
			continue
		}
		check := checks[dimension]
		dimensionScore := 0.0
		evidence := check.failure
		if check.ok {
			dimensionScore = issueOpsBenchmarkMaxScore
			evidence = check.evidence
		} else {
			score.DeterministicFailures = append(score.DeterministicFailures, check.failure)
		}
		score.DimensionScores = append(score.DimensionScores, IssueOpsDimensionScore{
			Dimension: dimension,
			Score:     dimensionScore,
			Evidence:  evidence,
		})
	}
	score.AverageScore, score.MinimumScore = summarizeIssueOpsDimensionScores(score.DimensionScores)
	score.CriticalFailures = append(score.CriticalFailures, detectIssueOpsCriticalFailures(fixture, artifact)...)
	score.Passed = len(score.CriticalFailures) == 0 && len(score.DeterministicFailures) == 0 && score.MinimumScore >= issueOpsBenchmarkMaxScore
	score.OK = score.Passed
	return score
}

// issueOpsDimensionNotApplicable returns a non-empty N/A reason when a fixture
// does not exercise a metadata-conditional dimension, so the dimension is
// recorded but excluded from average/minimum/Passed. Keep every conditional
// dimension here rather than hardcoding one name in the scoring loop.
func issueOpsDimensionNotApplicable(dimension string, fixture issueopscontract.IssueOpsBenchmarkFixture) string {
	switch dimension {
	case "pioneer_skill_contribution":
		if strings.TrimSpace(fixture.PioneerSkillTarget) == "" {
			return "N/A: fixture has no pioneer_skill_target; excluded from average/minimum"
		}
	case "skill_routing_fidelity":
		if len(fixture.ExpectedRouting) == 0 {
			return "N/A: fixture has no expected_routing; excluded from average/minimum"
		}
	}
	return ""
}
