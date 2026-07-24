package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/implementation"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/policy"
)

// IssueOpsImplementationReviewRequest는 구현 diff에 대한 brooks 리뷰 기록이다.
type IssueOpsImplementationReviewRequest struct {
	Verdict        string
	Findings       []string
	Evidence       []string
	ReviewerHost   string
	ReviewerModel  string
	ReviewerEffort string
}

// RecordIssueOpsImplementationReview는 verdict와 실질 내용(findings/evidence
// 각 1개 이상)을 요구한다. reviewer_* 필드는 감사 기록으로만 저장한다.
func RecordIssueOpsImplementationReview(stateRoot, id string, req IssueOpsImplementationReviewRequest) (IssueOpsRecord, error) {
	verdict := strings.ToLower(strings.TrimSpace(req.Verdict))
	if verdict != "pass" && verdict != "revise" && verdict != "stop" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("implementation review verdict must be pass|revise|stop")
	}
	findings := cleanReviewValues(req.Findings)
	evidence := cleanReviewValues(req.Evidence)
	if len(findings) == 0 || len(evidence) == 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("implementation review requires at least one finding and one evidence entry")
	}
	var record IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, e := ReadIssueOps(stateRoot, id)
		if e != nil {
			return e
		}
		// 기록 시점 하한: 구현 diff가 존재할 수 있는 phase에서만 의미가 있다
		// (C4b-F2 — F1의 fingerprint 바인딩과 이중 방어).
		if issueOpsPhaseRank(rec.Phase) < issueOpsPhaseRank(model.IssueOpsPhaseImplement) {
			return fmt.Errorf("implementation review can only be recorded from the implement phase onward (current: %s)", rec.Phase)
		}
		// 리뷰 대상 바인딩: 현재 변경 집합의 content fingerprint를 봉인한다.
		// 이후 diff가 바뀌면 게이트가 stale로 거부한다(C4b-F1).
		fingerprint := implementation.ChangeFingerprint(rec)
		if fingerprint == "" {
			return fmt.Errorf("implementation review requires a reviewable change set (no change fingerprint could be computed)")
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		rec.ImplementationReview = &model.IssueOpsImplementationReview{
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
		return IssueOpsRecord{OK: false}, err
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

// implementationReviewMissing은 orca 모드 사이클의 publication 게이트 판정이다.
// direct 모드는 게이트 대상이 아니다(빈 문자열 반환). currentFingerprint가
// 비어 있지 않으면 리뷰가 봉인한 fingerprint와 비교해 stale 리뷰를 거부한다.
func implementationReviewMissing(record IssueOpsRecord, currentFingerprint string) string {
	if record.Execution == nil || record.Execution.Mode != model.ExecutionModeOrca {
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
