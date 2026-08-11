package issueops

import (
	"strings"

	"agent-harness/internal/contract/issueops"
	issueopsstatusdomain "agent-harness/internal/domain/issueopsstatus"
	"agent-harness/internal/domain/stringlist"
)

func issueOpsReadinessFrom(record issueops.IssueOpsRecord, missing []string) issueops.IssueOpsReadiness {
	return issueops.IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      stringlist.UniqueSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

// IssueOpsProblemReadiness는 problem phase 완료 여부를 보고한다. problem 완료는
// intent contract만 요구하도록 의도적으로 최소화한다. remote issue나 branch가
// 생기기 전의 자유로운 problem -> grill 전이와 초기 탐색을 보존하기 위해서다.
// issue_url/branch는 grill artifact다.
func IssueOpsProblemReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsReadinessFrom(record, issueOpsIntentMissing(record))
}

// IssueOpsGrillReadiness는 grill phase 완료 여부를 보고한다. 필요한 것은
// issue_url + branch + plan_prep(적용 시) + split_decision + domain_review다.
// 이는 create-issue-after-grill workflow와 현재 plan-entry gate에 맞춰 plan
// 진입을 막는다.
func IssueOpsGrillReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := []string{}
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if strings.TrimSpace(record.Branch) == "" {
		missing = append(missing, "branch")
	}
	if planPrepGateApplies(record) {
		missing = append(missing, planPrepMissing(record.PlanPrep)...)
	}
	missing = append(missing, issueOpsSplitDecisionMissing(record)...)
	missing = append(missing, issueOpsDomainReviewMissing(record)...)
	return issueOpsReadinessFrom(record, missing)
}

// issueOpsSplitDecisionMissing는 기존 필드에서 split_decision을 파생한다. child/
// splits-from issue link는 분할이 일어났음을, scope decision은 분할하지 않은
// 근거를 뜻한다. 전용 필드는 필요 없다.
func issueOpsSplitDecisionMissing(record issueops.IssueOpsRecord) []string {
	for _, link := range record.IssueLinks {
		switch strings.ToLower(strings.TrimSpace(link.Type)) {
		case "child", "splits-from":
			return nil
		}
	}
	for _, decision := range record.Decisions {
		if strings.EqualFold(strings.TrimSpace(decision.Kind), "scope") {
			return nil
		}
	}
	return []string{"split_decision"}
}

func issueOpsDomainReviewMissing(record issueops.IssueOpsRecord) []string {
	if record.DomainReview == nil || strings.TrimSpace(record.DomainReview.ReviewedAt) == "" {
		return []string{"domain_review"}
	}
	return nil
}

func issueOpsPlanCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	// plan 완료(branch_prepare + worktree + plan + design_review)는 현재
	// "compatibility-review 진입 준비 완료" gate와 정확히 같다.
	return IssueOpsCompatibilityReviewReadiness(record)
}

func issueOpsCompatibilityReviewCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsReadinessFrom(record, issueOpsCompatibilityReviewMissing(record))
}

func issueOpsImplementCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	// implement 완료는 implement-entry readiness에 종료 artifact인
	// implementation_changes를 더한다. 이는 현재 ai-slop-clean entry gate와 같다.
	return IssueOpsAISlopCleanReadiness(record)
}

func issueOpsAISlopCleanCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := []string{}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		missing = append(missing, "ai_slop_clean_at")
	}
	if strings.TrimSpace(record.AISlopCleanHead) == "" {
		missing = append(missing, "ai_slop_clean_head")
	}
	if strings.TrimSpace(record.AISlopCleanFingerprint) == "" {
		missing = append(missing, "ai_slop_clean_fingerprint")
	}
	if len(cleanIssueOpsTextValues(record.AISlopCleanCategories)) == 0 {
		missing = append(missing, "cleanup_evidence")
	}
	if len(cleanIssueOpsTextValues(record.AISlopCleanVerification)) == 0 {
		missing = append(missing, "verification_evidence")
	}
	return issueOpsReadinessFrom(record, missing)
}

func issueOpsFeedbackCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := []string{}
	for _, item := range record.Feedback {
		if strings.TrimSpace(item.Classification) == "" {
			missing = append(missing, "feedback_classification")
			break
		}
	}
	if issueOpsHasUnresolvedContractFeedback(record) {
		missing = append(missing, "contract_feedback_issue_update")
	}
	for _, item := range record.Feedback {
		if strings.TrimSpace(item.Resolution) == "" {
			missing = append(missing, "feedback_resolution")
			break
		}
	}
	return issueOpsReadinessFrom(record, missing)
}

func issueOpsPRCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	// 완료/파생에는 git fetch를 하지 않는 non-strict readiness를 사용한다. network
	// 부수효과 없이 status 표시용 ledger를 파생하기 위해서다. 실제 pr-phase entry
	// gate는 계속 IssueOpsStrictPRReadiness를 사용한다.
	ready := IssueOpsPRReadiness(record)
	missing := append([]string{}, ready.Missing...)
	if record.RemoteArtifact == nil || strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		missing = append(missing, "remote_artifact")
	}
	missing = append(missing, issueOpsTargetBranchMatchMissing(record)...)
	ready.Missing = stringlist.UniqueSorted(missing)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

// issueOpsTargetBranchMatchMissing는 remote artifact가 있지만 target branch가
// branch_prepare.base_branch와 다를 때 target_branch_match를 보고한다. 비교 입력을
// 아직 확보하지 못했으면 remote_artifact가 부재를 다루므로 조용히 넘어가며, pr
// 진입을 교착시키지 않는다.
func issueOpsTargetBranchMatchMissing(record issueops.IssueOpsRecord) []string {
	if record.RemoteArtifact == nil || record.BranchPrepare == nil {
		return nil
	}
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	if base == "" {
		return nil
	}
	if strings.TrimSpace(record.RemoteArtifact.TargetBranch) != base {
		return []string{"target_branch_match"}
	}
	return nil
}

func issueOpsDoneCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := []string{}
	if issueOpsPhaseRank(record.Phase) < issueOpsPhaseRank(IssueOpsPhasePR) {
		missing = append(missing, "prior_phase_pr")
	}
	missing = append(missing, issueOpsRemoteArtifactMissing(record)...)
	return issueOpsReadinessFrom(record, missing)
}

// IssueOpsPhaseCompletion은 기존 source-of-truth 필드에서 phase 완료 여부를
// 계산해 ready/artifacts(missing)를 반환한다. 기존 readiness 함수를 색인할 뿐,
// 스스로 source of truth가 되지는 않는다.
func IssueOpsPhaseCompletion(record issueops.IssueOpsRecord, phase issueops.IssueOpsPhase) issueops.IssueOpsReadiness {
	switch phase {
	case IssueOpsPhaseProblem:
		return IssueOpsProblemReadiness(record)
	case IssueOpsPhaseGrill:
		return IssueOpsGrillReadiness(record)
	case IssueOpsPhasePlan:
		return issueOpsPlanCompletion(record)
	case IssueOpsPhaseCompatibilityReview:
		return issueOpsCompatibilityReviewCompletion(record)
	case IssueOpsPhaseImplement:
		return issueOpsImplementCompletion(record)
	case IssueOpsPhaseAISlopClean:
		return issueOpsAISlopCleanCompletion(record)
	case IssueOpsPhaseFeedback:
		return issueOpsFeedbackCompletion(record)
	case IssueOpsPhasePR:
		return issueOpsPRCompletion(record)
	case IssueOpsPhaseDone:
		return issueOpsDoneCompletion(record)
	default:
		return issueops.IssueOpsReadiness{OK: true, Ready: false, Missing: []string{"unknown_phase"}}
	}
}

// stampIssueOpsForwardTransition은 관찰한 phase 전이를 ledger에 기록한다(rule
// 4/5/11). 떠나는 phase는 완료로 표시한다. 성공한 forward transition은 이전 phase
// 완료를 요구하는 새 phase의 entry gate를 이미 통과했음을 뜻한다. 진입하는 phase에는
// 실제 entered_at을 쓴다. timestamp는 derived sentinel이 아니라 관찰한 `now`다.
// 실제 phase-change 지점에서만 호출하므로 artifact만 기록한 record에 ledger를 추가해
// golden을 흔들지 않는다.
func stampIssueOpsForwardTransition(ledger issueops.IssueOpsPhaseLedger, prevPhase, newPhase issueops.IssueOpsPhase, now string) issueops.IssueOpsPhaseLedger {
	if ledger == nil {
		ledger = issueops.IssueOpsPhaseLedger{}
	}
	prev := ledger[prevPhase]
	prev.Phase = prevPhase
	if prev.EnteredAt == "" {
		prev.EnteredAt = now
	}
	prev.CompletedAt = now
	prev.Artifacts = issueopsstatusdomain.ArtifactKeys(prevPhase)
	prev.Missing = nil
	// 이 phase가 실제로 다시 완료된 forward transition은
	// RegressIssueOpsForReplan이 남긴 stale-regression mark를 지운다. 정상적으로
	// 재완료된 phase는 더 이상 stale이 아니다.
	prev.Notes = clearStaleLedgerNotes(prev.Notes)
	ledger[prevPhase] = prev

	entry := ledger[newPhase]
	entry.Phase = newPhase
	if entry.EnteredAt == "" {
		entry.EnteredAt = now
	}
	ledger[newPhase] = entry
	return ledger
}

// clearStaleLedgerNotes는 ledger entry의 note에서 stale-regression marker
// (markIssueOpsLedgerStale 참조)를 제거한다. 이전에 regress된 phase가 forward
// transition으로 다시 완료되면, 더 이상 유효하지 않은 stale mark가 status에 계속
// 남지 않게 호출한다.
func clearStaleLedgerNotes(notes []string) []string {
	if len(notes) == 0 {
		return notes
	}
	kept := make([]string, 0, len(notes))
	for _, n := range notes {
		if strings.HasPrefix(n, "stale:") {
			continue
		}
		kept = append(kept, n)
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}
