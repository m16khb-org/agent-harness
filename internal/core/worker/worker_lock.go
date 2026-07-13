package worker

import (
	"context"

	"agent-harness/internal/core/sqlstore"
)

// withWorkerJobLock serializes the full read-modify-write span for a worker
// job against every other span on the same worker directory, in-process and
// cross-process, via the sqlstore span lock, while carrying cancellation and
// active-root metadata.
func withWorkerJobLock(ctx context.Context, dir, jobID string, fn func(context.Context) error) error {
	_ = jobID // spans are per-directory; the id names the span for callers
	db, err := sqlstore.Open(dir)
	if err != nil {
		return err
	}
	return db.WithSpanContext(ctx, fn)
}
