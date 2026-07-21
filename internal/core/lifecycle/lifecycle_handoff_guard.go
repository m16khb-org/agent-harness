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
		return ""
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
		return "IssueOps handoff envelope is invalid; remain read-only and require source-session recovery before claim, heartbeat, completion, or implementation mutation."
	}
	if !worker {
		resume := "agent-harness issueops resume --repo " + shellGuidanceQuote(record.Repo) + " --id " + shellGuidanceQuote(record.ID)
		return fmt.Sprintf("IssueOps handoff role=source state=%s attempt=%d context=%s. This cycle does not fence unrelated source-root work. Inspect this cycle explicitly: %s", h.State, h.Attempt, h.ContextSHA256, resume)
	}
	modelBoundary := " Host usage-limit, rate-limit, reset, and model-selection prompts are user-decision boundaries: dismiss or stop and relay; never auto switch models or reset usage."
	resume := "agent-harness issueops resume --repo " + shellGuidanceQuote(record.Repo) + " --id " + shellGuidanceQuote(record.ID)
	claimState := h.State == handoff.StateOwnershipDispatched
	if !claimState {
		if h.State == handoff.StateOwnerOrienting {
			return fmt.Sprintf("IssueOps ownership transfer role=owner state=%s attempt=%d context=%s. Acknowledge the sealed issue and plan context before editing; implementation remains read-only until acknowledgement. Resume: %s", h.State, h.Attempt, h.ContextSHA256, resume) + modelBoundary
		}
		if h.State == handoff.StateOwnerActive {
			return fmt.Sprintf("IssueOps ownership transfer role=owner state=%s attempt=%d context=%s. The acknowledged owner may implement and verify only inside the canonical worker root; publication, terminal steering, and resource cleanup remain human-directed. Resume: %s", h.State, h.Attempt, h.ContextSHA256, resume) + modelBoundary
		}
		return fmt.Sprintf("IssueOps handoff role=owner state=%s attempt=%d context=%s. Resume: %s", h.State, h.Attempt, h.ContextSHA256, resume) + modelBoundary
	}
	if h.Orca == nil || strings.TrimSpace(h.Orca.WorktreeID) == "" {
		return fmt.Sprintf("IssueOps handoff role=owner state=%s attempt=%d context=%s. External identity requires source-session recovery before claim.", h.State, h.Attempt, h.ContextSHA256) + modelBoundary
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
	return fmt.Sprintf("IssueOps handoff role=owner state=%s attempt=%d context=%s. Claim before editing: %s. Read-only resume: %s", h.State, h.Attempt, h.ContextSHA256, claim, resume) + modelBoundary
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

// supervisedFenceRecoverEscape returns the exact working escape command for a
// supervised-fence block, keyed to the record's current handoff sub-state (Task
// F1). CAUTIONS.md requires every stale/fence block message to name a working
// escape; this is the handoff-fence resolver. Each named command is allowed by
// allowedExactHandoffLifecycleCommand from the source checkout for the matching
// state — handoff recover has no session-identity gate there, so the operator
// (any session in the source checkout) can run it.
func supervisedFenceRecoverEscape(record IssueOpsRecord) string {
	h := record.ExecutionHandoff
	if h == nil {
		return ""
	}
	id := shellGuidanceQuote(record.ID)
	switch h.State {
	case handoff.StateRecoveryRequired:
		return "agent-harness issueops handoff recover --id " + id + " --action <reconcile|cancel|abandon> (mutating actions require --confirm; cancellation finishes with --action finalize-cancel --confirm)"
	default:
		return "agent-harness issueops resume --repo " + shellGuidanceQuote(record.Repo) + " --id " + id
	}
}

func handoffOwnershipBlockReason(req HookToolUseLifecycleRequest) (bool, string) {
	if !req.EnforceWorktree {
		return false, ""
	}
	// A Codex coordinator must observe this exact read-only command before an
	// attested supervised start. It carries no lifecycle target or mutation, so
	// multi-cycle source ambiguity cannot safely turn it into a deadlock.
	if searchrouting.IsShellTool(req.Tool) && strings.TrimSpace(req.Command) == "codex --help" {
		return false, ""
	}
	// Host-owned goal, plan, and input control never edits repository or
	// IssueOps state. Classify only the four exact tool identities before record
	// selection so multiple source-sharing handoffs cannot deadlock their own
	// coordinator. Look-alike and namespaced spellings remain fenced.
	if exactHostControlPlaneTool(req.Tool) {
		return true, ""
	}
	// Proven observations do not require choosing an owner. Evaluate the
	// deliberately narrow read-only grammar before supervised-record selection
	// so multiple source-matching cycles cannot deadlock inspection and recovery.
	if ambiguousSupervisedSourceCheckout(req) && (searchrouting.IsShellTool(req.Tool) && commandparse.ExactReadOnlyShellCommand(req.Command) || !searchrouting.IsShellTool(req.Tool) && explicitHandoffReadOnlyTool(req.Tool)) {
		return true, ""
	}
	record, ok, selectionReason := selectSupervisedHandoffRecord(req)
	if selectionReason != "" {
		return true, selectionReason
	}
	if !ok {
		return false, ""
	}
	if record.Invalid {
		return true, "invalid supervised IssueOps durable record: " + record.InvalidReason
	}
	if searchrouting.IsShellTool(req.Tool) && (commandparse.HasUnquotedControlOperator(req.Command) || commandparse.HasActiveCommandSubstitution(req.Command) || commandparse.HasActiveOutputRedirect(req.Command) || commandparse.HasActiveParameterOrTildeExpansion(req.Command) || commandparse.HasActivePathnameExpansion(req.Command) || commandparse.HasActiveShellSpecialQuoting(req.Command) || commandparse.HasActiveZshEqualsExpansion(req.Command)) {
		return true, "active shell control, command/process substitution, parameter/tilde expansion, pathname expansion, and output redirection are forbidden during a supervised IssueOps handoff; pass freeform text as argv-safe data or POSIX single-quoted literal data with an explicit canonical path"
	}
	if err := handoff.ValidateEnvelope(record); err != nil {
		return true, "invalid supervised IssueOps handoff envelope: " + err.Error()
	}
	if workspacePreparationStateKnown(record) {
		if searchrouting.IsShellTool(req.Tool) && exactReadOnlyShellCommand(req, record) || issueOpsObservationMCPTool(req.Tool) && allowedIssueOpsObservationMCP(req, record) {
			return true, ""
		}
		if command := bootstrapOwnershipStartGuidance(req, record); command != "" {
			return true, "supervised IssueOps bootstrap requires the authenticated native coordinator identity; rerun this exact harness-authored preview command: " + command
		}
		if isHandoffLifecycleCommand(req.Command) && allowedExactHandoffLifecycleCommand(req, record) {
			return true, ""
		}
		if allowedReadyWorkspacePreparationMCP(req, record) {
			return true, ""
		}
		if allowedSourceWorkspacePlanMutation(req, record) {
			return true, ""
		}
		if reason := workspacePreparationBlockReason(req, record); reason != "" {
			return true, reason
		}
		return true, ""
	}
	if issueOpsObservationMCPTool(req.Tool) {
		if allowedIssueOpsObservationMCP(req, record) {
			return true, ""
		}
		if kind, ok := issueOpsObservationMCPKind(req.Tool); ok && kind == "issueops_resume" && req.ToolInput != nil {
			if bind, exists := req.ToolInput["bind"]; exists && bind == true {
				return true, "IssueOps resume observation must omit bind or set bind=false for a supervised handoff"
			}
		}
		return true, "exact IssueOps status/resume MCP payload does not match the selected supervised cycle"
	}
	if terminalControlWriteRequest(req) {
		if allowedClosedOrcaCleanup(req, record) {
			return true, ""
		}
		return true, "raw terminal steering is blocked outside the explicit source cleanup flow"
	}
	if isHandoffMCPTool(req.Tool) {
		if allowedHandoffMCPTool(req, record) {
			return true, ""
		}
		return true, "supervised IssueOps MCP lifecycle payload does not match the native session, actor, and persisted fence"
	}
	if isPostTransferRecorderMCP(req.Tool) {
		if allowedPostTransferRecorderMCP(req, record) {
			return true, ""
		}
		return true, "ownership-transfer recorder MCP payload does not match the active native owner and canonical worker root"
	}
	if isHandoffLifecycleCommand(req.Command) {
		if allowedExactHandoffLifecycleCommand(req, record) {
			return true, ""
		}
		return true, "supervised IssueOps handoff lifecycle command is not in the supervised-fence allowlist for this session and state; the working escape from the source checkout " + shellGuidanceQuote(record.Repo) + " is " + supervisedFenceRecoverEscape(record)
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
	return ownershipTransferMutationBlockReason(req, record)
}

func ownershipTransferMutationBlockReason(req HookToolUseLifecycleRequest, record IssueOpsRecord) (bool, string) {
	h := record.ExecutionHandoff
	if h == nil {
		return true, "ownership transfer handoff record is incomplete"
	}
	if !handoff.OwnerStateAllows("mutate", h.State) {
		if h.State == handoff.StateOwnershipDispatched && cleanAbsPath(req.CWD) == cleanAbsPath(h.WorkerRoot) {
			if claim := buildExactClaimCommand(record, req); claim != "" {
				return true, "ownership transfer handoff must be claimed before mutation; run only: " + claim
			}
		}
		if h.State == handoff.StateOwnerOrienting && nativeSessionMatches(req, h.OwnerSession) {
			return true, "ownership transfer owner must acknowledge the sealed issue and plan context before mutation"
		}
		return true, "ownership transfer handoff does not grant worker-root mutation authority in state " + h.State
	}
	if !nativeSessionMatches(req, h.OwnerSession) {
		return true, "ownership transfer worker-root mutation is restricted to the acknowledged native owner session"
	}
	if !currentWorkerBranchMatches(record) {
		return true, "ownership transfer owner mutation requires the current Git branch to exactly match the persisted handoff branch; remain read-only and run the exact resume command"
	}
	if searchrouting.IsShellTool(req.Tool) {
		if violation := claimedWorkerRoleViolation(req.Command); violation != "" {
			return true, "ownership transfer owner may implement, verify, and locally commit only; " + violation + ". Source coordinator and owner cannot push, publish, steer terminals, or clean up resources automatically"
		}
	}
	if searchrouting.IsShellTool(req.Tool) && unresolvedNestedShellMutation(req.Command) {
		return true, "ownership transfer owner shell mutation target cannot be resolved safely inside the canonical worker worktree"
	}
	workerRoot := cleanAbsPath(h.WorkerRoot)
	if cleanAbsPath(req.CWD) != workerRoot || cleanAbsPath(req.Repo) != workerRoot {
		return true, "ownership transfer owner must mutate from the canonical worker worktree root"
	}
	for _, target := range worktreeGuardEditTargets(req) {
		if !pathWithin(target, workerRoot) || !resolvedPathWithin(target, workerRoot) {
			return true, "ownership transfer owner mutation target is outside the canonical worker worktree"
		}
	}
	return true, ""
}

func ambiguousSupervisedSourceCheckout(req HookToolUseLifecycleRequest) bool {
	cwd := cleanAbsPath(req.CWD)
	if cwd == "" {
		return false
	}
	byID := map[string]bool{}
	for _, repo := range []string{req.Repo, req.SourceCheckout, sourceCheckoutFromWorktree(req.Repo), sourceCheckoutFromWorktree(req.CWD)} {
		if strings.TrimSpace(repo) == "" {
			continue
		}
		for _, record := range supervisedHandoffGuardRecords(repo) {
			if record.ExecutionHandoff != nil && cwd == cleanAbsPath(record.Repo) {
				byID[record.ID] = true
			}
		}
	}
	return len(byID) > 1
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
	if exact, _, ok := parsedExactIssueOps(command); ok && exact.Path != "status" && exact.Path != "resume" {
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
		case "start", "recover", "claim", "acknowledge-context", "publish", "complete", "cleanup-preview", "cleanup-approve", "cleanup-record":
			return true
		}
	}
	return false
}
