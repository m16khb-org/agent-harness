package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/commandparse"
	issueopscore "agent-harness/internal/core/issueops"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/lifecycle/worktreeguard"
	"agent-harness/internal/core/searchrouting"
)

// 관찰 권한은 동시 execution 중 owner를 먼저 고르는 절차에 의존하지 않는다.
func executionObservation(req HookToolUseLifecycleRequest) bool {
	if !searchrouting.IsShellTool(req.Tool) {
		return explicitIssueOpsReadOnlyTool(req.Tool)
	}
	if commandparse.ExactReadOnlyShellCommand(req.Command) {
		return true
	}
	command, ok := commandparse.ParseExactIssueOpsCommand(req.Command)
	if !ok {
		return false
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return false
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return false
	}
	switch command.Path {
	case "status", "execution status":
		id, ok := oneFlag(flags, "--id")
		return ok && strings.TrimSpace(id) != ""
	case "remote score":
		return exactRemoteScoreObservation(flags)
	case "execution replace":
		_, preview := flags["--preview"]
		_, finalizePreview := flags["--finalize-preview"]
		_, confirm := flags["--confirm"]
		_, revoke := flags["--revoke"]
		_, finalize := flags["--finalize"]
		_, reseed := flags["--reseed"]
		id, idOK := oneFlag(flags, "--id")
		generation, generationOK := oneFlag(flags, "--expected-generation")
		return idOK && strings.TrimSpace(id) != "" && generationOK && strings.TrimSpace(generation) != "" &&
			preview != finalizePreview && !confirm && !revoke && !finalize && !reseed
	case "execution reconcile":
		_, preview := flags["--preview"]
		_, confirm := flags["--confirm"]
		id, idOK := oneFlag(flags, "--id")
		return idOK && strings.TrimSpace(id) != "" && preview && !confirm
	default:
		return false
	}
}

func executionTypedControlPlane(req HookToolUseLifecycleRequest) bool {
	if !searchrouting.IsShellTool(req.Tool) {
		tool := strings.TrimSpace(req.Tool)
		return (tool == "issueops_execution" || tool == "mcp__agent_harness__issueops_execution") && req.ToolInput != nil
	}
	command, ok := commandparse.ParseExactIssueOpsCommand(req.Command)
	if !ok {
		return false
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return false
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return false
	}
	switch command.Path {
	case "execution prepare", "execution claim", "execution release", "execution replace", "execution reconcile", "execution complete":
		id, ok := oneFlag(flags, "--id")
		return ok && strings.TrimSpace(id) != ""
	default:
		return false
	}
}

func executionMutationDecision(req HookToolUseLifecycleRequest) (bool, string, *IssueOpsDenyReason) {
	if !req.EnforceWorktree {
		return false, "", nil
	}
	unsafeReason := executionUnsafeMutationReason(req)
	mayMutate := toolUseMayMutateLifecycleFiles(req.Tool, req.Command)
	if searchrouting.IsShellTool(req.Tool) && !mayMutate {
		mayMutate = true
		if unsafeReason == "" {
			unsafeReason = "unclassified shell command is blocked while IssueOps mutation authority is active; use an exact listed reader or a statically classified foreground mutation command"
		}
	}
	if !mayMutate {
		return false, "", nil
	}
	targets := worktreeGuardEditTargets(req)
	records, err := executionGuardRecords(req, targets)
	if err != nil {
		return true, "IssueOps authority state could not be read (often transient state-store contention); retry once, and if it persists run `agent-harness doctor --repo " + cleanAbsPath(req.Repo) + " --json`", nil
	}
	if len(records) == 0 {
		return false, "", nil
	}
	for _, record := range records {
		if record.Execution == nil {
			if requestTouchesUnpreparedExecution(req, targets, record) {
				if unsafeReason != "" {
					return true, unsafeReason, nil
				}
				return true, fmt.Sprintf("IssueOps execution %s requires `agent-harness issueops execution prepare --id %s ...` before implementation mutation", record.ID, record.ID), nil
			}
			continue
		}
		if !requestTouchesExecution(req, targets, *record.Execution) {
			continue
		}
		if unsafeReason != "" {
			return true, unsafeReason, executionDeny(record, "unsafe_mutation", executionStatusCommand(record.ID))
		}
		if err := issueopsmodel.ValidateExecution(*record.Execution); err != nil {
			return true, "invalid IssueOps execution v1 record: " + err.Error(), nil
		}
		lease := record.Execution.Lease
		root := record.Execution.Workspace.Root
		if lease.Status == issueopsmodel.LeaseStatusActive && executionActorMatches(req, lease.Holder) &&
			sameExecutionPath(req.CWD, root) && allExecutionTargetsInside(targets, root) {
			return true, "", nil
		}
		reason, deny := executionMutationDenyReason(record)
		return true, reason, deny
	}
	if executionSharesSourceCheckout(req, records) {
		return true, "mutation is outside every canonical IssueOps worktree for this Git source checkout; use the assigned execution workspace", nil
	}
	return false, "", nil
}

func executionUnsafeMutationReason(req HookToolUseLifecycleRequest) string {
	if !searchrouting.IsShellTool(req.Tool) {
		if toolUseMayMutateLifecycleFiles(req.Tool, req.Command) && len(req.Paths) == 0 {
			return "filesystem mutation target is unresolved; provide one exact path inside the canonical IssueOps worktree"
		}
		return ""
	}
	command := strings.TrimSpace(req.Command)
	if commandparse.HasUnquotedBackgroundOperator(command) || executionDetachedShellCommand(command) {
		return "background or detached mutation is blocked; run the command in the foreground and observe it to completion in the holder session"
	}
	if worktreeguard.SealedGitTopologyMutation(command) {
		return "the IssueOps branch and worktree identity are sealed; direct switch/reset/rebase/merge/force-push/worktree mutation is blocked"
	}
	if commandparse.HasUnquotedControlOperator(command) || commandparse.HasActiveCommandSubstitution(command) ||
		commandparse.HasActiveInputRedirect(command) || commandparse.HasActiveParameterOrTildeExpansion(command) ||
		commandparse.HasActivePathnameExpansion(command) || commandparse.HasActiveShellSpecialQuoting(command) ||
		commandparse.HasActiveZshEqualsExpansion(command) || executionEvalWrapper(command) {
		return "shell substitution or wrapper target is not statically resolvable; use one exact foreground command with literal paths"
	}
	return ""
}

func executionDetachedShellCommand(command string) bool {
	for _, token := range commandparse.SplitCommandTokens(command) {
		switch searchrouting.SearchTokenName(token) {
		case "nohup", "daemonize", "setsid", "disown":
			return true
		}
		value := strings.ToLower(strings.TrimSpace(token))
		for _, flag := range []string{"--detach", "--detached", "--daemon", "--daemonize", "--background"} {
			if value == flag || strings.HasPrefix(value, flag+"=") {
				return true
			}
		}
	}
	return false
}

func executionEvalWrapper(command string) bool {
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		switch searchrouting.SearchTokenName(token) {
		case "bash", "sh", "zsh":
			for _, arg := range tokens[i+1:] {
				if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(strings.TrimPrefix(arg, "-"), "c") {
					return true
				}
			}
		case "python", "python3":
			if containsExecutionToken(tokens[i+1:], "-c") {
				return true
			}
		case "node":
			if containsExecutionToken(tokens[i+1:], "-e") || containsExecutionToken(tokens[i+1:], "--eval") {
				return true
			}
		}
	}
	return false
}

func containsExecutionToken(tokens []string, want string) bool {
	for _, token := range tokens {
		if token == want {
			return true
		}
	}
	return false
}

func executionGuardRecords(req HookToolUseLifecycleRequest, targets []string) ([]IssueOpsRecord, error) {
	records := []IssueOpsRecord{}
	if strings.TrimSpace(req.SourceCheckout) == "" {
		req.SourceCheckout = executionRequestSourceCheckout(req)
	}
	ids, err := issueopscore.ListIssueOpsIDs(IssueOpsStateRoot())
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		record, readErr := ReadIssueOps(IssueOpsStateRoot(), id)
		if readErr != nil {
			return nil, readErr
		}
		if executionRecordTouchesRequest(record, req, targets) {
			records = append(records, record)
		}
	}
	return records, nil
}

func executionRequestSourceCheckout(req HookToolUseLifecycleRequest) string {
	if source := cleanAbsPath(req.SourceCheckout); source != "" {
		return source
	}
	for _, root := range []string{req.CWD, req.Repo} {
		if source := sourceCheckoutFromWorktree(root); source != "" {
			return cleanAbsPath(source)
		}
	}
	return ""
}

func executionSharesSourceCheckout(req HookToolUseLifecycleRequest, records []IssueOpsRecord) bool {
	source := executionRequestSourceCheckout(req)
	if source == "" {
		return false
	}
	for _, record := range records {
		if record.Execution != nil && sameExecutionPath(source, record.Execution.Workspace.SourceRoot) {
			return true
		}
	}
	return false
}

func executionRecordTouchesRequest(record IssueOpsRecord, req HookToolUseLifecycleRequest, targets []string) bool {
	paths := append(append([]string{}, targets...), req.SourceCheckout, req.Repo, req.CWD)
	if record.Execution == nil {
		root := cleanAbsPath(record.Repo)
		for _, path := range paths {
			if pathWithin(cleanAbsPath(path), root) || sameExecutionPath(executionContainingSourceCheckout(path), root) {
				return true
			}
		}
		return false
	}
	workspace := record.Execution.Workspace
	for _, path := range paths {
		if pathWithin(cleanAbsPath(path), cleanAbsPath(workspace.SourceRoot)) || pathWithin(cleanAbsPath(path), cleanAbsPath(workspace.Root)) ||
			sameExecutionPath(executionContainingSourceCheckout(path), workspace.SourceRoot) {
			return true
		}
	}
	return false
}

func requestTouchesUnpreparedExecution(req HookToolUseLifecycleRequest, targets []string, record IssueOpsRecord) bool {
	if !issueopscore.IssueOpsPhaseExpectsWorktree(record.Phase) {
		return false
	}
	root := cleanAbsPath(record.Repo)
	for _, path := range append(append([]string{}, targets...), req.CWD, req.Repo) {
		if pathWithin(cleanAbsPath(path), root) {
			return true
		}
	}
	return false
}

func requestTouchesExecution(req HookToolUseLifecycleRequest, targets []string, execution issueopsmodel.Execution) bool {
	for _, path := range append(append([]string{}, targets...), req.CWD, req.Repo) {
		if pathWithin(cleanAbsPath(path), cleanAbsPath(execution.Workspace.Root)) ||
			pathWithin(cleanAbsPath(path), cleanAbsPath(execution.Workspace.SourceRoot)) ||
			sameExecutionPath(executionContainingSourceCheckout(path), execution.Workspace.SourceRoot) {
			return true
		}
	}
	return false
}

func executionContainingSourceCheckout(path string) string {
	current := cleanAbsPath(path)
	if current == "" {
		return ""
	}
	if info, err := os.Lstat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		if source := sourceCheckoutFromWorktree(current); source != "" {
			return source
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func executionActorMatches(req HookToolUseLifecycleRequest, holder *issueopsmodel.NativeActor) bool {
	if holder == nil || holder.SessionProcess == nil || !strings.EqualFold(strings.TrimSpace(req.Host), holder.Host) ||
		strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.SessionID) != holder.SessionID ||
		strings.TrimSpace(req.AgentID) != holder.AgentID {
		return false
	}
	for _, observed := range req.NativeProcessAncestry {
		if observed == *holder.SessionProcess {
			return true
		}
	}
	return false
}

func allExecutionTargetsInside(targets []string, root string) bool {
	if len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		if !executionResolvedTargetInside(target, root) {
			return false
		}
	}
	return true
}

func executionResolvedTargetInside(target, root string) bool {
	resolvedRoot, err := filepath.EvalSymlinks(cleanAbsPath(root))
	if err != nil {
		return false
	}
	resolvedTarget, ok := executionResolveExistingAncestor(target)
	return ok && pathWithin(resolvedTarget, resolvedRoot)
}

func executionResolveExistingAncestor(path string) (string, bool) {
	current := cleanAbsPath(path)
	if current == "" {
		return "", false
	}
	suffix := []string{}
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", false
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return cleanAbsPath(resolved), true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func sameExecutionPath(left, right string) bool {
	left, right = cleanAbsPath(left), cleanAbsPath(right)
	if resolved, err := filepath.EvalSymlinks(left); err == nil {
		left = resolved
	}
	if resolved, err := filepath.EvalSymlinks(right); err == nil {
		right = resolved
	}
	return left != "" && left == right
}

func executionMutationDenyReason(record IssueOpsRecord) (string, *IssueOpsDenyReason) {
	execution := record.Execution
	root := execution.Workspace.Root
	generation := execution.Lease.Generation
	switch execution.Lease.Status {
	case issueopsmodel.LeaseStatusRevoking:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("IssueOps execution %s generation %d is revoking and has no writer; inspect with `%s`", record.ID, generation, next), executionDeny(record, "lease_revoking", next)
	case issueopsmodel.LeaseStatusClaimable:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("IssueOps execution %s generation %d is claimable and has no writer; inspect with `%s`", record.ID, generation, next), executionDeny(record, "lease_claimable", next)
	case issueopsmodel.LeaseStatusReleased:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("IssueOps execution %s generation %d is released and has no writer; inspect with `%s`", record.ID, generation, next), executionDeny(record, "lease_released", next)
	default:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("mutation requires the current write lease for IssueOps execution %s generation %d and canonical root %s; inspect with `%s`", record.ID, generation, root, next), executionDeny(record, "write_lease_required", next)
	}
}

func executionDeny(record IssueOpsRecord, code, nextCommand string) *IssueOpsDenyReason {
	return &IssueOpsDenyReason{
		Code: code, LifecycleID: record.ID, ExpectedRoot: record.Execution.Workspace.Root,
		CurrentGeneration: record.Execution.Lease.Generation, NextCommand: nextCommand,
	}
}

func executionStatusCommand(id string) string {
	return fmt.Sprintf("agent-harness issueops execution status --id %s --json", id)
}

func exactRemoteScoreObservation(flags map[string][]string) bool {
	input, ok := oneFlag(flags, "--input")
	if !ok || strings.TrimSpace(input) == "" {
		return false
	}
	judge, hasJudge := oneFlag(flags, "--judge")
	judgeFile, hasJudgeFile := oneFlag(flags, "--judge-file")
	if !hasJudge {
		return !hasJudgeFile
	}
	switch judge {
	case "none":
		return !hasJudgeFile
	case "file":
		return hasJudgeFile && strings.TrimSpace(judgeFile) != ""
	default:
		return false
	}
}
