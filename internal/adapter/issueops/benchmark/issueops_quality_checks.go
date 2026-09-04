package benchmark

import issueopscontract "issueops/internal/contract/issueops"

import "strings"

func issueOpsLabelDecisionEvidenceComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	text := artifact.IssueDraft + "\n" + artifact.ProblemSummary
	hasDecision := containsAllFold(text, "selected label", "rejected label") ||
		containsAllFold(text, "선택 라벨", "거절 라벨")
	return hasDecision &&
		containsAnyFold(text, "threshold", "score", "임계값", "점수") &&
		containsAnyFold(text, "manual override", "수동 override", "수동 판단")
}

func issueOpsHierarchyEvidenceComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	text := artifact.TaskBreakdown + "\n" + artifact.Plan
	hasChildWork := containsAnyFold(text, "sub-issue", "subissue", "child item", "child work item") &&
		containsAnyFold(text, "link-child", "provider-native", "github sub-issue", "gitlab child")
	hasNonSplitReason := containsAnyFold(text, "non-split reason", "explicit non-split reason", "분리하지 않는 이유", "비분할 사유")
	hasLargeUnsafeRationale := containsAnyFold(text,
		"one issue would be unsafe",
		"unsafe as one work item",
		"single issue would hide",
		"single mr would hide",
		"large issue is unsafe",
		"umbrella issue",
	)
	hasCollaborationRationale := containsAnyFold(text,
		"explicitly requested for collaboration",
		"collaboration requested",
		"multiple owners",
		"parallel ownership",
	)
	return hasNonSplitReason || (hasChildWork && (hasLargeUnsafeRationale || hasCollaborationRationale) && issueOpsChildTaskDependencyClassificationComplete(text))
}

func issueOpsChildTaskDependencyClassificationComplete(text string) bool {
	lower := strings.ToLower(text)
	// Parallelizable-by-default policy: [p] is mandatory for every split.
	// [s] is reserved for genuinely unavoidable sequential dependencies, so an
	// all-parallelizable split with only [p] markers is valid classification.
	if !strings.Contains(lower, "[p]") {
		return false
	}
	hasSequentialMarker := strings.Contains(lower, "[s]")
	if !containsAnyFold(text,
		"parallelizable",
		"parallel-ready",
		"can run in parallel",
		"parallel execution",
		"parallel wave",
		"병렬 가능",
		"병렬 처리",
		"병렬 실행",
	) {
		return false
	}
	if !containsAnyFold(text,
		"depends on",
		"dependency",
		"prerequisite",
		"requires",
		"blocks",
		"의존",
		"선행",
		"전제",
	) {
		return false
	}
	if !containsAnyFold(text,
		"execution order",
		"execution wave",
		"wave 1",
		"wave 2",
		"recommended execution order",
		"실행 순서",
		"실행 웨이브",
		"권장 순서",
	) {
		return false
	}
	// Sequential evidence is required only when [s] children exist. An
	// all-parallelizable split has no sequential dependency to document.
	if !hasSequentialMarker {
		return true
	}
	return containsAnyFold(text,
		"sequential",
		"must run after",
		"ordered task",
		"serial task",
		"순차",
		"선행 작업 이후",
	)
}

func issueOpsDraftIssueCompletionEvidenceComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.CompletionHygiene, "draft issue completion record", "final diff", "evidence", "labels", "children", "unresolved follow-up") &&
		containsAnyFold(artifact.CompletionHygiene, "pr url", "mr url")
}

func issueOpsReviewAgentThreadEvidenceComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	text := artifact.ReviewFeedbackEvidence
	return containsAnyFold(text, "kodus", "gemini code assist", "review-agent") &&
		containsAllFold(text, "thread reply", "resolution") &&
		containsAnyFold(text, "resolveReviewThread", "resolved=true")
}

func detectIssueOpsQualityCriticalFailures(fixture issueopscontract.IssueOpsBenchmarkFixture, artifact issueopscontract.IssueOpsBenchmarkArtifact) []string {
	var failures []string
	for _, rule := range fixture.CriticalFailures {
		ruleText := strings.ToLower(rule)
		switch {
		case strings.Contains(ruleText, "intelligent label") && !issueOpsLabelDecisionEvidenceComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "large issue hierarchy") && !issueOpsHierarchyEvidenceComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "draft issue completion") && !issueOpsDraftIssueCompletionEvidenceComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "review-agent threads") && !issueOpsReviewAgentThreadEvidenceComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "skips pioneer method") && !issueOpsPioneerSkillEvidenceComplete(fixture, artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "skips expected routing") && len(fixture.ExpectedRouting) > 0 && !issueOpsSkillRoutingFidelityComplete(fixture, artifact):
			failures = append(failures, rule)
		}
	}
	return failures
}
