// Package stalescan classifies abandoned IssueOps cycles using multiple
// liveness signals rather than a single time threshold. It runs off the
// hot path (the cleanup command), so it may consult git/remote probes for
// higher-accuracy, lower-false-positive classification.
package stalescan

import (
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
)

type Category string

const (
	// CategoryConfirmedStale: the worktree (the work product) is provably gone or
	// repurposed — safe to release.
	CategoryConfirmedStale Category = "confirmed-stale"
	// CategoryLikelyDone: the remote branch backing a pr-stage / artifact-bearing
	// cycle no longer exists (merged or deleted) — safe to release.
	CategoryLikelyDone Category = "likely-done"
	// CategoryNeedsReview: only the age threshold matched; report only, never
	// auto-released, because a paused cycle can be legitimately idle.
	CategoryNeedsReview Category = "needs-review"
)

// Probe supplies the external signals used for classification. Any nil probe is
// skipped, so callers can opt out of git/remote checks (e.g. offline runs).
type Probe struct {
	// WorktreeDirExists reports whether the worktree directory still exists.
	WorktreeDirExists func(path string) bool
	// WorktreeHeadBranch returns the git branch currently checked out in the
	// worktree, or "" if the path is not a readable git worktree.
	WorktreeHeadBranch func(path string) string
	// RemoteBranchExists reports whether the cycle's remote branch still exists;
	// determined is false when the answer could not be established (offline, no
	// remote) so the caller must not infer absence.
	RemoteBranchExists func(record model.IssueOpsRecord) (exists bool, determined bool)
	// Now returns the current time for age comparison.
	Now func() time.Time
}

type Finding struct {
	ID           string   `json:"id"`
	Branch       string   `json:"branch,omitempty"`
	Phase        string   `json:"phase"`
	Category     Category `json:"category"`
	Reasons      []string `json:"reasons"`
	WorktreePath string   `json:"worktree_path,omitempty"`
	// Releasable marks findings that --apply may force-release (confirmed-stale
	// and likely-done). needs-review is never releasable.
	Releasable bool `json:"releasable"`
}

// Classify inspects a single record and returns a Finding plus whether the
// record is stale enough to surface. done cycles are never flagged. Signals are
// evaluated most-confident first: confirmed-stale, then likely-done, then the
// age-only needs-review fallback.
func Classify(record model.IssueOpsRecord, probe Probe, maxAge time.Duration) (Finding, bool) {
	if record.Phase == model.IssueOpsPhaseDone {
		return Finding{}, false
	}
	f := Finding{
		ID:           record.ID,
		Branch:       strings.TrimSpace(record.Branch),
		Phase:        string(record.Phase),
		WorktreePath: strings.TrimSpace(record.WorktreePath),
	}

	if reason := confirmedStaleReason(record, probe); reason != "" {
		f.Category = CategoryConfirmedStale
		f.Reasons = []string{reason}
		f.Releasable = true
		return f, true
	}

	if branchMismatch(record, probe) {
		f.Category = CategoryNeedsReview
		f.Reasons = []string{"worktree_branch_mismatch"}
		f.Releasable = false
		return f, true
	}

	if likelyDone(record, probe) {
		f.Category = CategoryLikelyDone
		f.Reasons = []string{"remote_branch_absent"}
		f.Releasable = true
		return f, true
	}

	if staleByAge(record, probe, maxAge) {
		f.Category = CategoryNeedsReview
		f.Reasons = []string{"stale_age"}
		f.Releasable = false
		return f, true
	}

	return Finding{}, false
}

func confirmedStaleReason(record model.IssueOpsRecord, probe Probe) string {
	if !model.IssueOpsPhaseExpectsWorktree(record.Phase) {
		return ""
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		return ""
	}
	if probe.WorktreeDirExists != nil && !probe.WorktreeDirExists(worktree) {
		return "worktree_deleted"
	}
	if probe.WorktreeHeadBranch != nil {
		head := strings.TrimSpace(probe.WorktreeHeadBranch(worktree))
		if head == "" {
			return "worktree_not_git"
		}
		if record.Branch != "" && head != strings.TrimSpace(record.Branch) {
			return "" // branch mismatch is not confirmed-stale; see Classify
		}
	}
	return ""
}

func branchMismatch(record model.IssueOpsRecord, probe Probe) bool {
	if !model.IssueOpsPhaseExpectsWorktree(record.Phase) {
		return false
	}
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" || probe.WorktreeHeadBranch == nil || probe.WorktreeDirExists == nil {
		return false
	}
	if !probe.WorktreeDirExists(worktree) {
		return false // covered by confirmedStaleReason
	}
	head := strings.TrimSpace(probe.WorktreeHeadBranch(worktree))
	if head == "" {
		return false // covered by confirmedStaleReason
	}
	return record.Branch != "" && head != strings.TrimSpace(record.Branch)
}

func likelyDone(record model.IssueOpsRecord, probe Probe) bool {
	if probe.RemoteBranchExists == nil {
		return false
	}
	reachedPR := model.IssueOpsPhaseRank(record.Phase) >= model.IssueOpsPhaseRank(model.IssueOpsPhasePR)
	if record.RemoteArtifact == nil && !reachedPR {
		return false
	}
	exists, determined := probe.RemoteBranchExists(record)
	return determined && !exists
}

func staleByAge(record model.IssueOpsRecord, probe Probe, maxAge time.Duration) bool {
	if maxAge <= 0 || probe.Now == nil {
		return false
	}
	// Prefer LastHeartbeatAt (explicit liveness signal) over UpdatedAt
	// (which only updates on state mutations). A cycle may be actively
	// worked on without state changes for long periods.
	ts := parseTime(lastActiveAt(record))
	if ts.IsZero() {
		return false
	}
	return probe.Now().Sub(ts) >= maxAge
}

// lastActiveAt returns the best liveness timestamp: LastHeartbeatAt if set,
// otherwise UpdatedAt.
func lastActiveAt(record model.IssueOpsRecord) string {
	s := strings.TrimSpace(record.LastHeartbeatAt)
	if s != "" {
		return s
	}
	return strings.TrimSpace(record.UpdatedAt)
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}
