package issueopscli

import (
	"context"
	"encoding/json"
	"os"

	"agent-harness/internal/adapter/orca"
	"agent-harness/internal/core"
)

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func prepareIssueOpsHandoffWorktree(ctx context.Context, stateRoot string, req core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error) {
	return core.PrepareIssueOpsHandoffWorktree(ctx, stateRoot, req, orca.New(), core.IssueOpsHandoffPrepareClock{})
}

func migrateIssueOpsLegacyWorktree(ctx context.Context, stateRoot string, req core.IssueOpsLegacyWorktreeMigrationRequest) (core.IssueOpsLegacyWorktreeMigrationResult, error) {
	return core.MigrateIssueOpsLegacyWorktree(ctx, stateRoot, req, orca.New(), core.IssueOpsHandoffPrepareClock{})
}

var issueOpsWorkerDoneProjectionClient = func() core.IssueOpsWorkerDoneProjectionClient {
	return orca.New()
}
