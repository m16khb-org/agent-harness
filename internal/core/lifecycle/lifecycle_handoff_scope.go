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
	if looksLikeIssueOpsControl(req) {
		return handoffFenceScopeAmbiguousCrossRoot
	}
	if _, control, literal := protectedOrcaResourceTargets(req); control {
		if !literal {
			return handoffFenceScopeAmbiguousCrossRoot
		}
		matches, _ := recordsMatchingProtectedOrcaResource(req, records)
		if len(matches) == 1 {
			return handoffFenceScopeWorkerOrCycleTargeted
		}
		if len(matches) > 1 {
			return handoffFenceScopeAmbiguousCrossRoot
		}
	}
	for _, record := range records {
		if requestRunsFromOrTargetsWorkerRoot(req, record.ExecutionHandoff.WorkerRoot) {
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

func looksLikeIssueOpsControl(req HookToolUseLifecycleRequest) bool {
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	if strings.Contains(tool, "issueops_") {
		return true
	}
	if !searchrouting.IsShellTool(req.Tool) {
		return false
	}
	for i, token := range commandparse.SplitCommandTokens(strings.TrimSpace(req.Command)) {
		if searchrouting.SearchTokenName(token) == "issueops" && i > 0 {
			return true
		}
	}
	return false
}

func requestRunsFromOrTargetsWorkerRoot(req HookToolUseLifecycleRequest, workerRoot string) bool {
	workerRoot = cleanAbsPath(workerRoot)
	if workerRoot == "" {
		return false
	}
	if cleanAbsPath(req.CWD) == workerRoot || cleanAbsPath(req.Repo) == workerRoot {
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
