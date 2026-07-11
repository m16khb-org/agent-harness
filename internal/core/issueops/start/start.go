package start

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
)

type Store struct {
	Read           func(stateRoot, id string) (model.IssueOpsRecord, error)
	Write          func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	NewID          func(repo, branch string) string
	ValidateBranch func(branch string) error
	// WorktreeValid reports whether a worktree path still exists on disk. When
	// nil, stale-worktree reset is disabled (fail-open); production wiring must
	// always set it (see package.go) so abandoned worktree cycles are reset.
	WorktreeValid func(path string) bool
}

func Start(store Store, stateRoot string, req model.IssueOpsStartRequest) (model.IssueOpsRecord, error) {
	rawRepo := strings.TrimSpace(req.Repo)
	if rawRepo == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("repo is required")
	}
	repo := rawRepo
	if absRepo, err := filepath.Abs(rawRepo); err == nil {
		repo = absRepo
	}
	branch := strings.TrimSpace(req.Branch)
	if err := store.ValidateBranch(branch); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	id := store.NewID(repo, branch)
	// Locking: package.go StartIssueOps wraps this call in withIssueOpsLock,
	// serializing concurrent Start/set-phase/link calls on the same state root
	// across processes (sqlstore span: in-process mutex + sqlite write lock).
	if existing, err := store.Read(stateRoot, id); err == nil {
		return resumeOrReset(store, stateRoot, existing)
	}
	if legacyID := store.NewID(rawRepo, branch); legacyID != id {
		if existing, err := store.Read(stateRoot, legacyID); err == nil {
			return resumeOrReset(store, stateRoot, existing)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := model.IssueOpsRecord{
		OK:        true,
		ID:        id,
		Repo:      repo,
		Branch:    branch,
		Phase:     model.IssueOpsPhaseProblem,
		Feedback:  []model.IssueOpsFeedbackItem{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return store.Write(stateRoot, record)
}

// resumeOrReset returns the existing cycle as-is, unless it is a stale
// reset-eligible cycle whose isolated worktree directory was deleted without
// releasing the cycle. Resuming such a record would resurrect old phase/plan/
// intent state for what is effectively new work and would immediately re-trigger
// the source-checkout worktree guard. In that case the cycle is reset to a fresh
// problem-phase record that keeps repo+branch identity, CreatedAt, the issue
// linkage anchors (IssueURL/IssueLinks), and the non-worktree analysis/audit
// metadata (Intent/DesignReview/DomainReview/Decisions/PlanPrep/
// CompatibilityReview/ExecutionDecision/RoutingTrace). That metadata lives in
// the state JSON, not the worktree, and describes the problem/plan that carries
// into re-work, so a transient worktree-miss must not destroy it. The
// design/compatibility approval flags inside that metadata are cleared, so the
// re-entered cycle must re-earn those gates while keeping the analysis content.
// The prior phase plus a timestamp are recorded for audit. Only the artifacts that
// genuinely describe the abandoned worktree (PlanPath, WorktreePath,
// BranchPrepare, WorktreeTools, ai-slop-clean evidence) and the transient
// feedback queue are cleared. The pr phase is never reset here — see
// model.IssueOpsPhaseResettableOnStaleWorktree — so remote PR linkage survives.
func resumeOrReset(store Store, stateRoot string, existing model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	if !staleResettableWorktreeCycle(store, existing) {
		return existing, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	createdAt := existing.CreatedAt
	if strings.TrimSpace(createdAt) == "" {
		createdAt = now
	}
	reset := model.IssueOpsRecord{
		OK:     true,
		ID:     existing.ID,
		Repo:   existing.Repo,
		Branch: existing.Branch,
		Phase:  model.IssueOpsPhaseProblem,
		// Non-worktree analysis/audit metadata: this state lives in the state
		// JSON (not the deleted worktree) and describes the problem/plan that
		// carries into re-work, so it survives the reset.
		Intent:              existing.Intent,
		DesignReview:        existing.DesignReview,
		DomainReview:        existing.DomainReview,
		IssueURL:            existing.IssueURL,
		IssueLinks:          existing.IssueLinks,
		Decisions:           existing.Decisions,
		PlanPrep:            existing.PlanPrep,
		ExecutionDecision:   existing.ExecutionDecision,
		CompatibilityReview: existing.CompatibilityReview,
		Feedback:            []model.IssueOpsFeedbackItem{},
		Delegation:          existing.Delegation,
		ChildCycles:         existing.ChildCycles,
		RoutingTrace:        existing.RoutingTrace,
		// Audit breadcrumbs for the abandoned worktree.
		StaleResetAt:         now,
		StaleResetPriorPhase: string(existing.Phase),
		OrphanWorktreePath:   existing.WorktreePath,
		CreatedAt:            createdAt,
		UpdatedAt:            now,
	}
	// A stale-reset re-enters at problem phase as effectively new work, so the
	// design/compatibility *approval gates* must be re-earned even though the
	// review content above is preserved for reference. This mirrors
	// RegressIssueOpsForReplan, which clears DesignReview.Approved when sending a
	// cycle back for re-plan. Copy the structs so clearing the gate flag does not
	// mutate the prior record's pointers.
	if existing.DesignReview != nil {
		dr := *existing.DesignReview
		dr.Approved = false
		reset.DesignReview = &dr
	}
	if existing.CompatibilityReview != nil {
		cr := *existing.CompatibilityReview
		cr.Approved = false
		reset.CompatibilityReview = &cr
	}
	return store.Write(stateRoot, reset)
}

// staleResettableWorktreeCycle reports whether a cycle is reset-eligible (a
// non-pr worktree phase) and points at a worktree directory that no longer
// exists. A nil WorktreeValid validator disables the check (fail-open): such a
// Store is only built by tests that do not exercise worktree-phase resumption;
// production wiring always injects the validator (see package.go).
func staleResettableWorktreeCycle(store Store, record model.IssueOpsRecord) bool {
	if record.ExecutionHandoff != nil {
		return false
	}
	if store.WorktreeValid == nil {
		return false
	}
	if !model.IssueOpsPhaseResettableOnStaleWorktree(record.Phase) {
		return false
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		return false
	}
	return !store.WorktreeValid(worktree)
}
