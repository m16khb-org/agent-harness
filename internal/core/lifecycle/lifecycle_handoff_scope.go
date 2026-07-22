package lifecycle

import (
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/searchrouting"
)

type handoffFenceScope string

const (
	handoffFenceScopeSourceOnly            handoffFenceScope = "source_only"
	handoffFenceScopeWorkerOrCycleTargeted handoffFenceScope = "worker_or_cycle_targeted"
	handoffFenceScopeAmbiguousCrossRoot    handoffFenceScope = "ambiguous_cross_root"
)

func classifyHandoffFenceScope(req HookToolUseLifecycleRequest, records []IssueOpsRecord) handoffFenceScope {
	if _, ok := lifecycleRecordID(req); ok {
		return handoffFenceScopeWorkerOrCycleTargeted
	}
	if isLiteralNewCycleStartFromSource(req, records) {
		return handoffFenceScopeSourceOnly
	}
	if looksLikeIssueOpsControl(req) {
		return handoffFenceScopeAmbiguousCrossRoot
	}
	if _, control, literal := protectedOrcaResourceTargets(req); control {
		if !literal {
			return handoffFenceScopeAmbiguousCrossRoot
		}
		matches, reason := recordsMatchingProtectedOrcaResource(req, records)
		if reason != "" {
			return handoffFenceScopeAmbiguousCrossRoot
		}
		if len(matches) == 1 {
			return handoffFenceScopeWorkerOrCycleTargeted
		}
		if len(matches) > 1 {
			return handoffFenceScopeAmbiguousCrossRoot
		}
	}
	for _, record := range records {
		if requestRunsFromOrTargetsWorkerRoot(req, executionWorkerRoot(record)) {
			return handoffFenceScopeWorkerOrCycleTargeted
		}
	}
	if shellRequestHasUnresolvedCrossRootSurface(req) {
		return handoffFenceScopeAmbiguousCrossRoot
	}
	for _, record := range records {
		if requestIsProvenSourceOnly(req, record.Repo) {
			return handoffFenceScopeSourceOnly
		}
	}
	return handoffFenceScopeSourceOnly
}

func isLiteralNewCycleStartFromSource(req HookToolUseLifecycleRequest, records []IssueOpsRecord) bool {
	sourceRoot := ""
	for _, record := range records {
		candidate := cleanAbsPath(record.Repo)
		if candidate == "" || cleanAbsPath(req.CWD) != candidate {
			continue
		}
		if sourceRoot != "" && sourceRoot != candidate {
			return false
		}
		sourceRoot = candidate
	}
	if sourceRoot == "" {
		return false
	}
	tool := strings.TrimSpace(req.Tool)
	if tool == "issueops_start" || tool == "mcp__agent_harness__issueops_start" {
		if req.ToolInput == nil {
			return false
		}
		repo, ok := req.ToolInput["repo"].(string)
		return ok && resolveHookTargetPath(req.CWD, repo) == sourceRoot
	}
	if !searchrouting.IsShellTool(req.Tool) {
		return false
	}
	command, ok := commandparse.ParseExactIssueOpsCommand(req.Command)
	if !ok || command.Path != "start" {
		return false
	}
	flags, ok := commandparse.ExactFlags(command,
		map[string]bool{"--repo": true, "--branch": true},
		map[string]bool{"--json": true},
		map[string]bool{},
	)
	if !ok {
		return false
	}
	repo, ok := flags["--repo"]
	return ok && len(repo) == 1 && resolveHookTargetPath(req.CWD, repo[0]) == sourceRoot
}

func executionWorkerRoot(record IssueOpsRecord) string {
	if retainedOwnershipHandoff(record) != nil {
		return retainedOwnershipHandoff(record).WorkerRoot
	}
	if retainedOwnershipWorkspace(record) != nil {
		return retainedOwnershipWorkspace(record).WorkerRoot
	}
	return ""
}

func looksLikeIssueOpsControl(req HookToolUseLifecycleRequest) bool {
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	if strings.Contains(tool, "issueops_") {
		return true
	}
	if !searchrouting.IsShellTool(req.Tool) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	return len(tokens) >= 2 &&
		(tokens[0] == "agent-harness" || tokens[0] == "bin/agent-harness" || tokens[0] == "./bin/agent-harness") &&
		tokens[1] == "issueops"
}

func requestRunsFromOrTargetsWorkerRoot(req HookToolUseLifecycleRequest, workerRoot string) bool {
	workerRoot = cleanAbsPath(workerRoot)
	if workerRoot == "" {
		return false
	}
	requestCWD := cleanAbsPath(req.CWD)
	if requestCWD == workerRoot || requestCWD == "" && cleanAbsPath(req.Repo) == workerRoot {
		return true
	}
	for _, target := range worktreeGuardEditTargets(req) {
		if pathWithin(target, workerRoot) {
			return true
		}
	}
	return false
}

func requestIsProvenSourceOnly(req HookToolUseLifecycleRequest, sourceRoot string) bool {
	sourceRoot = cleanAbsPath(sourceRoot)
	if sourceRoot == "" {
		return false
	}
	if cleanAbsPath(req.CWD) != sourceRoot && cleanAbsPath(req.Repo) != sourceRoot {
		return false
	}
	targets := worktreeGuardEditTargets(req)
	if len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		if !pathWithin(target, sourceRoot) || !resolvedPathWithin(target, sourceRoot) {
			return false
		}
	}
	return true
}

func shellRequestHasUnresolvedCrossRootSurface(req HookToolUseLifecycleRequest) bool {
	if !searchrouting.IsShellTool(req.Tool) {
		return false
	}
	return shellRequestHasNestedEvaluation(req.Command) ||
		commandparse.HasUnquotedControlOperator(req.Command) ||
		commandparse.HasActiveCommandSubstitution(req.Command) ||
		commandparse.HasActiveInputRedirect(req.Command) ||
		commandparse.HasActiveParameterOrTildeExpansion(req.Command) ||
		commandparse.HasActivePathnameExpansion(req.Command) ||
		commandparse.HasActiveShellSpecialQuoting(req.Command) ||
		commandparse.HasActiveZshEqualsExpansion(req.Command) ||
		strings.TrimSpace(req.Command) == ""
}

func shellRequestHasNestedEvaluation(command string) bool {
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(command))
	for i := 0; i+1 < len(tokens); i++ {
		name := searchrouting.SearchTokenName(tokens[i])
		if name != "bash" && name != "sh" && name != "zsh" {
			continue
		}
		for _, flag := range tokens[i+1:] {
			if strings.HasPrefix(flag, "-") && !strings.HasPrefix(flag, "--") && strings.Contains(strings.TrimPrefix(flag, "-"), "c") {
				return true
			}
		}
	}
	return false
}
