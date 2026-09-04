package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/contract/issueops"
	"issueops/internal/domain/policy"
)

// RecordIssueOpsImplementationReview는 verdict와 실질 내용(findings/evidence
// 각 1개 이상)을 요구한다. reviewer_* 필드는 감사 기록으로만 저장한다.
func RecordIssueOpsImplementationReview(stateRoot, id string, req IssueOpsImplementationReviewRequest) (issueops.IssueOpsRecord, error) {
	verdict := strings.ToLower(strings.TrimSpace(req.Verdict))
	if verdict != "pass" && verdict != "revise" && verdict != "stop" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("implementation review verdict must be pass|revise|stop")
	}
	findings := cleanReviewValues(req.Findings)
	evidence := cleanReviewValues(req.Evidence)
	if len(findings) == 0 || len(evidence) == 0 {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("implementation review requires at least one finding and one evidence entry")
	}
	var record issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, e := ReadIssueOps(stateRoot, id)
		if e != nil {
			return e
		}
		// 기록 시점 하한: 구현 diff가 존재할 수 있는 phase에서만 의미가 있다
		// (C4b-F2 — F1의 fingerprint 바인딩과 이중 방어).
		if issueOpsPhaseRank(rec.Phase) < issueOpsPhaseRank(issueops.IssueOpsPhaseImplement) {
			return fmt.Errorf("implementation review can only be recorded from the implement phase onward (current: %s)", rec.Phase)
		}
		// 리뷰 대상 바인딩: 현재 변경 집합의 content fingerprint를 봉인한다.
		// 이후 diff가 바뀌면 게이트가 stale로 거부한다(C4b-F1).
		//
		// fingerprint를 계산할 수 없는 사이클(비-git worktree 등)도 판정 자체는
		// 기록할 수 있다 — project_docs_review·ai_slop_clean과 같은 관용이다.
		// 게이트가 orca 한정이던 동안에는 실 worktree가 늘 있어 이 경우가 없었지만,
		// 모든 모드로 넓힌 뒤에는 거부가 곧 탈출구 없는 교착이 된다. 빈 채로
		// 봉인하면 나중에 fingerprint가 생겼을 때 stale로 잡혀 재기록을 요구하므로
		// 안전성은 유지된다.
		fingerprint := implementation.ChangeFingerprint(rec)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		rec.ImplementationReview = &issueops.IssueOpsImplementationReview{
			Verdict: verdict, Findings: findings, Evidence: evidence,
			ReviewedFingerprint: fingerprint,
			ReviewerHost:        strings.ToLower(strings.TrimSpace(req.ReviewerHost)),
			ReviewerModel:       strings.TrimSpace(req.ReviewerModel),
			ReviewerEffort:      strings.TrimSpace(req.ReviewerEffort),
			RecordedAt:          now,
		}
		rec.UpdatedAt = now
		record, e = writeIssueOps(stateRoot, rec)
		return e
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	return record, nil
}

func cleanReviewValues(values []string) []string {
	out := []string{}
	for _, v := range values {
		v = policy.RedactFreeform(strings.TrimSpace(v))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// implementationReviewMissing은 publication 게이트 판정이며 execution이 있는
// 모든 모드에 적용한다. execution이 없는 레코드(준비 전, legacy)만 면제다.
//
// 원래는 orca 모드 한정이었다. 하위 세션을 하네스가 띄우는 그 경로에서만
// 적대 리뷰를 강제할 수 있다고 봤기 때문이다. 9단계 재편(2026-09-04)에서
// direct가 기본 경로가 되고 검증 단계가 이 기록을 만들면서 전제가 바뀌었다.
// orca 한정으로 두면 기본 경로의 리뷰 게이트가 CLI 수준에서 비어 버린다.
//
// currentFingerprint가 비어 있지 않으면 리뷰가 봉인한 fingerprint와 비교해
// stale 리뷰를 거부한다.
func implementationReviewMissing(record issueops.IssueOpsRecord, currentFingerprint string) string {
	if record.Execution == nil {
		return ""
	}
	review := record.ImplementationReview
	if review == nil {
		return "implementation_review"
	}
	if review.Verdict != "pass" {
		return "implementation_review_verdict_" + review.Verdict
	}
	if currentFingerprint != "" && review.ReviewedFingerprint != currentFingerprint {
		return "implementation_review_stale"
	}
	return ""
}
