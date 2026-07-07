package worker

import "agent-harness/internal/core/sqlstore"

// withWorkerJobLock serializes the full read-modify-write span for a worker
// job against every other span on the same worker directory, in-process and
// cross-process, via the sqlstore span lock. Spans must not nest.
func withWorkerJobLock(dir, jobID string, fn func() error) error {
	_ = jobID // spans are per-directory; the id names the span for callers
	db, err := sqlstore.Open(dir)
	if err != nil {
		return err
	}
	return db.WithSpan(fn)
}
