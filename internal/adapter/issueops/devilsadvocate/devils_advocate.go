package devilsadvocate

import (
	"fmt"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/policy"
)

// Store is the persistence seam, mirroring the compatibility-review recorder.
// PlanDigest resolves the sha256 of the plan the cycle will implement (the
// linked plan file, or the staged plan artifact when no file is linked) so the
// verdict is bound to the plan content it reviewed. It fails when the cycle has
// no plan yet — a devil's advocate without a plan has nothing to review.
type Store struct {
	Read       func(string, string) (model.IssueOpsRecord, error)
	TouchWrite func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	PlanDigest func(stateRoot string, record model.IssueOpsRecord) (string, error)
}

var validVerdicts = map[string]bool{"pass": true, "revise": true, "stop": true}

var validReviewerContexts = map[string]bool{"subagent": true, "inline": true}

// Record validates the request and persists it as the record's
// DevilsAdvocateReview, keeping the earlier rounds of this plan phase in
// History. It does not gate on readiness: the devil's advocate runs on the
// completed plan, and the fail-closed gate lives at implement entry.
func Record(store Store, stateRoot, id string, req model.IssueOpsDevilsAdvocateReviewRequest) (model.IssueOpsRecord, error) {
	review, err := Validate(req)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if store.PlanDigest == nil {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan digest resolver is unavailable")
	}
	digest, err := store.PlanDigest(stateRoot, record)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	review.ReviewedPlanDigest = digest
	if prev := record.DevilsAdvocateReview; prev != nil {
		review.History = append(append([]model.IssueOpsDevilsAdvocateRound{}, prev.History...), roundOf(*prev))
	}
	record.DevilsAdvocateReview = &review
	return store.TouchWrite(stateRoot, record)
}

func roundOf(review model.IssueOpsDevilsAdvocateReview) model.IssueOpsDevilsAdvocateRound {
	return model.IssueOpsDevilsAdvocateRound{
		Verdict:            review.Verdict,
		Findings:           review.Findings,
		Waived:             review.Waived,
		WaiverRationale:    review.WaiverRationale,
		ReviewerContext:    review.ReviewerContext,
		ReviewedPlanDigest: review.ReviewedPlanDigest,
		RecordedAt:         review.RecordedAt,
	}
}

// Validate normalizes and checks a devil's-advocate request:
//   - verdict must be pass | revise | stop
//   - reviewer_context must be subagent | inline (audit field, always recorded)
//   - a pass needs at least one finding (what was attacked and why it failed)
//   - a stop/revise verdict needs concrete findings OR an explicit waiver
//   - a waiver needs a rationale
func Validate(req model.IssueOpsDevilsAdvocateReviewRequest) (model.IssueOpsDevilsAdvocateReview, error) {
	verdict := strings.ToLower(strings.TrimSpace(req.Verdict))
	if !validVerdicts[verdict] {
		return model.IssueOpsDevilsAdvocateReview{}, fmt.Errorf("verdict must be pass, revise, or stop")
	}
	reviewerContext := strings.ToLower(strings.TrimSpace(req.ReviewerContext))
	if !validReviewerContexts[reviewerContext] {
		return model.IssueOpsDevilsAdvocateReview{}, fmt.Errorf("reviewer_context must be subagent or inline")
	}
	findings := cleanList(req.Findings)
	rationale := strings.TrimSpace(req.WaiverRationale)
	if req.Waived && rationale == "" {
		return model.IssueOpsDevilsAdvocateReview{}, fmt.Errorf("waived review requires waiver_rationale")
	}
	if verdict == "pass" && len(findings) == 0 {
		return model.IssueOpsDevilsAdvocateReview{}, fmt.Errorf("pass verdict requires at least one finding (what was attacked and why it failed)")
	}
	if (verdict == "stop" || verdict == "revise") && !req.Waived && len(findings) == 0 {
		return model.IssueOpsDevilsAdvocateReview{}, fmt.Errorf("%s verdict requires findings or an explicit waiver", verdict)
	}
	return model.IssueOpsDevilsAdvocateReview{
		Verdict:         verdict,
		Findings:        findings,
		Waived:          req.Waived,
		WaiverRationale: policy.RedactFreeform(rationale),
		ReviewerPattern: "devils-advocate-review",
		ReviewerContext: reviewerContext,
		RecordedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func cleanList(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		v = policy.RedactFreeform(v)
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
