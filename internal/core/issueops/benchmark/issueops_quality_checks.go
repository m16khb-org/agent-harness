package benchmark

import "strings"

func issueOpsLabelDecisionEvidenceComplete(artifact IssueOpsBenchmarkArtifact) bool {
	text := artifact.IssueDraft + "\n" + artifact.ProblemSummary
	hasDecision := containsAllFold(text, "selected label", "rejected label") ||
		containsAllFold(text, "선택 라벨", "거절 라벨")
	return hasDecision &&
		containsAnyFold(text, "threshold", "score", "임계값", "점수") &&
		containsAnyFold(text, "manual override", "수동 override", "수동 판단")
}

func issueOpsHierarchyEvidenceComplete(artifact IssueOpsBenchmarkArtifact) bool {
	text := artifact.TaskBreakdown + "\n" + artifact.Plan
	hasChildWork := containsAnyFold(text, "sub-issue", "subissue", "child item", "child work item") &&
		containsAnyFold(text, "link-child", "provider-native", "github sub-issue", "gitlab child")
	hasNonSplitReason := containsAnyFold(text, "non-split reason", "explicit non-split reason", "분리하지 않는 이유", "비분할 사유")
	return hasChildWork || hasNonSplitReason
}

func issueOpsDraftIssueCompletionEvidenceComplete(artifact IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.CompletionHygiene, "draft issue completion record", "final diff", "evidence", "labels", "children", "unresolved follow-up") &&
		containsAnyFold(artifact.CompletionHygiene, "pr url", "mr url")
}

func issueOpsReviewAgentThreadEvidenceComplete(artifact IssueOpsBenchmarkArtifact) bool {
	text := artifact.ReviewFeedbackEvidence
	return containsAnyFold(text, "kodus", "gemini code assist", "review-agent") &&
		containsAllFold(text, "thread reply", "resolution") &&
		containsAnyFold(text, "resolveReviewThread", "resolved=true")
}

func detectIssueOpsQualityCriticalFailures(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) []string {
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
		}
	}
	return failures
}
