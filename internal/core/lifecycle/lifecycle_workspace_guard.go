package lifecycle

import (
	"path/filepath"
	"strings"

	"agent-harness/internal/core/commandparse"
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

// allowedSourceWorkspacePlanMutation keeps planning ownership in the sealed
// source session without granting source-side implementation authority. The
// only writable worker artifact is the already-linked plan, and the only Git
// mutation is an argv-shaped commit restricted by git commit --only to that
// same file.
func allowedSourceWorkspacePlanMutation(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	workspace := record.ExecutionWorkspace
	if workspace == nil || workspace.State != "ready" || workspace.PreparationSession == nil || !nativeSessionMatches(req, workspace.PreparationSession) {
		return false
	}
	sourceRoot, workerRoot, planPath := cleanAbsPath(record.Repo), cleanAbsPath(workspace.WorkerRoot), cleanAbsPath(record.PlanPath)
	requestSource := cleanAbsPath(req.SourceCheckout)
	if cleanAbsPath(req.CWD) != sourceRoot || requestSource != "" && requestSource != sourceRoot || planPath == "" || !pathWithin(planPath, workerRoot) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(req.Tool)) {
	case "apply_patch", "edit", "write", "multiedit":
		targets := worktreeGuardEditTargets(req)
		if len(targets) == 0 {
			return false
		}
		for _, target := range targets {
			if cleanAbsPath(target) != planPath {
				return false
			}
		}
		return true
	case "bash", "shell", "exec_command":
		return exactLinkedPlanOnlyCommit(req.Command, workerRoot, planPath)
	default:
		return false
	}
}

func exactLinkedPlanOnlyCommit(command, workerRoot, planPath string) bool {
	if commandparse.HasUnquotedControlOperator(command) || commandparse.HasActiveCommandSubstitution(command) || commandparse.HasActiveOutputRedirect(command) || commandparse.HasActiveParameterOrTildeExpansion(command) || commandparse.HasActivePathnameExpansion(command) || commandparse.HasActiveShellSpecialQuoting(command) || commandparse.HasActiveZshEqualsExpansion(command) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(command))
	if len(tokens) != 8 || tokens[0] != "git" || tokens[1] != "-C" || cleanAbsPath(tokens[2]) != workerRoot || tokens[3] != "commit" || tokens[4] != "--only" || tokens[6] != "-m" || strings.TrimSpace(tokens[7]) == "" {
		return false
	}
	target := tokens[5]
	if !filepath.IsAbs(target) {
		target = filepath.Join(workerRoot, target)
	}
	return cleanAbsPath(target) == planPath
}
