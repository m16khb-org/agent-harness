package devilsadvocate

import (
	"fmt"
	"strings"
	"time"

	model "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/policy"
)

// Store is the persistence seam, mirroring the compatibility-review recorder.
type Store struct {
	Read       func(string, string) (model.IssueOpsRecord, error)
	TouchWrite func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
}

var validVerdicts = map[string]bool{"pass": true, "revise": true, "stop": true}

// Record validates the request and persists it as the record's
// DevilsAdvocateReview. It does not gate on readiness: the devil's advocate runs
// on the completed plan, and the fail-closed gate lives at implement entry.
func Record(store Store, stateRoot, id string, req model.IssueOpsDevilsAdvocateReviewRequest) (model.IssueOpsRecord, error) {
	review, err := Validate(req)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.DevilsAdvocateReview = &review
	return store.TouchWrite(stateRoot, record)
}

// Validate normalizes and checks a devil's-advocate request:
//   - verdict must be pass | revise | stop
//   - a stop/revise verdict needs concrete findings OR an explicit waiver
//   - a waiver needs a rationale
func Validate(req model.IssueOpsDevilsAdvocateReviewRequest) (model.IssueOpsDevilsAdvocateReview, error) {
	verdict := strings.ToLower(strings.TrimSpace(req.Verdict))
	if !validVerdicts[verdict] {
		return model.IssueOpsDevilsAdvocateReview{}, fmt.Errorf("verdict must be pass, revise, or stop")
	}
	findings := cleanList(req.Findings)
	rationale := strings.TrimSpace(req.WaiverRationale)
	if req.Waived && rationale == "" {
		return model.IssueOpsDevilsAdvocateReview{}, fmt.Errorf("waived review requires waiver_rationale")
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
