package benchmark

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	"os"
	"strings"
)

func detectIssueOpsCriticalFailures(fixture issueopscontract.IssueOpsBenchmarkFixture, artifact issueopscontract.IssueOpsBenchmarkArtifact) []string {
	var failures []string
	for _, rule := range fixture.CriticalFailures {
		ruleText := strings.ToLower(rule)
		switch {
		case strings.Contains(ruleText, "works in source repo") && !implementationInWorktree(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "skips branch prompt") && strings.TrimSpace(artifact.BranchName) == "":
			failures = append(failures, rule)
		case strings.Contains(ruleText, "worker starts without context check") && !workerPromptHasContextGate(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "unbounded code-reviewer") && !reviewPromptIsBounded(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "removes dirty worktree") && containsAllFold(artifact.WorktreeCleanup, "dirty", "remove"):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "not written in korean") && (!containsHangul(artifact.IssueDraft) || !containsHangul(artifact.PRDraft)):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "missing issue/pr guideline reference") && !hasIssueOpsGuidelineRef(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "excessive emoji") && hasExcessiveEmoji(artifact.IssueDraft+"\n"+artifact.PRDraft):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "domain contract") && !issueOpsDomainContractEvidenceComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "api doc") && !issueOpsAPIDocGateEvidenceComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "live evidence") && !issueOpsLiveEvidenceMatrixComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "review feedback") && !strings.Contains(ruleText, "review-agent threads") && !issueOpsReviewFeedbackEvidenceComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "completion hygiene") && !issueOpsCompletionHygieneComplete(artifact):
			failures = append(failures, rule)
		}
	}
	return append(failures, detectIssueOpsQualityCriticalFailures(fixture, artifact)...)
}

func implementationInWorktree(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	worktreePath := strings.TrimSpace(artifact.WorktreePath)
	location := strings.TrimSpace(artifact.ImplementationLocation)
	return worktreePath != "" && location != "" && (location == worktreePath || strings.HasPrefix(location, worktreePath+string(os.PathSeparator)))
}

func issueOpsDomainContractEvidenceComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.DomainContractEvidence, "invariant", "exact mechanism", "equivalent behavior", "source") &&
		containsAnyFold(artifact.DomainContractEvidence, "file:", "line", ":")
}

func issueOpsAPIDocGateEvidenceComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.APIDocGateEvidence, "changed endpoint", "public error", "static check", "review") &&
		containsAnyFold(artifact.APIDocGateEvidence, "openapi", "swagger", "api-doc", "api doc")
}

func issueOpsLiveEvidenceMatrixComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.LiveEvidenceMatrix, "environment", "repo config", "runtime", "remediation order") &&
		containsAnyFold(artifact.LiveEvidenceMatrix, "dev", "stg", "prod", "local", "production")
}

func issueOpsReviewFeedbackEvidenceComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.ReviewFeedbackEvidence, "classification", "verification", "thread reply", "resolution") &&
		containsAnyFold(artifact.ReviewFeedbackEvidence, "valid", "stale", "noise", "contract_change", "defect") &&
		issueOpsReviewAgentThreadEvidenceComplete(artifact)
}

func issueOpsCompletionHygieneComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.CompletionHygiene, "final diff", "target branch", "remote artifact", "single commit", "cleanup") &&
		containsAnyFold(artifact.CompletionHygiene, "pr", "mr", "issue") &&
		issueOpsDraftIssueCompletionEvidenceComplete(artifact)
}

func workerPromptHasContextGate(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	prompt := artifact.SubagentPrompts
	return containsAllFold(prompt, "pwd", "branch", "head") &&
		(containsFold(prompt, "worktree") || strings.TrimSpace(artifact.WorktreePath) != "") &&
		containsAnyFold(prompt, "stop", "halt", "중단")
}

func reviewPromptIsBounded(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	prompt := artifact.SubagentPrompts
	if !containsAnyFold(prompt, "review", "code-reviewer", "verifier") {
		return true
	}
	if containsAnyFold(prompt, "verifier", "direct bounded review", "bounded review") {
		return true
	}
	return containsAnyFold(prompt, "do not spawn subagents", "no nested subagents", "nested subagent fan-out 금지") &&
		containsAnyFold(prompt, "minute", "minutes", "time budget", "분", "시간 예산")
}

var issueOpsIssueSectionConcepts = [][]string{
	{"problem", "문제"},
	{"current evidence", "근거", "현재 상태", "현재 증거"},
	{"related issue", "label", "관련 이슈", "라벨 판단"},
	{"acceptance criteria", "수용 기준", "완료 기준", "인수 기준"},
	{"non-goals", "비목표", "비-목표"},
	{"implementation scope", "구현 범위"},
	{"verification", "검증"},
	{"risk", "tradeoff", "위험", "트레이드오프"},
	{"feedback log", "피드백 로그", "피드백 기록"},
}

var issueOpsPRSectionConcepts = [][]string{
	{"intent", "의도"},
	{"issue", "이슈"},
	{"type", "change type", "변경 유형"},
	{"changes", "변경"},
	{"verification", "검증"},
	{"reviewer focus", "reviewer notes", "리뷰어 초점", "리뷰어 노트", "리뷰 노트", "리뷰어 참고"},
	{"risk", "rollback", "위험", "리스크"},
	{"breaking changes", "breaking change", "브레이킹 변경", "호환성 영향"},
	{"user impact", "release note", "사용자 영향", "릴리즈 노트"},
	{"documentation", "migration", "문서", "마이그레이션"},
	{"scope", "범위 관리"},
	{"worktree cleanup", "워크트리 정리", "cleanup status"},
	{"automation", "AI", "자동화", "AI 개입"},
}

func hasIssueOpsGuidelineRef(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	const guideline = "docs/superpowers/specs/issueops-issue-pr-guidelines.md"
	return containsFold(artifact.GuidelineRef, guideline) ||
		containsFold(artifact.IssueDraft, guideline) ||
		containsFold(artifact.PRDraft, guideline)
}

func hasExcessiveEmoji(s string) bool {
	count := 0
	for _, r := range s {
		if isEmojiRune(r) {
			count++
		}
	}
	return count > 3
}

func isEmojiRune(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1FAFF) ||
		(r >= 0x2600 && r <= 0x27BF)
}
