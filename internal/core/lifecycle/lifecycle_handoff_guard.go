package lifecycle

import (
	"fmt"
	"strconv"
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/searchrouting"
)

func BuildIssueOpsHandoffSessionGuidance(repo, host, sessionID, agentID string) string {
	records := ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
	for _, record := range records {
		h := record.ExecutionHandoff
		if h == nil {
			continue
		}
		if cleanAbsPath(repo) == cleanAbsPath(h.WorkerRoot) {
			resume := "agent-harness issueops resume --repo " + shellGuidanceQuote(record.Repo) + " --id " + shellGuidanceQuote(record.ID)
			if h.State != handoff.StateDispatched {
				return fmt.Sprintf("IssueOps supervised handoff role=worker state=%s attempt=%d context=%s. Resume: %s", h.State, h.Attempt, h.ContextSHA256, resume)
			}
			if h.Orca == nil || strings.TrimSpace(h.Orca.WorktreeID) == "" {
				return fmt.Sprintf("IssueOps supervised handoff role=worker state=%s attempt=%d context=%s. External identity requires coordinator recovery before claim.", h.State, h.Attempt, h.ContextSHA256)
			}
			claimParts := []string{
				"agent-harness issueops handoff claim",
				"--id " + shellGuidanceQuote(record.ID),
				"--attempt " + strconv.Itoa(h.Attempt),
				"--ownership-epoch " + shellGuidanceQuote(h.OwnershipEpoch),
				"--context-sha256 " + shellGuidanceQuote(h.ContextSHA256),
				"--host " + shellGuidanceQuote(host),
				"--session-id " + shellGuidanceQuote(sessionID),
			}
			if strings.TrimSpace(agentID) != "" {
				claimParts = append(claimParts, "--agent-id "+shellGuidanceQuote(agentID))
			}
			claimParts = append(claimParts,
				"--cwd "+shellGuidanceQuote(h.WorkerRoot),
				"--orca-worktree-id "+shellGuidanceQuote(h.Orca.WorktreeID),
			)
			claim := strings.Join(claimParts, " ")
			return fmt.Sprintf("IssueOps supervised handoff role=worker state=%s attempt=%d context=%s. Claim before editing: %s. Read-only resume: %s", h.State, h.Attempt, h.ContextSHA256, claim, resume)
		}
		if cleanAbsPath(repo) == cleanAbsPath(record.Repo) {
			return fmt.Sprintf("IssueOps supervised handoff role=coordinator state=%s attempt=%d context=%s. Inspect without mutation: agent-harness issueops resume --repo %s --id %s", h.State, h.Attempt, h.ContextSHA256, shellGuidanceQuote(record.Repo), shellGuidanceQuote(record.ID))
		}
	}
	return ""
}

func shellGuidanceQuote(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func handoffOwnershipBlockReason(req HookToolUseLifecycleRequest) (bool, string) {
	if !req.EnforceWorktree {
		return false, ""
	}
	record, ok := supervisedHandoffRecord(req)
	if !ok {
		return false, ""
	}
	if isHandoffLifecycleCommand(req.Command) {
		if allowedHandoffLifecycleCommand(req, record) {
			return true, ""
		}
		return true, "supervised IssueOps handoff lifecycle command flags do not match the native session and persisted fence"
	}
	if !toolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return false, ""
	}
	h := record.ExecutionHandoff
	if h.State != handoff.StateClaimed || h.WorkerSession == nil {
		return true, "supervised IssueOps handoff must be claimed by the dispatched worker before implementation mutation"
	}
	if strings.ToLower(strings.TrimSpace(req.Host)) != strings.ToLower(h.WorkerSession.Host) || strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.SessionID) != h.WorkerSession.SessionID {
		return true, "supervised IssueOps handoff mutation is restricted to the claimed native worker session"
	}
	workerRoot := cleanAbsPath(h.WorkerRoot)
	if cleanAbsPath(req.CWD) != workerRoot || cleanAbsPath(req.Repo) != workerRoot {
		return true, "supervised IssueOps worker must mutate from the canonical worker worktree root"
	}
	for _, target := range worktreeGuardEditTargets(req) {
		if !pathWithin(target, workerRoot) {
			return true, "supervised IssueOps worker mutation target is outside the claimed worker worktree"
		}
	}
	return true, ""
}

func supervisedHandoffRecord(req HookToolUseLifecycleRequest) (IssueOpsRecord, bool) {
	records := ActiveIssueOpsLinkedWorktreeCyclesForRepo(req.Repo)
	if len(records) == 0 && strings.TrimSpace(req.SourceCheckout) != "" {
		records = ActiveIssueOpsLinkedWorktreeCyclesForRepo(req.SourceCheckout)
	}
	for _, record := range records {
		if record.ExecutionHandoff == nil {
			continue
		}
		workerRoot := cleanAbsPath(record.ExecutionHandoff.WorkerRoot)
		cwd := cleanAbsPath(req.CWD)
		if cwd == workerRoot || cwd == cleanAbsPath(record.Repo) {
			return record, true
		}
		for _, target := range worktreeGuardEditTargets(req) {
			if pathWithin(target, workerRoot) || pathWithin(target, record.Repo) {
				return record, true
			}
		}
	}
	return IssueOpsRecord{}, false
}

func allowedHandoffLifecycleCommand(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	command := strings.TrimSpace(req.Command)
	if command == "" || strings.ContainsAny(command, ";&|\n\r") {
		return false
	}
	tokens := commandparse.SplitCommandTokens(command)
	issueops := -1
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) == "issueops" {
			issueops = i
			break
		}
	}
	if issueops < 0 || issueops+1 >= len(tokens) || !flagMatches(tokens, "--id", record.ID) {
		return false
	}
	h := record.ExecutionHandoff
	source := cleanAbsPath(req.CWD) == cleanAbsPath(record.Repo)
	worker := cleanAbsPath(req.CWD) == cleanAbsPath(h.WorkerRoot)
	switch searchrouting.SearchTokenName(tokens[issueops+1]) {
	case "worktree":
		return source && issueops+2 < len(tokens) && searchrouting.SearchTokenName(tokens[issueops+2]) == "prepare"
	case "heartbeat":
		return worker && matchingClaimedSession(req, record)
	case "handoff":
		if issueops+2 >= len(tokens) {
			return false
		}
		switch searchrouting.SearchTokenName(tokens[issueops+2]) {
		case "start", "recover", "accept":
			return source
		case "claim":
			return worker && h.State == handoff.StateDispatched && claimCommandMatchesNativeIdentity(req, h, record.ID, tokens)
		case "finish":
			return worker && matchingClaimedSession(req, record)
		}
	}
	return false
}

func isHandoffLifecycleCommand(command string) bool {
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(command))
	issueops := -1
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) == "issueops" {
			issueops = i
			break
		}
	}
	if issueops < 0 || issueops+1 >= len(tokens) {
		return false
	}
	switch searchrouting.SearchTokenName(tokens[issueops+1]) {
	case "heartbeat":
		return true
	case "worktree":
		return issueops+2 < len(tokens) && searchrouting.SearchTokenName(tokens[issueops+2]) == "prepare"
	case "handoff":
		if issueops+2 >= len(tokens) {
			return false
		}
		switch searchrouting.SearchTokenName(tokens[issueops+2]) {
		case "start", "recover", "accept", "claim", "finish":
			return true
		}
	}
	return false
}

func matchingClaimedSession(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	h := record.ExecutionHandoff
	return h != nil && h.State == handoff.StateClaimed && h.WorkerSession != nil &&
		strings.EqualFold(strings.TrimSpace(req.Host), h.WorkerSession.Host) && strings.TrimSpace(req.SessionID) == h.WorkerSession.SessionID
}

func claimCommandMatchesNativeIdentity(req HookToolUseLifecycleRequest, h *issueopsmodel.IssueOpsExecutionHandoff, recordID string, tokens []string) bool {
	if h == nil || h.Orca == nil || strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.SessionID) == "" {
		return false
	}
	id, idOK := uniqueFlagValue(tokens, "--id")
	attempt, attemptOK := uniqueFlagValue(tokens, "--attempt")
	epoch, epochOK := uniqueFlagValue(tokens, "--ownership-epoch")
	contextSHA, contextOK := uniqueFlagValue(tokens, "--context-sha256")
	host, hostOK := uniqueFlagValue(tokens, "--host")
	sessionID, sessionOK := uniqueFlagValue(tokens, "--session-id")
	cwd, cwdOK := uniqueFlagValue(tokens, "--cwd")
	worktreeID, worktreeOK := uniqueFlagValue(tokens, "--orca-worktree-id")
	if !idOK || id != recordID ||
		!attemptOK || attempt != strconv.Itoa(h.Attempt) ||
		!epochOK || epoch != h.OwnershipEpoch ||
		!contextOK || contextSHA != h.ContextSHA256 ||
		!hostOK || !strings.EqualFold(host, strings.TrimSpace(req.Host)) ||
		!sessionOK || sessionID != strings.TrimSpace(req.SessionID) ||
		!cwdOK || cleanAbsPath(cwd) != cleanAbsPath(h.WorkerRoot) ||
		!worktreeOK || worktreeID != strings.TrimSpace(h.Orca.WorktreeID) {
		return false
	}
	agentID, agentOK := uniqueFlagValue(tokens, "--agent-id")
	if strings.TrimSpace(req.AgentID) == "" {
		return !agentOK || strings.TrimSpace(agentID) == ""
	}
	return agentOK && agentID == strings.TrimSpace(req.AgentID)
}

func uniqueFlagValue(tokens []string, name string) (string, bool) {
	value := ""
	found := false
	for i, token := range tokens {
		candidate := ""
		matched := false
		switch {
		case token == name && i+1 < len(tokens):
			if strings.HasPrefix(tokens[i+1], "--") {
				return "", false
			}
			candidate, matched = tokens[i+1], true
		case strings.HasPrefix(token, name+"="):
			candidate, matched = strings.TrimPrefix(token, name+"="), true
		}
		if !matched {
			continue
		}
		if found {
			return "", false
		}
		value, found = candidate, true
	}
	return value, found
}

func flagMatches(tokens []string, name, want string) bool {
	for i, token := range tokens {
		if token == name && i+1 < len(tokens) && tokens[i+1] == want {
			return true
		}
		if strings.HasPrefix(token, name+"=") && strings.TrimPrefix(token, name+"=") == want {
			return true
		}
	}
	return false
}
