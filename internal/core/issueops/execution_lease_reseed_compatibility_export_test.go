package issueops

import "context"

func ReseedExecutionCompatibilityOracleForTest(ctx context.Context, stateRoot string, req ExecutionReplaceRequest, deps ExecutionReplaceDependencies) (ExecutionReplaceResult, error) {
	return reseedExecutionCompatibilityOracle(ctx, stateRoot, req, deps)
}
