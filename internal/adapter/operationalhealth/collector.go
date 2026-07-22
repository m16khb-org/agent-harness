package operationalhealth

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/pathutil"
	corehealth "agent-harness/internal/core/operationalhealth"
	"agent-harness/internal/port"
)

type OrcaInventory interface {
	Available() bool
	Status(context.Context) (port.OrcaStatus, error)
	ResolveRepo(context.Context, string) (port.OrcaRepo, error)
	ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
	ListTerminals(context.Context, string) ([]port.OrcaTerminal, error)
	ListAllTasks(context.Context) ([]port.OrcaTask, error)
	ListDispatchedTasks(context.Context) ([]port.OrcaTask, error)
	ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
	ListGates(context.Context) ([]port.OrcaGate, error)
	InboxPresence(context.Context) (port.OrcaInboxPresence, error)
}

type GitRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

const gitInventoryCommandTimeout = 15 * time.Second

type ExecGitRunner struct {
	timeout time.Duration
}

func (runner ExecGitRunner) Run(ctx context.Context, repo string, args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("git arguments are required")
	}
	timeout := runner.timeout
	if timeout <= 0 {
		timeout = gitInventoryCommandTimeout
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(commandCtx, "git", args...)
	command.Dir = repo
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never", "SSH_ASKPASS_REQUIRE=never")
	output, err := command.Output()
	if commandCtx.Err() != nil {
		return nil, fmt.Errorf("git inventory command: %w", commandCtx.Err())
	}
	return output, err
}

func canonicalInventoryPath(path string) string {
	abs := pathutil.CleanAbsPath(path)
	if abs == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}
	original := abs
	missing := make([]string, 0, 2)
	for {
		parent := filepath.Dir(abs)
		if parent == abs {
			return original
		}
		missing = append([]string{filepath.Base(abs)}, missing...)
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			parts := append([]string{filepath.Clean(resolved)}, missing...)
			return filepath.Join(parts...)
		}
		abs = parent
	}
}

type Collector struct {
	Git  GitRunner
	Orca OrcaInventory
}

func (collector Collector) Collect(ctx context.Context, repo string) corehealth.Snapshot {
	repo = canonicalInventoryPath(repo)
	snapshot := corehealth.Snapshot{
		RepoRoot: repo,
		Messages: corehealth.MessagePresence{Empty: true},
	}
	if repo == "" {
		addProblem(&snapshot, "repo", "repo_invalid", "requested repository path is empty")
		return snapshot
	}
	collector.collectGit(ctx, &snapshot)
	_, orcaOwned := collectIssueOps(&snapshot)
	collectBindings(&snapshot)
	collector.collectOrca(ctx, &snapshot, orcaOwned)
	sortSnapshot(&snapshot)
	return snapshot
}

func (collector Collector) collectGit(ctx context.Context, snapshot *corehealth.Snapshot) {
	if collector.Git == nil {
		addProblem(snapshot, "git", "git_runner_missing", "Git inventory reader is unavailable")
		return
	}
	commands := []struct {
		source string
		code   string
		args   []string
		apply  func([]byte) error
	}{
		{
			source: "git_branch", code: "git_symbolic_ref_failed",
			args: []string{"symbolic-ref", "--quiet", "--short", "HEAD"},
			apply: func(output []byte) error {
				snapshot.CanonicalBranch = strings.TrimSpace(string(output))
				if snapshot.CanonicalBranch == "" {
					return fmt.Errorf("empty branch")
				}
				return nil
			},
		},
		{
			source: "git_head", code: "git_head_failed",
			args: []string{"rev-parse", "--verify", "HEAD"},
			apply: func(output []byte) error {
				snapshot.SourceHead = strings.TrimSpace(string(output))
				if !validOID(snapshot.SourceHead) {
					return fmt.Errorf("invalid head")
				}
				return nil
			},
		},
		{
			source: "git_status", code: "git_status_failed",
			args: []string{"status", "--porcelain=v1", "-z"},
			apply: func(output []byte) error {
				snapshot.SourceClean = len(output) == 0
				return nil
			},
		},
		{
			source: "git_worktrees", code: "git_worktrees_failed",
			args: []string{"worktree", "list", "--porcelain", "-z"},
			apply: func(output []byte) error {
				values, err := parseWorktrees(output, snapshot.RepoRoot)
				snapshot.GitWorktrees = values
				return err
			},
		},
		{
			source: "git_local_refs", code: "git_local_refs_failed",
			args: []string{"for-each-ref", "--format=%(refname)%00%(objectname)%00", "refs/heads"},
			apply: func(output []byte) error {
				values, err := parseLocalRefs(output)
				snapshot.LocalRefs = values
				return err
			},
		},
		{
			source: "git_remote_refs", code: "git_remote_refs_failed",
			args: []string{"ls-remote", "--heads", "origin"},
			apply: func(output []byte) error {
				values, err := parseRemoteRefs(output)
				snapshot.RemoteRefs = values
				return err
			},
		},
	}
	for _, command := range commands {
		output, err := collector.Git.Run(ctx, snapshot.RepoRoot, command.args...)
		if err != nil {
			addProblem(snapshot, command.source, command.code, command.source+" inventory failed")
			continue
		}
		if err := command.apply(output); err != nil {
			addProblem(snapshot, command.source, command.code, command.source+" inventory is malformed")
		}
	}
	for index := range snapshot.GitWorktrees {
		if snapshot.GitWorktrees[index].Canonical {
			snapshot.GitWorktrees[index].Clean = snapshot.SourceClean
			if snapshot.SourceHead != "" && snapshot.GitWorktrees[index].Head != snapshot.SourceHead {
				addProblem(snapshot, "git_worktrees", "git_source_identity_mismatch", "canonical Git worktree head does not match source HEAD")
			}
		}
	}
}

func collectIssueOps(snapshot *corehealth.Snapshot) ([]issueops.IssueOpsRecord, bool) {
	stateRoot := issueops.IssueOpsStateRoot()
	ids, err := issueops.ListIssueOpsIDs(stateRoot)
	if err != nil {
		addProblem(snapshot, "issueops", "issueops_list_failed", "IssueOps ID inventory failed")
		return nil, false
	}
	records := make([]issueops.IssueOpsRecord, 0, len(ids))
	orcaOwned := false
	for _, id := range ids {
		record, err := issueops.ReadIssueOpsExisting(stateRoot, id)
		if err != nil {
			addProblem(snapshot, "issueops_record", "issueops_read_failed", "could not read IssueOps record "+strings.TrimSpace(id))
			continue
		}
		records = append(records, record)
		cycle, problems := cycleFromRecord(record)
		snapshot.Cycles = append(snapshot.Cycles, cycle)
		snapshot.InventoryProblems = append(snapshot.InventoryProblems, problems...)
		orcaOwned = orcaOwned || recordOwnsOrca(record)
	}
	return records, orcaOwned
}

func collectBindings(snapshot *corehealth.Snapshot) {
	bindings, err := issueops.ListAllIssueOpsSessionBindingsExisting()
	if err != nil {
		addProblem(snapshot, "issueops_binding", "binding_list_failed", "could not read complete binding inventory")
		return
	}
	seen := make(map[string]struct{})
	for _, binding := range bindings {
		value := corehealth.Binding{
			CycleID: strings.TrimSpace(binding.CycleID), Repo: canonicalInventoryPath(binding.Repo),
			Branch: strings.TrimSpace(binding.Branch), ExpectedWorktree: canonicalInventoryPath(binding.ExpectedWorktree),
		}
		key := strings.Join([]string{value.CycleID, value.Repo, value.Branch, value.ExpectedWorktree}, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		snapshot.Bindings = append(snapshot.Bindings, value)
	}
}

func (collector Collector) collectOrca(ctx context.Context, snapshot *corehealth.Snapshot, owned bool) {
	if collector.Orca == nil || !collector.Orca.Available() {
		if owned {
			addProblem(snapshot, "orca", "orca_unavailable", "Orca-owned IssueOps records exist but Orca is unavailable")
			return
		}
		snapshot.Messages.CompleteAbsence = true
		return
	}
	snapshot.OrcaObserved = true
	status, err := collector.Orca.Status(ctx)
	if err != nil {
		addProblem(snapshot, "orca_status", "orca_status_failed", "Orca status inventory failed")
		return
	}
	snapshot.OrcaRuntimeID = strings.TrimSpace(status.RuntimeID)
	if snapshot.OrcaRuntimeID == "" || !status.RuntimeReachable || strings.TrimSpace(status.RuntimeState) != "ready" || strings.TrimSpace(status.GraphState) != "ready" {
		addProblem(snapshot, "orca_status", "orca_runtime_unready", "Orca runtime and graph must be reachable and ready")
		return
	}

	resolved, resolveErr := collector.Orca.ResolveRepo(ctx, snapshot.RepoRoot)
	if resolveErr != nil {
		addProblem(snapshot, "orca_repo", "orca_repo_failed", "Orca repo resolution failed")
	} else if strings.TrimSpace(resolved.RuntimeID) != snapshot.OrcaRuntimeID || strings.TrimSpace(resolved.ID) == "" || canonicalInventoryPath(resolved.Path) != snapshot.RepoRoot {
		addProblem(snapshot, "orca_repo", "orca_repo_identity_mismatch", "Orca repo identity does not match the requested repository")
	}
	snapshot.OrcaRepoID = strings.TrimSpace(resolved.ID)
	repoPath := snapshot.RepoRoot
	if resolveErr == nil && canonicalInventoryPath(resolved.Path) != "" {
		repoPath = canonicalInventoryPath(resolved.Path)
	}

	worktrees, err := collector.Orca.ListWorktrees(ctx, snapshot.RepoRoot)
	if err != nil {
		addProblem(snapshot, "orca_worktrees", "orca_worktrees_failed", "Orca worktree inventory failed")
	} else {
		for _, value := range worktrees {
			if strings.TrimSpace(value.RuntimeID) != snapshot.OrcaRuntimeID || strings.TrimSpace(value.ID) == "" || strings.TrimSpace(value.InstanceID) == "" || canonicalInventoryPath(value.Path) == "" || (resolved.ID != "" && strings.TrimSpace(value.RepoID) != strings.TrimSpace(resolved.ID)) {
				addProblem(snapshot, "orca_worktrees", "orca_worktree_identity_invalid", "Orca worktree identity is incomplete or mismatched")
			}
			snapshot.OrcaWorktrees = append(snapshot.OrcaWorktrees, corehealth.OrcaWorktree{
				RuntimeID: value.RuntimeID, RepoID: value.RepoID, ID: value.ID, InstanceID: value.InstanceID, Repo: repoPath,
				Path: canonicalInventoryPath(value.Path), Branch: strings.TrimSpace(value.Branch), Head: strings.TrimSpace(value.Head),
			})
		}
	}

	terminals, err := collector.Orca.ListTerminals(ctx, "")
	if err != nil {
		addProblem(snapshot, "orca_terminals", "orca_terminals_failed", "Orca terminal inventory failed")
	} else {
		for _, value := range terminals {
			if strings.TrimSpace(value.RuntimeID) != snapshot.OrcaRuntimeID || strings.TrimSpace(value.Handle) == "" || strings.TrimSpace(value.PTYID) == "" || strings.TrimSpace(value.WorktreeID) == "" || strings.TrimSpace(value.TabID) == "" || strings.TrimSpace(value.LeafID) == "" {
				addProblem(snapshot, "orca_terminals", "orca_terminal_identity_invalid", "Orca terminal identity is incomplete")
			}
			snapshot.Terminals = append(snapshot.Terminals, corehealth.OrcaTerminal{
				RuntimeID: value.RuntimeID, Handle: value.Handle, PTYID: value.PTYID, TabID: value.TabID, LeafID: value.LeafID, WorktreeID: value.WorktreeID,
				WorktreePath: canonicalInventoryPath(value.WorktreePath), Connected: value.Connected, Writable: value.Writable,
			})
		}
	}

	tasks, err := collector.Orca.ListAllTasks(ctx)
	if err != nil {
		addProblem(snapshot, "orca_tasks", "orca_tasks_failed", "Orca task inventory failed")
	} else {
		for _, value := range tasks {
			task := corehealth.OrcaTask{RuntimeID: strings.TrimSpace(value.RuntimeID), ID: strings.TrimSpace(value.ID), Status: strings.TrimSpace(value.Status), HasResult: value.HasResult}
			if task.RuntimeID != snapshot.OrcaRuntimeID {
				addProblem(snapshot, "orca_tasks", "orca_task_runtime_mismatch", "task "+task.ID+" runtime identity does not match")
			}
			if strings.TrimSpace(value.CompletedAt) != "" {
				parsed, parseErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(value.CompletedAt))
				if parseErr != nil {
					addProblem(snapshot, "orca_tasks", "orca_task_timestamp_invalid", "task "+task.ID+" has an invalid completion timestamp")
				} else {
					task.CompletedAt = parsed
				}
			}
			snapshot.Tasks = append(snapshot.Tasks, task)
		}
	}

	dispatched, err := collector.Orca.ListDispatchedTasks(ctx)
	if err != nil {
		addProblem(snapshot, "orca_dispatches", "orca_dispatched_tasks_failed", "Orca dispatched-task inventory failed")
	} else {
		dispatchedCounts := make(map[string]int, len(dispatched))
		for _, task := range dispatched {
			taskID := strings.TrimSpace(task.ID)
			if strings.TrimSpace(task.RuntimeID) != snapshot.OrcaRuntimeID {
				addProblem(snapshot, "orca_dispatches", "orca_dispatched_task_runtime_mismatch", "dispatched task "+taskID+" runtime identity does not match")
			}
			dispatchedCounts[taskID]++
		}
		for _, task := range snapshot.Tasks {
			if strings.TrimSpace(task.Status) == "dispatched" && dispatchedCounts[strings.TrimSpace(task.ID)] != 1 {
				addProblem(snapshot, "orca_dispatches", "orca_dispatch_task_mismatch", "dispatched task sets do not match")
			}
		}
		for _, task := range dispatched {
			taskID := strings.TrimSpace(task.ID)
			if taskID == "" || strings.TrimSpace(task.Status) != "dispatched" || dispatchedCounts[taskID] != 1 || countTasksWithStatus(snapshot.Tasks, taskID, "dispatched") != 1 {
				addProblem(snapshot, "orca_dispatches", "orca_dispatch_task_mismatch", "dispatched task identity is missing or ambiguous")
			}
			if taskID == "" {
				continue
			}
			dispatch, showErr := collector.Orca.ShowDispatch(ctx, taskID)
			if showErr != nil {
				addProblem(snapshot, "orca_dispatches", "orca_dispatch_failed", "could not resolve dispatch for task "+taskID)
				continue
			}
			value := corehealth.OrcaDispatch{RuntimeID: strings.TrimSpace(dispatch.RuntimeID), ID: strings.TrimSpace(dispatch.ID), TaskID: strings.TrimSpace(dispatch.TaskID), AssigneeHandle: strings.TrimSpace(dispatch.AssigneeHandle), Status: strings.TrimSpace(dispatch.Status)}
			if value.RuntimeID != snapshot.OrcaRuntimeID || value.ID == "" || value.TaskID != taskID || value.AssigneeHandle == "" || value.Status != "dispatched" {
				addProblem(snapshot, "orca_dispatches", "orca_dispatch_identity_mismatch", "dispatch identity does not match task "+taskID)
			}
			snapshot.Dispatches = append(snapshot.Dispatches, value)
			if value.ID != "" && value.TaskID == taskID {
				for index := range snapshot.Tasks {
					if snapshot.Tasks[index].ID == taskID {
						snapshot.Tasks[index].DispatchID = value.ID
					}
				}
			}
		}
	}

	gates, err := collector.Orca.ListGates(ctx)
	if err != nil {
		addProblem(snapshot, "orca_gates", "orca_gates_failed", "Orca gate inventory failed")
	} else {
		for _, value := range gates {
			gate := corehealth.OrcaGate{RuntimeID: strings.TrimSpace(value.RuntimeID), ID: strings.TrimSpace(value.ID), TaskID: strings.TrimSpace(value.TaskID), Status: strings.TrimSpace(value.Status)}
			if gate.RuntimeID != snapshot.OrcaRuntimeID {
				addProblem(snapshot, "orca_gates", "orca_gate_runtime_mismatch", "gate "+gate.ID+" runtime identity does not match")
			}
			snapshot.Gates = append(snapshot.Gates, gate)
		}
	}
	inbox, err := collector.Orca.InboxPresence(ctx)
	if err != nil {
		addProblem(snapshot, "orca_inbox", "orca_inbox_failed", "Orca inbox presence inventory failed")
	} else {
		snapshot.Messages = corehealth.MessagePresence{RuntimeID: strings.TrimSpace(inbox.RuntimeID), Count: inbox.Count, Empty: inbox.RowCount == 0, CompleteAbsence: inbox.CompleteAbsence}
		if snapshot.Messages.RuntimeID != snapshot.OrcaRuntimeID || inbox.Count != inbox.RowCount || inbox.RowCount < 0 || inbox.RowCount > 1 {
			addProblem(snapshot, "orca_inbox", "orca_inbox_count_mismatch", "bounded Orca inbox count does not match returned rows")
		}
	}
}

func cycleFromRecord(record issueops.IssueOpsRecord) (corehealth.Cycle, []corehealth.InventoryProblem) {
	cycle := corehealth.Cycle{
		ID: strings.TrimSpace(record.ID), Repo: canonicalInventoryPath(record.Repo), Branch: strings.TrimSpace(record.Branch),
		Phase: string(record.Phase),
	}
	var problems []corehealth.InventoryProblem
	worktreeConflict := false
	mergeWorktreePath := func(raw string) {
		path := canonicalInventoryPath(raw)
		if path == "" {
			return
		}
		if cycle.WorktreePath == "" {
			cycle.WorktreePath = path
			return
		}
		if cycle.WorktreePath != path && !worktreeConflict {
			problems = append(problems, corehealth.InventoryProblem{Source: "issueops_record", Code: "issueops_worktree_identity_mismatch", Detail: "IssueOps record " + cycle.ID + " contains conflicting worktree paths"})
			worktreeConflict = true
		}
	}
	mergeWorktreePath(record.WorktreePath)
	attempt := issueops.CurrentOwnershipAttempt(record)
	if attempt == nil || attempt.Handoff == nil {
		cycle.LastHeartbeatAt, problems = parsePersistedTime(record.LastHeartbeatAt, record.ID, "record", problems)
		return cycle, problems
	}
	handoff := attempt.Handoff
	cycle.HandoffState = strings.TrimSpace(handoff.State)
	if attempt.Workspace != nil {
		cycle.WorkspaceState = strings.TrimSpace(attempt.Workspace.State)
	}
	cycle.Attempt = handoff.Attempt
	cycle.OwnershipEpoch = strings.TrimSpace(handoff.OwnershipEpoch)
	cycle.ContextSHA256 = strings.TrimSpace(handoff.ContextSHA256)
	if handoff.OwnerSession != nil {
		cycle.WorkerSessionID = strings.TrimSpace(handoff.OwnerSession.SessionID)
		cycle.WorkerAgentID = strings.TrimSpace(handoff.OwnerSession.AgentID)
	}
	if handoff.Orca != nil {
		cycle.OrcaRuntimeID = strings.TrimSpace(handoff.Orca.RuntimeID)
		cycle.OrcaRepoID = strings.TrimSpace(handoff.Orca.RepoID)
		cycle.OrcaWorktreeID = strings.TrimSpace(handoff.Orca.WorktreeID)
		cycle.OrcaWorktreeInstanceID = strings.TrimSpace(handoff.Orca.WorktreeInstanceID)
		cycle.TerminalHandle = strings.TrimSpace(handoff.Orca.WorkerTerminalHandle)
		cycle.PTYID = strings.TrimSpace(handoff.Orca.WorkerPTYID)
		cycle.TerminalTabID = strings.TrimSpace(handoff.Orca.WorkerTabID)
		cycle.TerminalLeafID = strings.TrimSpace(handoff.Orca.WorkerLeafID)
		cycle.TaskID = strings.TrimSpace(handoff.Orca.TaskID)
		cycle.DispatchID = strings.TrimSpace(handoff.Orca.DispatchID)
		mergeWorktreePath(handoff.Orca.WorktreePath)
	}
	mergeWorktreePath(handoff.WorkerRoot)
	heartbeat := strings.TrimSpace(handoff.LastHeartbeatAt)
	if heartbeat == "" {
		heartbeat = strings.TrimSpace(record.LastHeartbeatAt)
	}
	cycle.LastHeartbeatAt, problems = parsePersistedTime(heartbeat, record.ID, "heartbeat", problems)
	return cycle, problems
}

func parsePersistedTime(value, id, field string, problems []corehealth.InventoryProblem) (time.Time, []corehealth.InventoryProblem) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, problems
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		problems = append(problems, corehealth.InventoryProblem{Source: "issueops_record", Code: "issueops_timestamp_invalid", Detail: "IssueOps record " + strings.TrimSpace(id) + " has an invalid " + field + " timestamp"})
		return time.Time{}, problems
	}
	return parsed, problems
}

func recordOwnsOrca(record issueops.IssueOpsRecord) bool {
	attempt := issueops.CurrentOwnershipAttempt(record)
	return attempt != nil && (attempt.Handoff != nil && attempt.Handoff.Orca != nil ||
		attempt.Workspace != nil && attempt.Workspace.Orca != nil)
}

func parseWorktrees(output []byte, repo string) ([]corehealth.GitWorktree, error) {
	var result []corehealth.GitWorktree
	current := corehealth.GitWorktree{}
	flush := func() error {
		if current.Path == "" {
			if current.Head == "" && current.Branch == "" {
				return nil
			}
			return fmt.Errorf("worktree path missing")
		}
		if current.Head == "" || !validOID(current.Head) {
			return fmt.Errorf("worktree head invalid")
		}
		current.Canonical = current.Path == repo
		result = append(result, current)
		current = corehealth.GitWorktree{}
		return nil
	}
	for _, raw := range bytes.Split(output, []byte{0}) {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			if err := flush(); err != nil {
				return result, err
			}
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			current.Path = canonicalInventoryPath(value)
		case "HEAD":
			current.Head = strings.TrimSpace(value)
		case "branch":
			current.Branch = strings.TrimPrefix(strings.TrimSpace(value), "refs/heads/")
		}
	}
	if err := flush(); err != nil {
		return result, err
	}
	return result, nil
}

func parseLocalRefs(output []byte) ([]corehealth.GitRef, error) {
	var fields []string
	for _, raw := range bytes.Split(output, []byte{0}) {
		if value := strings.TrimSpace(string(raw)); value != "" {
			fields = append(fields, value)
		}
	}
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("local ref tuple incomplete")
	}
	result := make([]corehealth.GitRef, 0, len(fields)/2)
	for index := 0; index < len(fields); index += 2 {
		name, oid := fields[index], fields[index+1]
		if !strings.HasPrefix(name, "refs/heads/") || !validOID(oid) {
			return result, fmt.Errorf("local ref identity invalid")
		}
		result = append(result, corehealth.GitRef{Name: name, Branch: strings.TrimPrefix(name, "refs/heads/"), OID: oid, Location: "local"})
	}
	return result, nil
}

func parseRemoteRefs(output []byte) ([]corehealth.GitRef, error) {
	var result []corehealth.GitRef
	for _, raw := range bytes.Split(output, []byte{'\n'}) {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !validOID(fields[0]) || !strings.HasPrefix(fields[1], "refs/heads/") {
			return result, fmt.Errorf("remote ref identity invalid")
		}
		result = append(result, corehealth.GitRef{Name: fields[1], Branch: strings.TrimPrefix(fields[1], "refs/heads/"), OID: fields[0], Location: "remote"})
	}
	return result, nil
}

func validOID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func countTasksWithStatus(values []corehealth.OrcaTask, id, status string) int {
	count := 0
	for _, value := range values {
		if value.ID == id && value.Status == status {
			count++
		}
	}
	return count
}

func addProblem(snapshot *corehealth.Snapshot, source, code, detail string) {
	snapshot.InventoryProblems = append(snapshot.InventoryProblems, corehealth.InventoryProblem{Source: source, Code: code, Detail: detail})
}

func sortSnapshot(snapshot *corehealth.Snapshot) {
	sort.Slice(snapshot.InventoryProblems, func(i, j int) bool {
		left, right := snapshot.InventoryProblems[i], snapshot.InventoryProblems[j]
		return orderedBefore([]string{left.Source, left.Code, left.Detail}, []string{right.Source, right.Code, right.Detail})
	})
	sort.Slice(snapshot.Cycles, func(i, j int) bool {
		left, right := snapshot.Cycles[i], snapshot.Cycles[j]
		return orderedBefore(
			[]string{left.ID, left.Repo, left.Branch, left.Phase, left.HandoffState, strconv.Itoa(left.Attempt), left.OwnershipEpoch, left.ContextSHA256, left.WorkerSessionID, left.WorkerAgentID, left.OrcaRuntimeID, left.OrcaRepoID, left.WorktreePath, left.OrcaWorktreeID, left.OrcaWorktreeInstanceID, left.TerminalHandle, left.PTYID, left.TerminalTabID, left.TerminalLeafID, left.TaskID, left.DispatchID, timeSortValue(left.LastHeartbeatAt)},
			[]string{right.ID, right.Repo, right.Branch, right.Phase, right.HandoffState, strconv.Itoa(right.Attempt), right.OwnershipEpoch, right.ContextSHA256, right.WorkerSessionID, right.WorkerAgentID, right.OrcaRuntimeID, right.OrcaRepoID, right.WorktreePath, right.OrcaWorktreeID, right.OrcaWorktreeInstanceID, right.TerminalHandle, right.PTYID, right.TerminalTabID, right.TerminalLeafID, right.TaskID, right.DispatchID, timeSortValue(right.LastHeartbeatAt)},
		)
	})
	sort.Slice(snapshot.Bindings, func(i, j int) bool {
		left, right := snapshot.Bindings[i], snapshot.Bindings[j]
		return orderedBefore([]string{left.Repo, left.CycleID, left.Branch, left.ExpectedWorktree}, []string{right.Repo, right.CycleID, right.Branch, right.ExpectedWorktree})
	})
	sort.Slice(snapshot.GitWorktrees, func(i, j int) bool {
		left, right := snapshot.GitWorktrees[i], snapshot.GitWorktrees[j]
		return orderedBefore(
			[]string{left.Path, left.Branch, left.Head, strconv.FormatBool(left.Clean), strconv.FormatBool(left.Canonical)},
			[]string{right.Path, right.Branch, right.Head, strconv.FormatBool(right.Clean), strconv.FormatBool(right.Canonical)},
		)
	})
	sortRefs(snapshot.LocalRefs)
	sortRefs(snapshot.RemoteRefs)
	sort.Slice(snapshot.OrcaWorktrees, func(i, j int) bool {
		left, right := snapshot.OrcaWorktrees[i], snapshot.OrcaWorktrees[j]
		return orderedBefore(
			[]string{left.RuntimeID, left.RepoID, left.ID, left.InstanceID, left.Repo, left.Path, left.Branch, left.Head},
			[]string{right.RuntimeID, right.RepoID, right.ID, right.InstanceID, right.Repo, right.Path, right.Branch, right.Head},
		)
	})
	sort.Slice(snapshot.Terminals, func(i, j int) bool {
		left, right := snapshot.Terminals[i], snapshot.Terminals[j]
		return orderedBefore(
			[]string{left.RuntimeID, left.Handle, left.PTYID, left.TabID, left.LeafID, left.WorktreeID, left.WorktreePath, strconv.FormatBool(left.Connected), strconv.FormatBool(left.Writable)},
			[]string{right.RuntimeID, right.Handle, right.PTYID, right.TabID, right.LeafID, right.WorktreeID, right.WorktreePath, strconv.FormatBool(right.Connected), strconv.FormatBool(right.Writable)},
		)
	})
	sort.Slice(snapshot.Tasks, func(i, j int) bool {
		left, right := snapshot.Tasks[i], snapshot.Tasks[j]
		return orderedBefore(
			[]string{left.RuntimeID, left.ID, left.Status, left.DispatchID, timeSortValue(left.CompletedAt), strconv.FormatBool(left.HasResult)},
			[]string{right.RuntimeID, right.ID, right.Status, right.DispatchID, timeSortValue(right.CompletedAt), strconv.FormatBool(right.HasResult)},
		)
	})
	sort.Slice(snapshot.Dispatches, func(i, j int) bool {
		left, right := snapshot.Dispatches[i], snapshot.Dispatches[j]
		return orderedBefore(
			[]string{left.RuntimeID, left.ID, left.TaskID, left.AssigneeHandle, left.Status},
			[]string{right.RuntimeID, right.ID, right.TaskID, right.AssigneeHandle, right.Status},
		)
	})
	sort.Slice(snapshot.Gates, func(i, j int) bool {
		left, right := snapshot.Gates[i], snapshot.Gates[j]
		return orderedBefore([]string{left.RuntimeID, left.ID, left.TaskID, left.Status}, []string{right.RuntimeID, right.ID, right.TaskID, right.Status})
	})
	sort.Slice(snapshot.StateArtifacts, func(i, j int) bool {
		left, right := snapshot.StateArtifacts[i], snapshot.StateArtifacts[j]
		return orderedBefore([]string{left.Path, left.Code}, []string{right.Path, right.Code})
	})
}

func sortRefs(values []corehealth.GitRef) {
	sort.Slice(values, func(i, j int) bool {
		left, right := values[i], values[j]
		return orderedBefore([]string{left.Name, left.Branch, left.OID, left.Location}, []string{right.Name, right.Branch, right.OID, right.Location})
	})
}

func orderedBefore(left, right []string) bool {
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index] != right[index] {
			return left[index] < right[index]
		}
	}
	return len(left) < len(right)
}

func timeSortValue(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
