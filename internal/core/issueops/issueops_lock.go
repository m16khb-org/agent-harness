package issueops

import "agent-harness/internal/core/sqlstore"

// withIssueOpsLock serializes the full read-modify-write span for a cycle
// against every other span on the same state root, in-process and
// cross-process, via the sqlstore span lock (a held BEGIN IMMEDIATE
// transaction on the state root's lock database that dies with the process).
// Spans must not nest: calling a with*Lock-wrapped function from inside
// another span callback on the same state root self-deadlocks, exactly like
// the flock re-entry hazard this replaced. Multi-entity operations stay
// sequential single-span steps with read-repair.
func withIssueOpsLock(stateRoot, id string, fn func() error) error {
	if _, err := normalizeIssueOpsID(id); err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.WithSpan(fn)
}
