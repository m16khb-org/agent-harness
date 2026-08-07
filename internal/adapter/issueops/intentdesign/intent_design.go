package intentdesign

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"agent-harness/internal/adapter/policy"
	model "agent-harness/internal/contract/issueops"
)

type Store struct {
	Read          func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite    func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	PlanReadiness func(record model.IssueOpsRecord) model.IssueOpsReadiness
}

const (
	DesignReviewEvidenceExample  = "design review checked alternatives and risks"
	designReviewEvidenceGuidance = `approved design review requires design_review_evidence: this is not a separate flag or decision record; add --verification "design review checked alternatives and risks" or a Korean equivalent such as "설계 검토 완료: 대안과 위험 확인"`
)

func RecordIntent(store Store, stateRoot, id string, req model.IssueOpsIntentRecordRequest) (model.IssueOpsRecord, error) {
	rawRequest := strings.TrimSpace(req.RawRequest)
	if rawRequest == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("raw_request is required")
	}
	interpretedIntent := strings.TrimSpace(req.InterpretedIntent)
	if interpretedIntent == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("interpreted_intent is required")
	}
	if interpretedIntent == rawRequest {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("interpreted_intent must differ from raw_request")
	}
	if !materiallyDifferentIntent(rawRequest, interpretedIntent) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("interpreted_intent must materially differ from raw_request")
	}
	successCriteria := CleanTextValues(req.SuccessCriteria)
	if len(successCriteria) == 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("success_criteria is required")
	}
	intentClass, err := model.NormalizeIntentClass(req.IntentClass)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.Intent = &model.IssueOpsIntentContract{
		RawRequest:        policy.RedactFreeform(rawRequest),
		InterpretedIntent: policy.RedactFreeform(interpretedIntent),
		SuccessCriteria:   successCriteria,
		Constraints:       CleanTextValues(req.Constraints),
		Ambiguities:       CleanTextValues(req.Ambiguities),
		NonGoals:          CleanTextValues(req.NonGoals),
		IntentClass:       intentClass,
		RecordedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	return store.TouchWrite(stateRoot, record)
}

func RecordDesignReview(store Store, stateRoot, id string, req model.IssueOpsDesignReviewRequest) (model.IssueOpsRecord, error) {
	problemSummary := strings.TrimSpace(req.ProblemSummary)
	if problemSummary == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("problem_summary is required")
	}
	proposedDesign := strings.TrimSpace(req.ProposedDesign)
	if proposedDesign == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("proposed_design is required")
	}
	verification := CleanTextValues(req.Verification)
	if len(verification) == 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("verification is required")
	}
	openQuestions := CleanTextValues(req.OpenQuestions)
	if req.Approved && len(openQuestions) > 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("approved design review must not have open_questions")
	}
	refactorPlan := strings.TrimSpace(req.RefactorPlan)
	alternatives := CleanTextValues(req.Alternatives)
	risks := CleanTextValues(req.Risks)
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	// Design review only requires the intent contract (and issue link). The
	// plan-prep evidence gate is enforced at plan-phase entry, not here: design
	// review happens inside the plan phase, by which point plan-prep is already
	// satisfied. Ignore plan_prep_* missing keys so the design-review prerequisite
	// stays "intent contract exists".
	if ready := store.PlanReadiness(record); !ready.Ready {
		if blocking := nonPlanPrepMissing(ready.Missing); len(blocking) > 0 {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot record design review before intent contract: missing %s", strings.Join(blocking, ", "))
		}
	}
	if req.Approved && refactorPlan == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("approved design review requires refactor_plan")
	}
	if req.Approved && len(alternatives) == 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("approved design review requires alternatives")
	}
	if req.Approved && len(risks) == 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("approved design review requires risks")
	}
	if req.Approved && !HasDesignReviewEvidence(verification) {
		return model.IssueOpsRecord{OK: false}, errors.New(designReviewEvidenceGuidance)
	}
	record.DesignReview = &model.IssueOpsDesignReview{
		ProblemSummary: policy.RedactFreeform(problemSummary),
		ProposedDesign: policy.RedactFreeform(proposedDesign),
		RefactorPlan:   policy.RedactFreeform(refactorPlan),
		Alternatives:   alternatives,
		Risks:          risks,
		Verification:   verification,
		OpenQuestions:  openQuestions,
		Approved:       req.Approved,
		ReviewedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	return store.TouchWrite(stateRoot, record)
}

func nonPlanPrepMissing(missing []string) []string {
	out := []string{}
	for _, m := range missing {
		if !strings.HasPrefix(m, "plan_prep_") {
			out = append(out, m)
		}
	}
	return out
}

func HasDesignReviewEvidence(values []string) bool {
	for _, value := range values {
		text := strings.ToLower(strings.TrimSpace(value))
		if text == "" {
			continue
		}
		if strings.Contains(text, "design") && (strings.Contains(text, "review") || strings.Contains(text, "audit") || strings.Contains(text, "evaluat")) {
			return true
		}
		if strings.Contains(text, "설계") && (strings.Contains(text, "검수") || strings.Contains(text, "검토")) {
			return true
		}
	}
	return false
}

func materiallyDifferentIntent(rawRequest, interpretedIntent string) bool {
	rawTokens := intentTokenSet(rawRequest)
	interpretedTokens := intentTokenSet(interpretedIntent)
	if len(rawTokens) < 4 || len(interpretedTokens) < 4 {
		return true
	}
	shared := 0
	for token := range rawTokens {
		if interpretedTokens[token] {
			shared++
		}
	}
	union := len(rawTokens) + len(interpretedTokens) - shared
	return union == 0 || float64(shared)/float64(union) < 0.85
}

func intentTokenSet(text string) map[string]bool {
	out := map[string]bool{}
	for _, token := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if token == "" || intentStopWord(token) {
			continue
		}
		out[token] = true
	}
	return out
}

func intentStopWord(token string) bool {
	switch token {
	case "a", "an", "the", "please", "좀", "해주세요":
		return true
	default:
		return false
	}
}

func CleanTextValues(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "\x00") || seen[value] {
			continue
		}
		value = policy.RedactFreeform(value)
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
