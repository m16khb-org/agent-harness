package issueops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"agent-harness/internal/adapter/outbound/sqlstore"
)

type cleanupAbandonLockKey struct{}

// withIssueOpsLock serializes the full read-modify-write span for a cycle
// against every other span on the same state root, in-process and
// cross-process, via the sqlstore span lock (a held BEGIN IMMEDIATE
// transaction on the state root's lock database that dies with the process).
// The callback receives the active-root context, so same-root re-entry and
// cycles are rejected before waiting while documented distinct-root ordering
// remains possible.
func withIssueOpsLock(ctx context.Context, stateRoot, id string, fn func(context.Context) error) error {
	if _, err := normalizeIssueOpsID(id); err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, func(spanCtx context.Context) error {
		if bypass, _ := ctx.Value(cleanupAbandonLockKey{}).(bool); !bypass {
			record, readErr := ReadIssueOps(stateRoot, id)
			switch {
			case readErr == nil && record.CleanupAbandonFailure != nil && record.CleanupAbandonFailure.Step == "applying":
				return fmt.Errorf("cleanup abandon apply is in progress")
			case readErr != nil && !errors.Is(readErr, fs.ErrNotExist):
				return readErr
			}
		}
		return fn(spanCtx)
	})
}

// withCleanupAbandonLock만 applying fence를 갱신하거나 최종 삭제할 수 있다.
// 다른 lifecycle writer는 같은 span에서 fence를 보고 Git mutation 동안 거부된다.
func withCleanupAbandonLock(ctx context.Context, stateRoot, id string, fn func(context.Context) error) error {
	return withIssueOpsLock(context.WithValue(ctx, cleanupAbandonLockKey{}, true), stateRoot, id, fn)
}
