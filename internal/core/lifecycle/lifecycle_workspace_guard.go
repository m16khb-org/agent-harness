package lifecycle

import (
	"strings"
)

// workspacePreparationBlockReason controls only a request already resolved to
// an exact IssueOps workspace. Ordinary source-root work never reaches here.
// Before ownership dispatch, the sealed preparation session may set up the
// isolated root once it is ready; all other actor/root/state combinations fail
// closed without converting the workspace into a handoff fence.
func workspacePreparationBlockReason(req HookToolUseLifecycleRequest, record IssueOpsRecord) string {
	workspace := record.ExecutionWorkspace
	if workspace == nil {
		return ""
	}
	if workspace.State != "ready" {
		return "execution workspace is " + workspace.State + "; only exact workspace reconciliation is allowed before further preparation"
	}
	if workspace.PreparationSession == nil || strings.TrimSpace(req.Host) != workspace.PreparationSession.Host || strings.TrimSpace(req.SessionID) != workspace.PreparationSession.SessionID || strings.TrimSpace(req.AgentID) != workspace.PreparationSession.AgentID {
		return "isolated workspace preparation requires the exact sealed native preparation session"
	}
	if cleanAbsPath(req.CWD) != cleanAbsPath(workspace.WorkerRoot) {
		return "isolated workspace preparation requires the canonical ready workspace cwd"
	}
	if source := cleanAbsPath(req.SourceCheckout); source != "" && source != cleanAbsPath(record.Repo) {
		return "isolated workspace preparation source checkout does not match the sealed cycle"
	}
	return ""
}

func workspacePreparationStateKnown(record IssueOpsRecord) bool {
	return record.ExecutionWorkspace != nil && record.ExecutionHandoff == nil
}
