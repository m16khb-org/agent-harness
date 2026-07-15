package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/searchrouting"
)

func BuildIssueOpsHandoffSessionGuidance(repo, host, sessionID, agentID string) string {
	records := uniqueSupervisedHandoffRecords(supervisedHandoffGuardRecords(repo))
	target := cleanAbsPath(repo)
	workerMatches := filterHandoffRecords(records, func(record IssueOpsRecord) bool {
		return record.ExecutionHandoff != nil && target == cleanAbsPath(record.ExecutionHandoff.WorkerRoot)
	})
	if len(workerMatches) == 1 {
		return renderHandoffSessionGuidance(workerMatches[0], true, host, sessionID, agentID)
	}
	if len(workerMatches) > 1 {
		return ambiguousHandoffSessionGuidance(workerMatches, "worker root")
	}
	sourceMatches := filterHandoffRecords(records, func(record IssueOpsRecord) bool {
		return target == cleanAbsPath(record.Repo)
	})
	if len(sourceMatches) == 1 {
		return renderHandoffSessionGuidance(sourceMatches[0], false, host, sessionID, agentID)
	}
	if len(sourceMatches) > 1 {
		return ambiguousHandoffSessionGuidance(sourceMatches, "source checkout")
	}
	return ""
}

func uniqueSupervisedHandoffRecords(records []IssueOpsRecord) []IssueOpsRecord {
	byID := map[string]IssueOpsRecord{}
	for _, record := range records {
		if record.ExecutionHandoff != nil {
			byID[record.ID] = record
		}
	}
	unique := make([]IssueOpsRecord, 0, len(byID))
	for _, record := range byID {
		unique = append(unique, record)
	}
	sort.Slice(unique, func(i, j int) bool {
		if unique[i].Branch != unique[j].Branch {
			return unique[i].Branch < unique[j].Branch
		}
		return unique[i].ID < unique[j].ID
	})
	return unique
}

func renderHandoffSessionGuidance(record IssueOpsRecord, worker bool, host, sessionID, agentID string) string {
	h := record.ExecutionHandoff
	if h == nil {
		return ""
	}
	if record.Invalid {
		return "IssueOps supervised handoff durable record is invalid; remain read-only and require coordinator recovery: " + record.InvalidReason
	}
	if err := handoff.ValidateEnvelope(record); err != nil {
		return "IssueOps supervised handoff envelope is invalid; remain read-only and require coordinator recovery before claim, heartbeat, finish, or implementation mutation."
	}
	if !worker {
		resume := "agent-harness issueops resume --repo " + shellGuidanceQuote(record.Repo) + " --id " + shellGuidanceQuote(record.ID)
		if h.State == handoff.StateCoordinatorPreparing {
			// Coordinator-dispatch reachability (Task G1): the hook holds the
			// authenticated native identity, so it authors the identity-filled
			// `handoff start` preview command. The coordinator runs it verbatim
			// after appending its sealed cycle packet; identity is never guessed
			// and the unchanged fence still gates the emitted command.
			start := buildCoordinatorDispatchCommand(record, host, sessionID, agentID)
			return fmt.Sprintf("IssueOps supervised handoff role=coordinator state=%s attempt=%d context=%s. Dispatch the worker with this harness-authored identity-filled preview, then append your sealed --criteria-id/--required-doc/--required-skill/--worker-scope/--verification packet, review the returned context_sha256, and finalize with --expected-context-sha256 <preview context_sha256> --confirm: %s. Inspect without mutation: %s", h.State, h.Attempt, h.ContextSHA256, start, resume)
		}
		return fmt.Sprintf("IssueOps supervised handoff role=coordinator state=%s attempt=%d context=%s. Inspect without mutation: %s", h.State, h.Attempt, h.ContextSHA256, resume)
	}
	modelBoundary := " Host usage-limit, rate-limit, reset, and model-selection prompts are user-decision boundaries: dismiss or stop and relay; never auto switch models or reset usage."
	resume := "agent-harness issueops resume --repo " + shellGuidanceQuote(record.Repo) + " --id " + shellGuidanceQuote(record.ID)
	if h.State != handoff.StateDispatched {
		return fmt.Sprintf("IssueOps supervised handoff role=worker state=%s attempt=%d context=%s. Resume: %s", h.State, h.Attempt, h.ContextSHA256, resume) + modelBoundary
	}
	if h.Orca == nil || strings.TrimSpace(h.Orca.WorktreeID) == "" {
		return fmt.Sprintf("IssueOps supervised handoff role=worker state=%s attempt=%d context=%s. External identity requires coordinator recovery before claim.", h.State, h.Attempt, h.ContextSHA256) + modelBoundary
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
	return fmt.Sprintf("IssueOps supervised handoff role=worker state=%s attempt=%d context=%s. Claim before editing: %s. Read-only resume: %s", h.State, h.Attempt, h.ContextSHA256, claim, resume) + modelBoundary
}

func ambiguousHandoffSessionGuidance(records []IssueOpsRecord, location string) string {
	ids := make([]string, 0, 8)
	for _, record := range records {
		id := strings.TrimSpace(record.ID)
		if len(ids) == 8 {
			ids = append(ids, "...")
			break
		}
		if len(id) > 128 || !strings.HasPrefix(id, "io-") {
			id = "<invalid-id>"
		}
		ids = append(ids, id)
	}
	return fmt.Sprintf("Multiple active supervised IssueOps cycles match this %s: %s. Remain read-only; select one exact cycle ID with issueops status --id or issueops resume --id before proceeding.", location, strings.Join(ids, ", "))
}

func shellGuidanceQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func handoffOwnershipBlockReason(req HookToolUseLifecycleRequest) (bool, string) {
	if !req.EnforceWorktree {
		return false, ""
	}
	record, ok, selectionReason := selectSupervisedHandoffRecord(req)
	if selectionReason != "" {
		return true, selectionReason
	}
	if !ok {
		return false, ""
	}
	if record.Invalid {
		if allowedInvalidLegacyV5PublicationSeal(req, record) {
			return true, ""
		}
		return true, "invalid supervised IssueOps durable record: " + record.InvalidReason
	}
	if searchrouting.IsShellTool(req.Tool) && (commandparse.HasUnquotedControlOperator(req.Command) || commandparse.HasActiveCommandSubstitution(req.Command) || commandparse.HasActiveOutputRedirect(req.Command) || commandparse.HasActiveParameterOrTildeExpansion(req.Command) || commandparse.HasActivePathnameExpansion(req.Command) || commandparse.HasActiveShellSpecialQuoting(req.Command) || commandparse.HasActiveZshEqualsExpansion(req.Command)) {
		return true, "active shell control, command/process substitution, parameter/tilde expansion, pathname expansion, and output redirection are forbidden during a supervised IssueOps handoff; pass freeform text as argv-safe data or POSIX single-quoted literal data with an explicit canonical path"
	}
	if err := handoff.ValidateEnvelope(record); err != nil {
		return true, "invalid supervised IssueOps handoff envelope: " + err.Error()
	}
	if claimedWorkerProgressMessageAllowed(req, record) {
		return true, ""
	}
	if terminalControlWriteRequest(req) {
		if allowedClosedOrcaCleanup(req, record) {
			return true, ""
		}
		if sourceCoordinatorTerminalSteeringAllowed(req, record) {
			return true, ""
		}
		return true, "raw terminal steering is blocked outside literal-safe claimed-worker guidance from the exact source coordinator root; use issueops handoff start for prepare and dispatch"
	}
	if isHandoffMCPTool(req.Tool) {
		if allowedHandoffMCPTool(req, record) {
			return true, ""
		}
		if record.ExecutionHandoff.State == handoff.StateClaimed && cleanAbsPath(req.CWD) == cleanAbsPath(record.ExecutionHandoff.WorkerRoot) {
			return true, "supervised IssueOps role=worker may use exact heartbeat or handoff finish only; coordinator owns start, recover, accept, phase, remote publish, and cleanup"
		}
		return true, "supervised IssueOps MCP lifecycle payload does not match the native session, actor, and persisted fence"
	}
	if acceptedCoordinatorDownstreamCommand(req, record) {
		return true, ""
	}
	if isHandoffLifecycleCommand(req.Command) {
		if allowedExactHandoffLifecycleCommand(req, record) {
			return true, ""
		}
		if record.ExecutionHandoff.State == handoff.StateClaimed && cleanAbsPath(req.CWD) == cleanAbsPath(record.ExecutionHandoff.WorkerRoot) {
			return true, "supervised IssueOps role=worker may run exact status, resume, heartbeat, or handoff finish only; coordinator owns start, recover, accept, phase, remote publish, and cleanup"
		}
		return true, "supervised IssueOps handoff lifecycle command flags do not match the native session and persisted fence"
	}
	if searchrouting.IsShellTool(req.Tool) {
		if exactReadOnlyShellCommand(req, record) || allowedClosedOrcaCleanup(req, record) {
			return true, ""
		}
	} else if explicitHandoffReadOnlyTool(req.Tool) {
		return true, ""
	}
	if protectedWorkerRootMutation(req, record) {
		return true, "supervised IssueOps worker cannot delete, move, or change permissions on the canonical worker root or its Git metadata; coordinator recovery owns lifecycle cleanup"
	}
	h := record.ExecutionHandoff
	if coordinatorPlanMutationAllowed(req, record) {
		return true, ""
	}
	if coordinatorPlanGitCommandAllowed(req, record) {
		return true, ""
	}
	if record.Phase != issueopsmodel.IssueOpsPhaseImplement && h.State == handoff.StateClaimed {
		return true, "supervised IssueOps handoff cannot authorize implementation mutation before the durable implement phase"
	}
	if h.State != handoff.StateClaimed || h.WorkerSession == nil {
		if h.State == handoff.StateDispatched && cleanAbsPath(req.CWD) == cleanAbsPath(h.WorkerRoot) {
			claim := buildExactClaimCommand(record, req)
			if claim != "" {
				return true, "supervised IssueOps handoff must be claimed before mutation; run only: " + claim
			}
		}
		if h.State == handoff.StateCoordinatorPreparing || h.State == handoff.StateRecoveryRequired {
			return true, "supervised IssueOps handoff is not dispatched; remain read-only and poll: agent-harness issueops resume --repo " + shellGuidanceQuote(record.Repo) + " --id " + shellGuidanceQuote(record.ID)
		}
		return true, "supervised IssueOps handoff must be claimed by the dispatched worker before implementation mutation"
	}
	if !nativeSessionMatches(req, h.WorkerSession) {
		return true, "supervised IssueOps handoff mutation is restricted to the claimed native worker session"
	}
	if !currentWorkerBranchMatches(record) {
		return true, "supervised IssueOps worker mutation requires the current Git branch to exactly match the persisted handoff branch; remain read-only and run the exact resume command"
	}
	if searchrouting.IsShellTool(req.Tool) {
		if violation := claimedWorkerRoleViolation(req.Command); violation != "" {
			return true, "supervised IssueOps role=worker may implement, verify, locally commit, heartbeat, and finish only; " + violation + ". Coordinator owns push, PR/MR, accept, recovery, and cleanup"
		}
	}
	if searchrouting.IsShellTool(req.Tool) && unresolvedNestedShellMutation(req.Command) {
		return true, "supervised IssueOps worker shell mutation target cannot be resolved safely inside the claimed worktree"
	}
	workerRoot := cleanAbsPath(h.WorkerRoot)
	if cleanAbsPath(req.CWD) != workerRoot || cleanAbsPath(req.Repo) != workerRoot {
		return true, "supervised IssueOps worker must mutate from the canonical worker worktree root"
	}
	for _, target := range worktreeGuardEditTargets(req) {
		if !pathWithin(target, workerRoot) || !resolvedPathWithin(target, workerRoot) {
			return true, "supervised IssueOps worker mutation target is outside the claimed worker worktree"
		}
	}
	return true, ""
}

func allowedInvalidLegacyV5PublicationSeal(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	native := issueopsmodel.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}
	if !handoff.LegacyCoordinatorIdentityCanBeSealed(record, native, req.CWD) {
		return false
	}
	if searchrouting.IsShellTool(req.Tool) {
		command, ok := parseExactIssueOpsCommand(req.Command)
		return ok && command.path == "handoff publish" && allowedExactHandoffLifecycleCommand(req, record)
	}
	tool, ok := handoffMCPToolKind(req.Tool)
	if !ok || tool != "issueops_handoff" {
		return false
	}
	input, ok := flatMCPInput(req.ToolInput)
	if !ok {
		return false
	}
	action, ok := mcpString(input, "action")
	return ok && action == "publish" && allowedHandoffMCPTool(req, record)
}

func protectedWorkerRootMutation(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	if !searchrouting.IsShellTool(req.Tool) || record.ExecutionHandoff == nil {
		return false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	if len(tokens) == 0 {
		return false
	}
	switch tokens[0] {
	case "rm", "rmdir", "mv", "chmod", "chown":
	case "find":
		foundDelete := false
		for _, token := range tokens[1:] {
			if token == "-delete" {
				foundDelete = true
			}
		}
		if !foundDelete {
			return false
		}
	default:
		return false
	}
	root := cleanAbsPath(record.ExecutionHandoff.WorkerRoot)
	gitPath := filepath.Join(root, ".git")
	targets := make([]string, 0, len(req.Paths))
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(req.Repo, path); target != "" {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		targets = shellCommandWorktreeGuardPaths(req.Repo, req.Command)
	}
	for _, target := range targets {
		clean := cleanAbsPath(target)
		if clean == root || clean == gitPath || pathWithin(clean, gitPath) {
			return true
		}
	}
	return false
}

func coordinatorPlanMutationAllowed(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	h := record.ExecutionHandoff
	if h == nil || h.State != handoff.StateCoordinatorPreparing || strings.TrimSpace(h.ContextSHA256) != "" || h.PendingOperation != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(req.Tool)) {
	case "apply_patch", "edit", "write", "multiedit":
	default:
		return false
	}
	coordinatorRoot := cleanAbsPath(record.Repo)
	if cleanAbsPath(req.CWD) != coordinatorRoot || cleanAbsPath(req.Repo) != coordinatorRoot {
		return false
	}
	workerRoot := cleanAbsPath(h.WorkerRoot)
	targets := worktreeGuardEditTargets(req)
	if workerRoot == "" || len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		if !coordinatorPlanPathAllowed(record, target) {
			return false
		}
	}
	return true
}

func coordinatorPlanPathAllowed(record IssueOpsRecord, target string) bool {
	h := record.ExecutionHandoff
	if h == nil {
		return false
	}
	workerRoot := cleanAbsPath(h.WorkerRoot)
	target = cleanAbsPath(target)
	if workerRoot == "" || !pathWithin(target, workerRoot) || !resolvedPathWithin(target, workerRoot) || strings.ToLower(filepath.Ext(target)) != ".md" {
		return false
	}
	linkedPlan := strings.TrimSpace(record.PlanPath)
	if linkedPlan != "" {
		if !filepath.IsAbs(linkedPlan) {
			linkedPlan = filepath.Join(workerRoot, linkedPlan)
		}
		linkedPlan = cleanAbsPath(linkedPlan)
		if !pathWithin(linkedPlan, workerRoot) || !resolvedPathWithin(linkedPlan, workerRoot) {
			return false
		}
		if target == linkedPlan {
			return true
		}
		if strings.Contains(filepath.Base(linkedPlan), record.ID) || !strings.Contains(filepath.Base(target), record.ID) {
			return false
		}
	}
	allowedRoots := []string{
		filepath.Join(workerRoot, ".agent-harness", "plans"),
		filepath.Join(workerRoot, "docs", "superpowers", "plans"),
		filepath.Join(workerRoot, "plans"),
	}
	for _, root := range allowedRoots {
		if pathWithin(target, root) && resolvedPathWithin(target, root) {
			return true
		}
	}
	return false
}

func coordinatorPlanGitCommandAllowed(req HookToolUseLifecycleRequest, record IssueOpsRecord) bool {
	h := record.ExecutionHandoff
	if h == nil || h.State != handoff.StateCoordinatorPreparing || strings.TrimSpace(h.ContextSHA256) != "" || h.PendingOperation != nil || !searchrouting.IsShellTool(req.Tool) {
		return false
	}
	if cleanAbsPath(req.CWD) != cleanAbsPath(record.Repo) || cleanAbsPath(req.Repo) != cleanAbsPath(record.Repo) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	if len(tokens) < 6 || tokens[0] != "git" || tokens[1] != "-C" || cleanAbsPath(tokens[2]) != cleanAbsPath(h.WorkerRoot) {
		return false
	}
	switch tokens[3] {
	case "add":
		return len(tokens) == 6 && tokens[4] == "--" && coordinatorPlanPathAllowed(record, tokens[5])
	case "commit":
		return len(tokens) == 9 && tokens[4] == "--only" && tokens[5] == "-m" && strings.TrimSpace(tokens[6]) != "" && len(tokens[6]) <= 256 && tokens[7] == "--" && coordinatorPlanPathAllowed(record, tokens[8])
	default:
		return false
	}
}

func resolvedPathWithin(path, root string) bool {
	resolvedPath, err := resolvePathWithMissingLeaf(path)
	if err != nil {
		return false
	}
	resolvedRoot, err := resolvePathWithMissingLeaf(root)
	if err != nil {
		return false
	}
	return pathWithin(resolvedPath, resolvedRoot)
}

func resolvePathWithMissingLeaf(path string) (string, error) {
	path = cleanAbsPath(path)
	missing := []string{}
	current := path
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return cleanAbsPath(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func isHandoffLifecycleCommand(command string) bool {
	if exact, _, ok := parsedExactIssueOps(command); ok && exact.path != "status" && exact.path != "resume" {
		return true
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(command))
	if len(tokens) >= 2 && (tokens[0] == "agent-harness" || tokens[0] == "./bin/agent-harness") && tokens[1] == "issueops" {
		return !(len(tokens) >= 3 && (tokens[2] == "status" || tokens[2] == "resume"))
	}
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
	case "link-plan", "phase":
		return true
	case "compatibility", "execution", "devils-advocate":
		return issueops+2 < len(tokens)
	case "worktree":
		if issueops+2 >= len(tokens) {
			return false
		}
		switch searchrouting.SearchTokenName(tokens[issueops+2]) {
		case "prepare", "prepare-tools":
			return true
		}
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
