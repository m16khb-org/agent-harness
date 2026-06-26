package compatibilityreview

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/policy"
)

type Store struct {
	Read       func(string, string) (model.IssueOpsRecord, error)
	TouchWrite func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Ready      func(model.IssueOpsRecord) model.IssueOpsReadiness
	PhaseRank  func(model.IssueOpsPhase) int
}

func Record(store Store, stateRoot, id string, req model.IssueOpsCompatibilityReviewRequest) (model.IssueOpsRecord, error) {
	review, err := Validate(req)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if ready := store.Ready(record); !ready.Ready {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot record compatibility review before plan/worktree readiness: missing %s", strings.Join(ready.Missing, ", "))
	}
	record.CompatibilityReview = &review
	if store.PhaseRank(record.Phase) < store.PhaseRank(model.IssueOpsPhaseCompatibilityReview) {
		record.Phase = model.IssueOpsPhaseCompatibilityReview
	}
	return store.TouchWrite(stateRoot, record)
}

func Validate(req model.IssueOpsCompatibilityReviewRequest) (model.IssueOpsCompatibilityReview, error) {
	backwardCompatibility, err := cleanRequiredList("backward_compatibility", req.BackwardCompatibility)
	if err != nil {
		return model.IssueOpsCompatibilityReview{}, err
	}
	sideEffects, err := cleanRequiredList("side_effects", req.SideEffects)
	if err != nil {
		return model.IssueOpsCompatibilityReview{}, err
	}
	verification, err := cleanRequiredList("verification", req.Verification)
	if err != nil {
		return model.IssueOpsCompatibilityReview{}, err
	}
	rollbackPlan := strings.TrimSpace(req.RollbackPlan)
	if rollbackPlan == "" {
		return model.IssueOpsCompatibilityReview{}, fmt.Errorf("rollback_plan is required")
	}
	blockers := cleanList(req.Blockers)
	if req.Approved && len(blockers) > 0 {
		return model.IssueOpsCompatibilityReview{}, fmt.Errorf("approved compatibility review must not have blockers")
	}
	return model.IssueOpsCompatibilityReview{
		BackwardCompatibility: backwardCompatibility,
		SideEffects:           sideEffects,
		RollbackPlan:          policy.RedactFreeform(rollbackPlan),
		Verification:          verification,
		Blockers:              blockers,
		Approved:              req.Approved,
		ReviewedAt:            time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func cleanRequiredList(field string, values []string) ([]string, error) {
	out := cleanList(values)
	if len(out) == 0 {
		return nil, fmt.Errorf("%s requires at least one entry", field)
	}
	return out, nil
}

func cleanList(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
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
