package lifecycle

import (
	"path/filepath"
	"strconv"
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/searchrouting"
)

// The exact-issueops command parser, flag spec, and read-only shell/ripgrep/orca
// security filters were extracted to internal/core/commandparse (Task C). This
// layer is a consumer; parsedExactIssueOps composes ParseExactIssueOpsCommand +
// IssueOpsCommandSpec + ExactFlags.
func parsedExactIssueOps(command string) (commandparse.ExactIssueOpsCommand, map[string][]string, bool) {
	parsed, ok := commandparse.ParseExactIssueOpsCommand(command)
	if !ok {
		return commandparse.ExactIssueOpsCommand{}, nil, false
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(parsed.Path)
	if !ok {
		return commandparse.ExactIssueOpsCommand{}, nil, false
	}
	flags, ok := commandparse.ExactFlags(parsed, values, booleans, repeatable)
	return parsed, flags, ok
}

func oneFlag(flags map[string][]string, name string) (string, bool) {
	values := flags[name]
	return func() (string, bool) {
		if len(values) != 1 {
			return "", false
		}
		return values[0], true
	}()
}

func exactLifecycleID(command string) (string, bool) {
	_, flags, ok := parsedExactIssueOps(command)
	if !ok {
		return "", false
	}
	return oneFlag(flags, "--id")
}

func nativeSessionMatches(req HookToolUseLifecycleRequest, session *issueopsmodel.IssueOpsHostSessionIdentity) bool {
	if session == nil || !strings.EqualFold(strings.TrimSpace(req.Host), strings.TrimSpace(session.Host)) || strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.SessionID) != strings.TrimSpace(session.SessionID) {
		return false
	}
	nativeAgent, persistedAgent := strings.TrimSpace(req.AgentID), strings.TrimSpace(session.AgentID)
	if persistedAgent == "" {
		return nativeAgent == ""
	}
	return nativeAgent == persistedAgent
}

func exactFenceFlags(flags map[string][]string, record IssueOpsRecord) bool {
	h := record.ExecutionHandoff
	attempt, aok := oneFlag(flags, "--attempt")
	epoch, eok := oneFlag(flags, "--ownership-epoch")
	contextSHA, cok := oneFlag(flags, "--context-sha256")
	return h != nil && aok && attempt == strconv.Itoa(h.Attempt) && eok && epoch == h.OwnershipEpoch && cok && contextSHA == h.ContextSHA256
}

func eventIdentityFlagsMatch(req HookToolUseLifecycleRequest, flags map[string][]string) bool {
	host, hok := oneFlag(flags, "--host")
	session, sok := oneFlag(flags, "--session-id")
	if !hok || !sok || !strings.EqualFold(host, strings.TrimSpace(req.Host)) || session != strings.TrimSpace(req.SessionID) {
		return false
	}
	agent, aok := oneFlag(flags, "--agent-id")
	if strings.TrimSpace(req.AgentID) == "" {
		return !aok
	}
	return aok && agent == strings.TrimSpace(req.AgentID)
}

func allowedExactHandoffLifecycleCommand(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	command, flags, ok := parsedExactIssueOps(req.Command)
	if !ok {
		return false
	}
	id, ok := oneFlag(flags, "--id")
	if !ok || id != record.ID || record.ExecutionHandoff == nil {
		return false
	}
	h := record.ExecutionHandoff
	source := cleanAbsPath(req.CWD) == cleanAbsPath(record.Repo)
	worker := cleanAbsPath(req.CWD) == cleanAbsPath(h.WorkerRoot)
	switch command.Path {
	case "status", "resume":
		return true
	case "handoff start":
		host, hok := oneFlag(flags, "--coordinator-host")
		sessionID, sok := oneFlag(flags, "--coordinator-session-id")
		agentID, aok := oneFlag(flags, "--coordinator-agent-id")
		cwd, cwdOK := oneFlag(flags, "--source-cwd")
		agentMatches := strings.TrimSpace(req.AgentID) == "" && !aok || aok && agentID == strings.TrimSpace(req.AgentID)
		return source && coordinatorLifecycleStateAllows(command.Path, record) && hok && sok && cwdOK && strings.EqualFold(host, strings.TrimSpace(req.Host)) && sessionID == strings.TrimSpace(req.SessionID) && agentMatches && cleanAbsPath(cwd) == cleanAbsPath(record.Repo)
	case "link-plan", "compatibility review", "execution decide", "devils-advocate review", "worktree prepare", "worktree prepare-tools", "handoff recover":
		return source && coordinatorLifecycleStateAllows(command.Path, record)
	case "handoff accept":
		cwd, cwdOK := oneFlag(flags, "--source-cwd")
		return source && coordinatorLifecycleStateAllows(command.Path, record) && eventIdentityFlagsMatch(req, flags) && cwdOK && cleanAbsPath(cwd) == cleanAbsPath(record.Repo) && handoff.CoordinatorIdentityMatches(record, issueopsmodel.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}, req.CWD)
	case "handoff publish":
		cwd, cwdOK := oneFlag(flags, "--source-cwd")
		_, confirmed := flags["--confirm"]
		_, approveLegacySeal := flags["--approve-legacy-coordinator-seal"]
		native := issueopsmodel.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}
		coordinator := handoff.CoordinatorIdentityMatches(record, native, req.CWD)
		legacySeal := approveLegacySeal && confirmed && handoff.LegacyCoordinatorIdentityCanBeSealed(record, native, req.CWD)
		return source && coordinatorLifecycleStateAllows(command.Path, record) && eventIdentityFlagsMatch(req, flags) && cwdOK && cleanAbsPath(cwd) == cleanAbsPath(record.Repo) && (coordinator || legacySeal)
	case "phase":
		return source && h.State == handoff.StateCoordinatorPreparing
	case "handoff claim":
		cwd, cwdOK := oneFlag(flags, "--cwd")
		worktreeID, wtOK := oneFlag(flags, "--orca-worktree-id")
		return worker && currentWorkerBranchMatches(record) && h.State == handoff.StateDispatched && exactFenceFlags(flags, record) && eventIdentityFlagsMatch(req, flags) && cwdOK && cleanAbsPath(cwd) == cleanAbsPath(h.WorkerRoot) && wtOK && h.Orca != nil && worktreeID == h.Orca.WorktreeID
	case "heartbeat", "handoff finish":
		return worker && currentWorkerBranchMatches(record) && exactFenceFlags(flags, record) && nativeSessionMatches(req, h.WorkerSession) && eventIdentityFlagsMatch(req, flags)
	default:
		return false
	}
}

func coordinatorLifecycleStateAllows(path string, record IssueOpsRecord) bool {
	// Default-deny wrapper over the shared declarative authority table (Task A):
	// the state dimension lives in handoff.CoordinatorCommandStateAllows; the
	// fence/identity/cwd predicates are applied by the caller.
	h := record.ExecutionHandoff
	if h == nil {
		return false
	}
	return handoff.CoordinatorCommandStateAllows(path, h.State, h.ClosedDisposition)
}

func exactReadOnlyShellCommand(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	command, flags, ok := parsedExactIssueOps(req.Command)
	if ok && (command.Path == "status" || command.Path == "resume") {
		id, idOK := oneFlag(flags, "--id")
		if idOK && id != record.ID {
			return false
		}
		return true
	}
	return commandparse.ExactReadOnlyShellCommand(req.Command)
}

func unresolvedNestedShellMutation(command string) bool {
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(command))
	if len(tokens) > 0 {
		switch tokens[0] {
		case "eval", "source", ".":
			return true
		case "builtin":
			if len(tokens) > 1 && tokens[1] == "eval" {
				return true
			}
		}
	}
	for i, token := range tokens {
		name := searchrouting.SearchTokenName(token)
		switch name {
		case "python", "python3":
			for _, arg := range tokens[i+1:] {
				if arg == "-c" {
					return true
				}
			}
		case "node":
			for _, arg := range tokens[i+1:] {
				if arg == "-e" || arg == "--eval" || strings.HasPrefix(arg, "--eval=") {
					return true
				}
			}
		case "perl":
			for j := i + 1; j < len(tokens); j++ {
				arg := tokens[j]
				if arg != "-e" && !strings.HasPrefix(arg, "-e=") {
					continue
				}
				scriptEnd := j
				if arg == "-e" {
					if j+1 >= len(tokens) {
						return true
					}
					scriptEnd = j + 1
				}
				for _, operand := range tokens[scriptEnd+1:] {
					if operand != "" && !strings.HasPrefix(operand, "-") {
						return false
					}
				}
				return true
			}
		case "awk", "gawk", "mawk":
			for _, arg := range tokens[i+1:] {
				if strings.Contains(strings.ToLower(arg), "system(") {
					return true
				}
			}
		case "bash", "sh", "zsh":
			for j := i + 1; j < len(tokens); j++ {
				arg := tokens[j]
				if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(strings.TrimPrefix(arg, "-"), "c") {
					return j+1 >= len(tokens) || len(worktreepathShellPaths(".", tokens[j+1])) == 0
				}
			}
		}
	}
	return false
}

func worktreepathShellPaths(repo, command string) []string {
	return shellCommandWorktreeGuardPaths(repo, command)
}

func allowedClosedOrcaCleanup(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	h := record.ExecutionHandoff
	if h == nil || h.State != handoff.StateClosed || cleanAbsPath(req.CWD) != cleanAbsPath(record.Repo) || cleanAbsPath(req.Repo) != cleanAbsPath(record.Repo) {
		return false
	}
	if commandparse.HasUnquotedControlOperator(req.Command) || commandparse.HasActiveCommandSubstitution(req.Command) || commandparse.HasActiveOutputRedirect(req.Command) || commandparse.HasActiveParameterOrTildeExpansion(req.Command) || commandparse.HasActivePathnameExpansion(req.Command) || commandparse.HasActiveShellSpecialQuoting(req.Command) || commandparse.HasActiveZshEqualsExpansion(req.Command) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	if len(tokens) < 2 || tokens[0] != "orca" {
		return false
	}
	if len(tokens) == 6 && tokens[1] == "terminal" && tokens[2] == "close" {
		if h.ClosedDisposition != handoff.DispositionWorkerFailed && h.ClosedDisposition != handoff.DispositionCancelled {
			return false
		}
		return cleanupStepAuthorized(h, "terminal_quiescent") && h.Orca != nil && h.Orca.WorkerTerminalHandle != "" && tokens[3] == "--terminal" && tokens[4] == h.Orca.WorkerTerminalHandle && tokens[5] == "--json"
	}
	if len(tokens) == 6 && tokens[1] == "terminal" && tokens[2] == "stop" {
		if h.ClosedDisposition != handoff.DispositionWorkerFailed && h.ClosedDisposition != handoff.DispositionCancelled {
			return false
		}
		return cleanupStepAuthorized(h, "terminal_quiescent") && h.Orca != nil && h.Orca.WorktreeID != "" && tokens[3] == "--worktree" && tokens[4] == "id:"+h.Orca.WorktreeID && tokens[5] == "--json"
	}
	if len(tokens) >= 3 && tokens[1] == "orchestration" && tokens[2] == "task-update" {
		if h.Orca == nil || h.Orca.TaskID == "" {
			return false
		}
		id, idOK := uniqueTokenFlag(tokens[3:], "--id")
		status, statusOK := uniqueTokenFlag(tokens[3:], "--status")
		result, resultOK := uniqueTokenFlag(tokens[3:], "--result")
		wantStatus := "failed"
		if h.ClosedDisposition == handoff.DispositionAccepted {
			wantStatus = "completed"
		} else if !cleanupStepAuthorized(h, "task_terminal") {
			return false
		}
		return idOK && id == h.Orca.TaskID && statusOK && status == wantStatus && (!resultOK || len(result) <= 4096) && onlyTokenFlags(tokens[3:], map[string]bool{"--id": true, "--status": true, "--result": true}, map[string]bool{"--json": true})
	}
	return false
}

func cleanupStepAuthorized(h *issueopsmodel.IssueOpsExecutionHandoff, step string) bool {
	if h == nil || h.Cleanup == nil {
		return false
	}
	switch step {
	case "task_terminal":
		return !cleanupReceiptExists(h, "task_terminal")
	case "terminal_quiescent":
		return cleanupReceiptExists(h, "task_terminal") && !cleanupReceiptExists(h, "terminal_quiescent")
	default:
		return false
	}
}

func cleanupReceiptExists(h *issueopsmodel.IssueOpsExecutionHandoff, step string) bool {
	if h == nil || h.Cleanup == nil {
		return false
	}
	for _, receipt := range h.Cleanup.Receipts {
		if receipt.Step == step {
			return true
		}
	}
	return false
}

func acceptedCoordinatorDownstreamCommand(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	h := record.ExecutionHandoff
	if h == nil || h.State != handoff.StateClosed || h.ClosedDisposition != handoff.DispositionAccepted || cleanAbsPath(req.CWD) != cleanAbsPath(record.Repo) || cleanAbsPath(req.Repo) != cleanAbsPath(record.Repo) ||
		!handoff.CoordinatorIdentityMatches(record, issueopsmodel.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}, req.CWD) {
		return false
	}
	if commandparse.HasUnquotedControlOperator(req.Command) || commandparse.HasActiveCommandSubstitution(req.Command) || commandparse.HasActiveOutputRedirect(req.Command) || commandparse.HasActiveParameterOrTildeExpansion(req.Command) || commandparse.HasActivePathnameExpansion(req.Command) || commandparse.HasActiveShellSpecialQuoting(req.Command) || commandparse.HasActiveZshEqualsExpansion(req.Command) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	if len(tokens) == 0 {
		return false
	}
	switch tokens[0] {
	case "git":
		return false
	case "gh":
		if len(tokens) <= 2 || tokens[1] != "pr" {
			return false
		}
		if tokens[2] == "create" {
			return false
		}
		return allowedAcceptedReviewSubcommand(tokens[2], map[string]bool{"view": true, "list": true, "status": true, "checks": true, "diff": true})
	case "glab":
		if len(tokens) <= 2 || tokens[1] != "mr" {
			return false
		}
		if tokens[2] == "create" {
			return false
		}
		return allowedAcceptedReviewSubcommand(tokens[2], map[string]bool{"view": true, "list": true, "status": true, "diff": true})
	case "agent-harness", "./bin/agent-harness":
		return acceptedIssueOpsDownstreamCommand(req, record)
	}
	return false
}

func acceptedIssueOpsDownstreamCommand(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	command, ok := commandparse.ParseExactIssueOpsCommand(req.Command)
	if !ok {
		return false
	}
	values := map[string]bool{}
	booleans := map[string]bool{"--json": true}
	repeatable := map[string]bool{}
	required := []string{"--id"}
	switch command.Path {
	case "phase":
		values["--id"], values["--to"] = true, true
		required = append(required, "--to")
	case "feedback add":
		for _, name := range []string{"--id", "--source", "--body", "--classification"} {
			values[name] = true
		}
		required = append(required, "--source", "--body")
	case "feedback resolve":
		for _, name := range []string{"--id", "--index", "--resolution"} {
			values[name] = true
		}
		required = append(required, "--index", "--resolution")
	case "feedback mark-issue-updated":
		values["--id"] = true
	case "pr-readiness":
		values["--id"] = true
		booleans["--strict"] = true
	case "ai-slop-clean record":
		for _, name := range []string{"--id", "--category", "--verification"} {
			values[name] = true
		}
		repeatable["--category"], repeatable["--verification"] = true, true
		required = append(required, "--category", "--verification")
	case "cleanup status":
		values["--id"] = true
		booleans["--merged"] = true
	case "remote verify-artifact":
		for _, name := range []string{"--id", "--provider", "--kind", "--url", "--label", "--labels", "--assignee", "--assignees"} {
			values[name] = true
		}
		for _, name := range []string{"--label", "--labels", "--assignee", "--assignees"} {
			repeatable[name] = true
		}
		required = append(required, "--provider", "--kind", "--url")
	case "remote create-pr":
		for _, name := range []string{"--id", "--provider", "--title", "--body", "--head", "--base", "--label", "--assignee"} {
			values[name] = true
		}
		repeatable["--label"], repeatable["--assignee"] = true, true
		booleans["--confirm"] = true
		required = append(required, "--provider", "--title", "--body", "--head", "--base", "--label", "--assignee")
	case "remote reconcile-create":
		for _, name := range []string{"--id", "--claim-id", "--coordinator-recipient", "--host", "--session-id", "--agent-id", "--source-cwd"} {
			values[name] = true
		}
		booleans["--confirm"], booleans["--approve-zero-clear"] = true, true
		required = append(required, "--claim-id", "--coordinator-recipient")
	default:
		return false
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return false
	}
	for _, name := range required {
		if repeatable[name] {
			if len(flags[name]) == 0 {
				return false
			}
			for _, value := range flags[name] {
				if value == "" {
					return false
				}
			}
			continue
		}
		value, present := oneFlag(flags, name)
		if !present || value == "" {
			return false
		}
	}
	id, _ := oneFlag(flags, "--id")
	if id != record.ID {
		return false
	}
	switch command.Path {
	case "phase":
		to, _ := oneFlag(flags, "--to")
		return to == "ai-slop-clean" || to == "feedback" || to == "pr"
	case "remote verify-artifact":
		provider, _ := oneFlag(flags, "--provider")
		kind, _ := oneFlag(flags, "--kind")
		labels := append(append([]string(nil), flags["--label"]...), flags["--labels"]...)
		assignees := append(append([]string(nil), flags["--assignee"]...), flags["--assignees"]...)
		return (provider == "github" && kind == "pr" || provider == "gitlab" && kind == "mr") && len(labels) > 0 && len(assignees) > 0
	case "remote create-pr":
		provider, _ := oneFlag(flags, "--provider")
		head, _ := oneFlag(flags, "--head")
		base, _ := oneFlag(flags, "--base")
		_, confirmed := oneFlag(flags, "--confirm")
		return confirmed && publicationReceiptMatches(record, provider, head, base)
	case "remote reconcile-create":
		claimID, _ := oneFlag(flags, "--claim-id")
		coordinator, _ := oneFlag(flags, "--coordinator-recipient")
		_, confirmed := oneFlag(flags, "--confirm")
		host, hok := oneFlag(flags, "--host")
		sessionID, sok := oneFlag(flags, "--session-id")
		agentID, aok := oneFlag(flags, "--agent-id")
		cwd, cwdOK := oneFlag(flags, "--source-cwd")
		agentMatches := strings.TrimSpace(req.AgentID) == "" && !aok || aok && agentID == strings.TrimSpace(req.AgentID)
		return confirmed && record.RemoteCreateClaim != nil && claimID == record.RemoteCreateClaim.ClaimID && record.ExecutionHandoff != nil && coordinator == record.ExecutionHandoff.CoordinatorMailboxHandle && hok && sok && cwdOK && strings.EqualFold(host, strings.TrimSpace(req.Host)) && sessionID == strings.TrimSpace(req.SessionID) && agentMatches && cleanAbsPath(cwd) == cleanAbsPath(record.Repo)
	default:
		return true
	}
}

func publicationReceiptMatches(record IssueOpsRecord, provider, head, base string) bool {
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.Result == nil || record.ExecutionHandoff.Orca == nil || record.ExecutionHandoff.PublishReceipt == nil || record.BranchPrepare == nil {
		return false
	}
	receipt := record.ExecutionHandoff.PublishReceipt
	branch := strings.TrimSpace(record.Branch)
	provider = strings.ToLower(strings.TrimSpace(provider))
	baseRef := strings.TrimSpace(record.ExecutionHandoff.Orca.BaseRef)
	prefix, suffix := "refs/remotes/", "/"+branch
	if !strings.HasPrefix(baseRef, prefix) || !strings.HasSuffix(baseRef, suffix) {
		return false
	}
	remote := strings.TrimSuffix(strings.TrimPrefix(baseRef, prefix), suffix)
	return provider != "" && provider == strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider)) &&
		strings.TrimSpace(head) == branch && strings.TrimSpace(base) == strings.TrimSpace(record.BranchPrepare.BaseBranch) &&
		receipt.Provider == provider && receipt.Remote == remote && receipt.Branch == branch && receipt.RemoteRef == "refs/heads/"+branch &&
		receipt.FinalHead == record.ExecutionHandoff.Result.FinalHead && receipt.VerifiedAt != ""
}

func allowedAcceptedReviewSubcommand(value string, allowed map[string]bool) bool {
	return allowed[value]
}

func acceptedGitHubPRCreate(tokens []string, record IssueOpsRecord) bool {
	if record.BranchPrepare == nil {
		return false
	}
	head, headOK := uniqueAliasedTokenFlag(tokens, "--head", "-H")
	base, baseOK := uniqueAliasedTokenFlag(tokens, "--base", "-B")
	if !headOK || !baseOK || head != strings.TrimSpace(record.Branch) || base != strings.TrimSpace(record.BranchPrepare.BaseBranch) || aliasedBoolCount(tokens, "--draft", "-d") != 1 {
		return false
	}
	return onlyAcceptedCreateFlags(tokens,
		map[string]bool{"--head": true, "-H": true, "--base": true, "-B": true, "--title": true, "--body": true, "--label": true, "--assignee": true, "--reviewer": true, "--milestone": true, "--project": true},
		map[string]bool{"--draft": true, "-d": true})
}

func acceptedGitLabMRCreate(tokens []string, record IssueOpsRecord) bool {
	if record.BranchPrepare == nil {
		return false
	}
	source, sourceOK := uniqueAliasedTokenFlag(tokens, "--source-branch", "-s")
	target, targetOK := uniqueAliasedTokenFlag(tokens, "--target-branch", "-b")
	if !sourceOK || !targetOK || source != strings.TrimSpace(record.Branch) || target != strings.TrimSpace(record.BranchPrepare.BaseBranch) || aliasedBoolCount(tokens, "--draft") != 1 {
		return false
	}
	return onlyAcceptedCreateFlags(tokens,
		map[string]bool{"--source-branch": true, "-s": true, "--target-branch": true, "-b": true, "--title": true, "--description": true, "--label": true, "--assignee": true, "--reviewer": true, "--milestone": true},
		map[string]bool{"--draft": true})
}

func uniqueAliasedTokenFlag(tokens []string, names ...string) (string, bool) {
	value, found := "", false
	for i := 0; i < len(tokens); i++ {
		for _, name := range names {
			if tokens[i] == name {
				if found || i+1 >= len(tokens) || strings.HasPrefix(tokens[i+1], "-") {
					return "", false
				}
				value, found = tokens[i+1], true
				i++
				break
			}
			if strings.HasPrefix(tokens[i], name+"=") {
				if found || strings.TrimPrefix(tokens[i], name+"=") == "" {
					return "", false
				}
				value, found = strings.TrimPrefix(tokens[i], name+"="), true
				break
			}
		}
	}
	return value, found
}

func aliasedBoolCount(tokens []string, names ...string) int {
	count := 0
	for _, token := range tokens {
		for _, name := range names {
			if token == name {
				count++
			}
		}
	}
	return count
}

func onlyAcceptedCreateFlags(tokens []string, valueFlags, boolFlags map[string]bool) bool {
	for i := 0; i < len(tokens); i++ {
		name := tokens[i]
		if at := strings.Index(name, "="); at >= 0 {
			if !valueFlags[name[:at]] || at == len(name)-1 {
				return false
			}
			continue
		}
		if boolFlags[name] {
			continue
		}
		if !valueFlags[name] || i+1 >= len(tokens) {
			return false
		}
		i++
	}
	return true
}

func currentWorkerBranchMatches(record IssueOpsRecord) bool {
	return record.ExecutionHandoff != nil && strings.TrimSpace(record.Branch) != "" && gitBranchFromHead(record.ExecutionHandoff.WorkerRoot) == strings.TrimSpace(record.Branch)
}

func claimedWorkerRoleViolation(command string) string {
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(command))
	if len(tokens) == 0 {
		return "empty worker shell command"
	}
	if gitTokens, ok := claimedWorkerGitTokens(tokens); ok {
		i := commandAfterGitRepositoryOptions(gitTokens, 1)
		if i < 0 || i >= len(gitTokens) {
			return "unresolved git command is not allowed for the worker"
		}
		switch gitTokens[i] {
		case "add", "commit", "status", "diff", "log", "show", "rev-parse":
			return ""
		case "branch":
			if len(gitTokens) == i+2 && gitTokens[i+1] == "--show-current" {
				return ""
			}
		}
		return "push, remote, branch switching, history rewrite, worktree, and cleanup operations are coordinator-owned"
	}
	protected := map[string]bool{"git": true, "gh": true, "glab": true, "orca": true, "agent-harness": true}
	first := tokens[0]
	base := filepath.Base(first)
	if protected[base] && first != base && first != "./bin/agent-harness" {
		return "executable aliases and path-shadowed controller commands are not allowed"
	}
	for i := 1; i < len(tokens); i++ {
		if protected[filepath.Base(tokens[i])] {
			return "wrapped controller commands are coordinator-owned"
		}
	}
	switch first {
	case "gh", "glab", "orca":
		return "remote, Orca, and cleanup controllers are coordinator-owned"
	case "agent-harness", "./bin/agent-harness":
		if len(tokens) > 1 && tokens[1] == "issueops" {
			return "IssueOps coordinator lifecycle commands are not worker implementation commands"
		}
	case "git":
		i := commandparse.CommandAfterDirectoryOption(tokens, 1)
		if i < 0 || i >= len(tokens) {
			return "unresolved git command is not allowed for the worker"
		}
		switch tokens[i] {
		case "add", "commit":
			return ""
		case "status", "diff", "log", "show", "rev-parse":
			return ""
		case "branch":
			if len(tokens) == i+2 && tokens[i+1] == "--show-current" {
				return ""
			}
		}
		return "push, remote, branch switching, history rewrite, worktree, and cleanup operations are coordinator-owned"
	}
	return ""
}

func claimedWorkerGitTokens(tokens []string) ([]string, bool) {
	i := 0
	if len(tokens) > 0 && tokens[0] == "env" {
		i++
	}
	for i < len(tokens) && (strings.HasPrefix(tokens[i], "GIT_DIR=") || strings.HasPrefix(tokens[i], "GIT_WORK_TREE=")) {
		if strings.TrimSpace(strings.SplitN(tokens[i], "=", 2)[1]) == "" {
			return nil, false
		}
		i++
	}
	if i >= len(tokens) || tokens[i] != "git" {
		return nil, false
	}
	return tokens[i:], true
}

func commandAfterGitRepositoryOptions(tokens []string, start int) int {
	for start < len(tokens) {
		token := tokens[start]
		switch {
		case token == "-C" || token == "--git-dir" || token == "--work-tree":
			if start+1 >= len(tokens) || strings.HasPrefix(tokens[start+1], "-") {
				return -1
			}
			start += 2
		case strings.HasPrefix(token, "-C=") || strings.HasPrefix(token, "--git-dir=") || strings.HasPrefix(token, "--work-tree="):
			if strings.TrimSpace(strings.SplitN(token, "=", 2)[1]) == "" {
				return -1
			}
			start++
		default:
			return start
		}
	}
	return -1
}

func uniqueTokenFlag(tokens []string, name string) (string, bool) {
	value, found := "", false
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == name {
			if found || i+1 >= len(tokens) || strings.HasPrefix(tokens[i+1], "--") {
				return "", false
			}
			value, found = tokens[i+1], true
			i++
		} else if strings.HasPrefix(tokens[i], name+"=") {
			if found {
				return "", false
			}
			value, found = strings.TrimPrefix(tokens[i], name+"="), true
		}
	}
	return value, found
}

func onlyTokenFlags(tokens []string, values, bools map[string]bool) bool {
	for i := 0; i < len(tokens); i++ {
		name := tokens[i]
		if at := strings.Index(name, "="); at >= 0 {
			name = name[:at]
		}
		if values[name] {
			if !strings.Contains(tokens[i], "=") {
				i++
			}
			continue
		}
		if bools[name] {
			continue
		}
		return false
	}
	return true
}

func buildExactClaimCommand(record IssueOpsRecord, req HookToolUseLifecycleRequest) string {
	h := record.ExecutionHandoff
	if h == nil || h.Orca == nil {
		return ""
	}
	parts := []string{"agent-harness issueops handoff claim", "--id " + shellGuidanceQuote(record.ID), "--attempt " + strconv.Itoa(h.Attempt), "--ownership-epoch " + shellGuidanceQuote(h.OwnershipEpoch), "--context-sha256 " + shellGuidanceQuote(h.ContextSHA256), "--host " + shellGuidanceQuote(req.Host), "--session-id " + shellGuidanceQuote(req.SessionID)}
	if strings.TrimSpace(req.AgentID) != "" {
		parts = append(parts, "--agent-id "+shellGuidanceQuote(req.AgentID))
	}
	parts = append(parts, "--cwd "+shellGuidanceQuote(h.WorkerRoot), "--orca-worktree-id "+shellGuidanceQuote(h.Orca.WorktreeID))
	return strings.Join(parts, " ")
}

// buildCoordinatorDispatchCommand renders the harness-authored identity-filled
// `handoff start` preview command for a coordinator_preparing handoff. The
// coordinator-identity flags are copied verbatim from the authenticated native
// event so identity is never agent-guessed; the emitted command satisfies the
// unchanged allowedExactHandoffLifecycleCommand fence (source checkout, exact
// coordinator host/session/agent, --source-cwd == record.Repo). Preview form
// omits --confirm; the guidance names the --expected-context-sha256/--confirm
// finalize step separately.
func buildCoordinatorDispatchCommand(record IssueOpsRecord, host, sessionID, agentID string) string {
	h := record.ExecutionHandoff
	if h == nil {
		return ""
	}
	parts := []string{"agent-harness issueops handoff start", "--id " + shellGuidanceQuote(record.ID)}
	if handle := strings.TrimSpace(h.CoordinatorMailboxHandle); handle != "" {
		parts = append(parts, "--coordinator-recipient "+shellGuidanceQuote(handle))
	}
	parts = append(parts, "--coordinator-host "+shellGuidanceQuote(host), "--coordinator-session-id "+shellGuidanceQuote(sessionID))
	if strings.TrimSpace(agentID) != "" {
		parts = append(parts, "--coordinator-agent-id "+shellGuidanceQuote(agentID))
	}
	parts = append(parts, "--source-cwd "+shellGuidanceQuote(record.Repo))
	return strings.Join(parts, " ")
}

func lifecycleRecordID(req HookToolUseLifecycleRequest) (string, bool) {
	if id, ok := exactLifecycleID(req.Command); ok {
		return id, true
	}
	if isHandoffMCPTool(req.Tool) {
		input, ok := flatMCPInput(req.ToolInput)
		if !ok {
			return "", false
		}
		id, ok := mcpString(input, "id")
		return id, ok && id != ""
	}
	return "", false
}

func selectSupervisedHandoffRecord(req HookToolUseLifecycleRequest) (IssueOpsRecord, bool, string) {
	records := []IssueOpsRecord{}
	for _, repo := range []string{req.Repo, req.SourceCheckout, sourceCheckoutFromWorktree(req.Repo), sourceCheckoutFromWorktree(req.CWD)} {
		if strings.TrimSpace(repo) != "" {
			records = append(records, supervisedHandoffGuardRecords(repo)...)
		}
	}
	byID := map[string]IssueOpsRecord{}
	for _, record := range records {
		if record.ExecutionHandoff != nil {
			byID[record.ID] = record
		}
	}
	records = records[:0]
	for _, record := range byID {
		records = append(records, record)
	}
	if len(records) == 0 {
		return IssueOpsRecord{}, false, ""
	}
	if terminalControlWriteRequest(req) {
		if handle, ok := literalSafeTerminalSendHandle(req); ok {
			matches := filterHandoffRecords(records, func(record IssueOpsRecord) bool {
				return record.ExecutionHandoff.Orca != nil && strings.TrimSpace(record.ExecutionHandoff.Orca.WorkerTerminalHandle) == handle
			})
			switch len(matches) {
			case 1:
				return matches[0], true, ""
			case 0:
				return IssueOpsRecord{}, false, "raw terminal steering does not match a persisted worker terminal handle"
			default:
				return IssueOpsRecord{}, false, "raw terminal steering is ambiguous across duplicate persisted worker terminal handles"
			}
		}
		if len(records) == 1 {
			return records[0], true, ""
		}
		return IssueOpsRecord{}, false, "raw terminal steering is ambiguous across active supervised IssueOps cycles"
	}
	cwd := cleanAbsPath(req.CWD)
	if matches := filterHandoffRecords(records, func(record IssueOpsRecord) bool { return cwd == cleanAbsPath(record.ExecutionHandoff.WorkerRoot) }); len(matches) == 1 {
		return matches[0], true, ""
	} else if len(matches) > 1 {
		return IssueOpsRecord{}, false, "ambiguous supervised IssueOps worker-root ownership"
	}
	if id, ok := lifecycleRecordID(req); ok {
		if record, exists := byID[id]; exists {
			return record, true, ""
		}
	}
	targets := worktreeGuardEditTargets(req)
	if matches := filterHandoffRecords(records, func(record IssueOpsRecord) bool {
		workerRoot := cleanAbsPath(record.ExecutionHandoff.WorkerRoot)
		for _, target := range targets {
			if pathWithin(target, workerRoot) {
				return true
			}
		}
		return false
	}); len(matches) == 1 {
		return matches[0], true, ""
	} else if len(matches) > 1 {
		return IssueOpsRecord{}, false, "ambiguous supervised IssueOps mutation target"
	}
	// Fence-scope narrowing (Task F2): the source-checkout fallback below binds
	// every unmatched command in the source checkout to the fenced record. A
	// command that explicitly names a *different* cycle id is provably unrelated
	// — any explicit id that reaches here missed the byID match above, so it is
	// not one of the supervised records fencing this checkout. Do not capture it
	// (this is the plan's single allowed-set change). id-less commands (bare
	// mutations, no lifecycle/MCP id) fall through and stay fenced by default,
	// so nothing without an explicit different target escapes. Worker-context
	// and same-id-as-stranded commands were already resolved above, so this only
	// unblocks source-checkout commands targeting another cycle.
	if _, hasExplicitID := lifecycleRecordID(req); hasExplicitID {
		return IssueOpsRecord{}, false, ""
	}
	sourceMatches := filterHandoffRecords(records, func(record IssueOpsRecord) bool { return cwd == cleanAbsPath(record.Repo) })
	if len(sourceMatches) == 1 {
		return sourceMatches[0], true, ""
	}
	if len(sourceMatches) > 1 {
		return IssueOpsRecord{}, false, "multiple active supervised IssueOps cycles share this source checkout; use an exact lifecycle --id or worker target"
	}
	return IssueOpsRecord{}, false, ""
}

func terminalControlWriteRequest(req HookToolUseLifecycleRequest) bool {
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	for _, suffix := range []string{
		"terminal_send", "terminal_stop", "terminal_create", "terminal_switch", "terminal_focus", "terminal_close", "terminal_rename", "terminal_split",
		"terminal_write", "terminal_input", "terminal_type", "terminal_paste",
	} {
		if strings.HasSuffix(tool, suffix) {
			return true
		}
	}
	if !searchrouting.IsShellTool(req.Tool) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	for i := 0; i+2 < len(tokens); i++ {
		if filepath.Base(tokens[i]) != "orca" || tokens[i+1] != "terminal" {
			continue
		}
		switch tokens[i+2] {
		case "send", "stop", "create", "switch", "focus", "close", "rename", "split", "write", "input", "type", "paste":
			return true
		}
	}
	return false
}

func invalidOrcaOrchestrationMessageTypeReason(req HookToolUseLifecycleRequest) string {
	if !searchrouting.IsShellTool(req.Tool) {
		return ""
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	if len(tokens) < 3 || tokens[0] != "orca" || tokens[1] != "orchestration" || tokens[2] != "send" {
		return ""
	}
	value, count := "", 0
	for i := 3; i < len(tokens); i++ {
		switch {
		case tokens[i] == "--type":
			count++
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "--") {
				value = tokens[i+1]
				i++
			}
		case strings.HasPrefix(tokens[i], "--type="):
			count++
			value = strings.TrimPrefix(tokens[i], "--type=")
		}
	}
	if count == 0 {
		return ""
	}
	if count == 1 {
		for _, allowed := range []string{"status", "dispatch", "worker_done", "merge_ready", "escalation", "handoff", "decision_gate", "heartbeat"} {
			if value == allowed {
				return ""
			}
		}
	}
	return "Orca orchestration message --type must be one of status, dispatch, worker_done, merge_ready, escalation, handoff, decision_gate, or heartbeat"
}

func unsafeOrcaMailboxInjectReason(req HookToolUseLifecycleRequest) string {
	if !searchrouting.IsShellTool(req.Tool) {
		return ""
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	if len(tokens) < 3 || tokens[0] != "orca" || tokens[1] != "orchestration" || tokens[2] != "check" {
		return ""
	}
	for _, token := range tokens[3:] {
		if token == "--inject" || strings.HasPrefix(token, "--inject=") {
			return "bulk mailbox injection is blocked; inspect first with orca orchestration check --all --json and select the exact current task, dispatch, and sequence"
		}
	}
	return ""
}

func linkedWorktreeDecisionGateReason(req HookToolUseLifecycleRequest) string {
	if !searchrouting.IsShellTool(req.Tool) {
		return ""
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	if len(tokens) < 3 || tokens[0] != "orca" || tokens[1] != "orchestration" || (tokens[2] != "ask" && tokens[2] != "gate-create") {
		return ""
	}
	for _, candidate := range []string{req.CWD, req.Repo} {
		root := cleanAbsPath(candidate)
		source := sourceCheckoutFromWorktree(root)
		if root != "" && source != "" && source != root {
			return "linked worktree workers must not create Orca decision gates; send one escalation, heartbeat, and wait for the source coordinator"
		}
	}
	return ""
}

func sourceCoordinatorTerminalSteeringAllowed(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	h := record.ExecutionHandoff
	if h == nil || h.State != handoff.StateClaimed || !searchrouting.IsShellTool(req.Tool) || cleanAbsPath(req.CWD) != cleanAbsPath(record.Repo) || cleanAbsPath(req.Repo) != cleanAbsPath(record.Repo) {
		return false
	}
	handle, ok := literalSafeTerminalSendHandle(req)
	if !ok {
		return false
	}
	persistedHandle := ""
	if h.Orca != nil {
		persistedHandle = strings.TrimSpace(h.Orca.WorkerTerminalHandle)
	}
	return persistedHandle != "" && handle == persistedHandle
}

func claimedWorkerProgressMessageAllowed(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	h := record.ExecutionHandoff
	if h == nil || h.State != handoff.StateClaimed || h.Orca == nil || !searchrouting.IsShellTool(req.Tool) ||
		cleanAbsPath(req.CWD) != cleanAbsPath(h.WorkerRoot) || cleanAbsPath(req.Repo) != cleanAbsPath(h.WorkerRoot) ||
		!nativeSessionMatches(req, h.WorkerSession) || !currentWorkerBranchMatches(record) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	if len(tokens) < 3 || tokens[0] != "orca" || tokens[1] != "orchestration" || tokens[2] != "send" {
		return false
	}
	flags, ok := commandparse.ExactFlags(commandparse.ExactIssueOpsCommand{Tokens: tokens, Start: 3}, map[string]bool{
		"--to": true, "--type": true, "--subject": true, "--body": true, "--task-id": true, "--dispatch-id": true, "--phase": true,
	}, map[string]bool{"--json": true}, map[string]bool{})
	if !ok {
		return false
	}
	to, toOK := oneFlag(flags, "--to")
	messageType, typeOK := oneFlag(flags, "--type")
	subject, subjectOK := oneFlag(flags, "--subject")
	taskID, taskOK := oneFlag(flags, "--task-id")
	dispatchID, dispatchOK := oneFlag(flags, "--dispatch-id")
	body, bodyOK := oneFlag(flags, "--body")
	phase, phaseOK := oneFlag(flags, "--phase")
	if !toOK || to != h.CoordinatorMailboxHandle || !typeOK || !subjectOK || strings.TrimSpace(subject) == "" || len(subject) > 256 || commandparse.ContainsASCIITerminalControl(subject) ||
		!taskOK || taskID != h.Orca.TaskID || !dispatchOK || dispatchID != h.Orca.DispatchID {
		return false
	}
	switch messageType {
	case "heartbeat":
		return phaseOK && !bodyOK && strings.TrimSpace(phase) != "" && len(phase) <= 256 && !commandparse.ContainsASCIITerminalControl(phase)
	case "status", "escalation":
		return bodyOK && !phaseOK && strings.TrimSpace(body) != "" && len(body) <= 4096 && !commandparse.ContainsASCIITerminalControl(body)
	default:
		return false
	}
}

func literalSafeTerminalSendHandle(req HookToolUseLifecycleRequest) (string, bool) {
	if !searchrouting.IsShellTool(req.Tool) {
		return "", false
	}
	if commandparse.HasUnquotedControlOperator(req.Command) || commandparse.HasActiveCommandSubstitution(req.Command) || commandparse.HasActiveOutputRedirect(req.Command) || commandparse.HasActiveParameterOrTildeExpansion(req.Command) || commandparse.HasActivePathnameExpansion(req.Command) || commandparse.HasActiveShellSpecialQuoting(req.Command) || commandparse.HasActiveZshEqualsExpansion(req.Command) {
		return "", false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	if len(tokens) != 9 || tokens[0] != "orca" || tokens[1] != "terminal" || tokens[2] != "send" || tokens[3] != "--terminal" || tokens[5] != "--text" || tokens[7] != "--enter" || tokens[8] != "--json" {
		return "", false
	}
	handle, guidance := strings.TrimSpace(tokens[4]), tokens[6]
	if handle == "" || len(handle) > 256 || !strings.HasPrefix(guidance, "# agent-harness guidance: ") || len(guidance) > 4096 || commandparse.ContainsASCIITerminalControl(guidance) {
		return "", false
	}
	if commandparse.HasUnquotedControlOperator(guidance) || commandparse.HasActiveCommandSubstitution(guidance) || commandparse.HasActiveOutputRedirect(guidance) || commandparse.HasActiveParameterOrTildeExpansion(guidance) || commandparse.HasActivePathnameExpansion(guidance) || commandparse.HasActiveShellSpecialQuoting(guidance) || commandparse.HasActiveZshEqualsExpansion(guidance) {
		return "", false
	}
	return handle, true
}

func supervisedHandoffGuardRecords(repo string) []IssueOpsRecord {
	records := append([]IssueOpsRecord(nil), ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)...)
	records = append(records, ActiveIssueOpsSupervisedHandoffCyclesForRepo(repo)...)
	return records
}

func filterHandoffRecords(records []IssueOpsRecord, predicate func(IssueOpsRecord) bool) []IssueOpsRecord {
	result := make([]IssueOpsRecord, 0, len(records))
	for _, record := range records {
		if predicate(record) {
			result = append(result, record)
		}
	}
	return result
}

func handoffMCPToolKind(tool string) (string, bool) {
	for _, name := range []string{"issueops_handoff", "issueops_heartbeat", "issueops_remote_create_pr", "issueops_remote_reconcile_create"} {
		if tool == name || tool == "mcp__agent_harness__"+name {
			return name, true
		}
	}
	return "", false
}

func isHandoffMCPTool(tool string) bool {
	_, ok := handoffMCPToolKind(tool)
	return ok
}

func explicitHandoffReadOnlyTool(tool string) bool {
	tool = strings.ToLower(strings.TrimSpace(tool))
	switch tool {
	case "read", "glob", "grep", "search", "list", "ls":
		return true
	}
	for _, suffix := range []string{
		"__read_file", "__read_text_file", "__list_directory", "__list_files", "__search_files",
		"__codegraph_explore", "__get_library_docs", "__resolve_library_id",
	} {
		if strings.HasSuffix(tool, suffix) {
			return true
		}
	}
	return false
}

func flatMCPInput(input map[string]any) (map[string]any, bool) {
	if input == nil {
		return nil, false
	}
	flat := make(map[string]any, len(input))
	for key, value := range input {
		if key == "flags" {
			continue
		}
		flat[key] = value
	}
	if flags, ok := input["flags"].(map[string]any); ok {
		for key, value := range flags {
			if _, duplicate := flat[key]; duplicate {
				return nil, false
			}
			flat[key] = value
		}
	} else if _, exists := input["flags"]; exists {
		return nil, false
	}
	return flat, true
}

func mcpString(input map[string]any, key string) (string, bool) {
	value, ok := input[key].(string)
	return value, ok
}

func mcpNonEmptyStringList(input map[string]any, key string) bool {
	values, ok := input[key].([]any)
	if !ok || len(values) == 0 {
		return false
	}
	for _, value := range values {
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return false
		}
	}
	return true
}

func mcpInt(input map[string]any, key string) (int, bool) {
	switch value := input[key].(type) {
	case int:
		return value, true
	case float64:
		return int(value), value == float64(int(value))
	default:
		return 0, false
	}
}

func allowedHandoffMCPTool(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	tool, recognized := handoffMCPToolKind(req.Tool)
	if !recognized {
		return false
	}
	input, ok := flatMCPInput(req.ToolInput)
	if !ok || record.ExecutionHandoff == nil {
		return false
	}
	id, ok := mcpString(input, "id")
	if !ok || id != record.ID {
		return false
	}
	h := record.ExecutionHandoff
	worker := cleanAbsPath(req.CWD) == cleanAbsPath(h.WorkerRoot)
	source := cleanAbsPath(req.CWD) == cleanAbsPath(record.Repo)
	coordinator := source && h.State == handoff.StateClosed && h.ClosedDisposition == handoff.DispositionAccepted && handoff.CoordinatorIdentityMatches(record, issueopsmodel.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}, req.CWD)
	if tool == "issueops_remote_create_pr" {
		provider, pok := mcpString(input, "provider")
		head, hok := mcpString(input, "head")
		base, bok := mcpString(input, "base")
		confirm, cok := input["confirm"].(bool)
		title, tok := mcpString(input, "title")
		body, bodyOK := mcpString(input, "body")
		return coordinator && pok && strings.TrimSpace(provider) != "" && hok && strings.TrimSpace(head) != "" && bok && strings.TrimSpace(base) != "" && cok && confirm && tok && strings.TrimSpace(title) != "" && bodyOK && strings.TrimSpace(body) != "" && mcpNonEmptyStringList(input, "labels") && mcpNonEmptyStringList(input, "assignees") && publicationReceiptMatches(record, provider, head, base)
	}
	if tool == "issueops_remote_reconcile_create" {
		claimID, claimOK := mcpString(input, "claim_id")
		recipient, recipientOK := mcpString(input, "coordinator_recipient")
		confirm, confirmOK := input["confirm"].(bool)
		host, hostOK := mcpString(input, "host")
		sessionID, sessionOK := mcpString(input, "session_id")
		agentID, agentOK := mcpString(input, "agent_id")
		cwd, cwdOK := mcpString(input, "source_cwd")
		agentMatches := strings.TrimSpace(req.AgentID) == "" && !agentOK || agentOK && agentID == strings.TrimSpace(req.AgentID)
		return coordinator && record.RemoteCreateClaim != nil && claimOK && claimID == record.RemoteCreateClaim.ClaimID && recipientOK && recipient == h.CoordinatorMailboxHandle && confirmOK && confirm && hostOK && strings.EqualFold(host, strings.TrimSpace(req.Host)) && sessionOK && sessionID == strings.TrimSpace(req.SessionID) && agentMatches && cwdOK && cleanAbsPath(cwd) == cleanAbsPath(record.Repo)
	}
	if tool == "issueops_heartbeat" {
		return worker && currentWorkerBranchMatches(record) && mcpFenceMatches(input, record) && mcpEventIdentityMatches(input, req) && nativeSessionMatches(req, h.WorkerSession)
	}
	action, ok := mcpString(input, "action")
	if !ok {
		return false
	}
	switch action {
	case "start":
		host, hostOK := mcpString(input, "coordinator_host")
		sessionID, sessionOK := mcpString(input, "coordinator_session_id")
		agentID, agentOK := mcpString(input, "coordinator_agent_id")
		cwd, cwdOK := mcpString(input, "source_cwd")
		agentMatches := strings.TrimSpace(req.AgentID) == "" && !agentOK || agentOK && agentID == strings.TrimSpace(req.AgentID)
		return source && coordinatorLifecycleStateAllows("handoff start", record) && hostOK && strings.EqualFold(host, strings.TrimSpace(req.Host)) && sessionOK && sessionID == strings.TrimSpace(req.SessionID) && agentMatches && cwdOK && cleanAbsPath(cwd) == cleanAbsPath(record.Repo)
	case "accept":
		cwd, cwdOK := mcpString(input, "source_cwd")
		return source && coordinatorLifecycleStateAllows("handoff accept", record) && mcpFenceMatches(input, record) && mcpEventIdentityMatches(input, req) && cwdOK && cleanAbsPath(cwd) == cleanAbsPath(record.Repo) && handoff.CoordinatorIdentityMatches(record, issueopsmodel.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}, req.CWD)
	case "recover":
		return source && coordinatorLifecycleStateAllows("handoff recover", record)
	case "publish":
		cwd, cwdOK := mcpString(input, "source_cwd")
		confirm, confirmOK := input["confirm"].(bool)
		approveLegacySeal, _ := input["approve_legacy_coordinator_seal"].(bool)
		native := issueopsmodel.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}
		legacySeal := approveLegacySeal && confirmOK && confirm && handoff.LegacyCoordinatorIdentityCanBeSealed(record, native, req.CWD)
		return source && coordinatorLifecycleStateAllows("handoff publish", record) && mcpEventIdentityMatches(input, req) && cwdOK && cleanAbsPath(cwd) == cleanAbsPath(record.Repo) && (handoff.CoordinatorIdentityMatches(record, native, req.CWD) || legacySeal)
	case "claim":
		cwd, cwdOK := mcpString(input, "cwd")
		wt, wtOK := mcpString(input, "orca_worktree_id")
		return worker && currentWorkerBranchMatches(record) && h.State == handoff.StateDispatched && mcpFenceMatches(input, record) && mcpEventIdentityMatches(input, req) && cwdOK && cleanAbsPath(cwd) == cleanAbsPath(h.WorkerRoot) && wtOK && h.Orca != nil && wt == h.Orca.WorktreeID
	case "finish":
		return worker && currentWorkerBranchMatches(record) && mcpFenceMatches(input, record) && mcpEventIdentityMatches(input, req) && nativeSessionMatches(req, h.WorkerSession)
	default:
		return false
	}
}

func mcpFenceMatches(input map[string]any, record IssueOpsRecord) bool {
	attempt, aok := mcpInt(input, "attempt")
	epoch, eok := mcpString(input, "ownership_epoch")
	contextSHA, cok := mcpString(input, "context_sha256")
	h := record.ExecutionHandoff
	return h != nil && aok && attempt == h.Attempt && eok && epoch == h.OwnershipEpoch && cok && contextSHA == h.ContextSHA256
}

func mcpEventIdentityMatches(input map[string]any, req HookToolUseLifecycleRequest) bool {
	host, hok := mcpString(input, "host")
	session, sok := mcpString(input, "session_id")
	if !hok || !sok || !strings.EqualFold(host, strings.TrimSpace(req.Host)) || session != strings.TrimSpace(req.SessionID) {
		return false
	}
	agent, exists := input["agent_id"]
	if strings.TrimSpace(req.AgentID) == "" {
		return !exists
	}
	value, ok := agent.(string)
	return ok && value == strings.TrimSpace(req.AgentID)
}
