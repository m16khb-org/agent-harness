package operationalhealth

import (
	"fmt"
	"sort"
	"strings"
)

func EvaluateCycleAuthority(cycle Cycle, opts Options) CycleAuthority {
	phase := strings.TrimSpace(cycle.Phase)
	status := strings.TrimSpace(cycle.LeaseStatus)
	if !knownPhase(phase) || strings.TrimSpace(cycle.ID) == "" || strings.TrimSpace(cycle.Repo) == "" {
		return AuthorityUnknown
	}
	if !knownLeaseStatus(status) {
		return AuthorityUnknown
	}
	if status == "" {
		if phase == "done" {
			return AuthorityUnknown
		}
		if hasExecutionProjection(cycle) {
			return AuthorityUnknown
		}
		if preserveContains(opts.PreserveCycleIDs, cycle.ID) && !preservableIdentityComplete(cycle) {
			return AuthorityUnknown
		}
		return AuthorityPreserved
	}
	if !executionIdentityComplete(cycle) {
		return AuthorityUnknown
	}
	if phase == "done" {
		if status == "released" && cycle.CompletionPresent && retainedLeaseIdentityComplete(cycle) {
			return AuthorityDead
		}
		return AuthorityUnknown
	}
	if cycle.CompletionPresent {
		return AuthorityUnknown
	}
	if preserveContains(opts.PreserveCycleIDs, cycle.ID) {
		if !preservableIdentityComplete(cycle) {
			return AuthorityUnknown
		}
		return AuthorityPreserved
	}
	if status == "active" {
		if !activeHolderIdentityComplete(cycle) {
			return AuthorityUnknown
		}
		switch strings.TrimSpace(cycle.HolderProcessStatus) {
		case ProcessStatusLive:
			return AuthorityLive
		case ProcessStatusDead, ProcessStatusIdentityMismatch:
			return AuthorityDead
		default:
			return AuthorityUnknown
		}
	}
	if retainedLeaseIdentityComplete(cycle) {
		return AuthorityPreserved
	}
	return AuthorityUnknown
}

func Classify(snapshot Snapshot, opts Options) Result {
	builder := findingBuilder{seen: map[string]struct{}{}}
	if opts.Now.IsZero() {
		builder.add(FindingInventoryUnknown, "clock", "now", "operational classification requires an explicit current time", "")
	}
	for _, problem := range snapshot.InventoryProblems {
		id := firstNonEmpty(problem.Code, problem.Source, "inventory")
		builder.add(FindingInventoryUnknown, problem.Source, id, firstNonEmpty(problem.Detail, problem.Code, "inventory collection failed"), "")
	}
	validateCanonicalSource(&builder, snapshot)
	if snapshot.OrcaObserved {
		runtimeID := strings.TrimSpace(snapshot.OrcaRuntimeID)
		repoID := strings.TrimSpace(snapshot.OrcaRepoID)
		if runtimeID == "" {
			builder.add(FindingInventoryUnknown, "orca_runtime", "runtime", "observed Orca inventory has no runtime identity", "")
		}
		if repoID == "" {
			builder.add(FindingInventoryUnknown, "orca_repo", "repo", "observed Orca inventory has no repository identity", clean(snapshot.RepoRoot))
		}
		for _, worktree := range snapshot.OrcaWorktrees {
			if strings.TrimSpace(worktree.RuntimeID) != runtimeID || strings.TrimSpace(worktree.RepoID) != repoID {
				builder.add(FindingInventoryUnknown, "worktree", strings.TrimSpace(worktree.ID), "Orca worktree runtime or repository identity does not match the observed inventory", clean(worktree.Path))
			}
		}
		for _, terminal := range snapshot.Terminals {
			if strings.TrimSpace(terminal.RuntimeID) != runtimeID {
				builder.add(FindingInventoryUnknown, "terminal", strings.TrimSpace(terminal.Handle), "Orca terminal runtime identity does not match the observed runtime", clean(terminal.WorktreePath))
			}
		}
		for _, task := range snapshot.Tasks {
			if strings.TrimSpace(task.RuntimeID) != runtimeID {
				builder.add(FindingInventoryUnknown, "task", strings.TrimSpace(task.ID), "Orca task runtime identity does not match the observed runtime", "")
			}
		}
		for _, dispatch := range snapshot.Dispatches {
			if strings.TrimSpace(dispatch.RuntimeID) != runtimeID {
				builder.add(FindingInventoryUnknown, "dispatch", strings.TrimSpace(dispatch.ID), "Orca dispatch runtime identity does not match the observed runtime", "")
			}
		}
		for _, gate := range snapshot.Gates {
			if strings.TrimSpace(gate.RuntimeID) != runtimeID {
				builder.add(FindingInventoryUnknown, "gate", strings.TrimSpace(gate.ID), "Orca gate runtime identity does not match the observed runtime", "")
			}
		}
		if strings.TrimSpace(snapshot.Messages.RuntimeID) != runtimeID {
			builder.add(FindingInventoryUnknown, "message", "inbox", "Orca inbox runtime identity does not match the observed runtime", "")
		}
		validateCanonicalOrcaSource(&builder, snapshot)
	}

	cycleCounts := countBy(snapshot.Cycles, func(cycle Cycle) string { return strings.TrimSpace(cycle.ID) })
	worktreeCounts := countBy(snapshot.OrcaWorktrees, func(worktree OrcaWorktree) string { return strings.TrimSpace(worktree.ID) })
	instanceCounts := countBy(snapshot.OrcaWorktrees, func(worktree OrcaWorktree) string { return strings.TrimSpace(worktree.InstanceID) })
	terminalCounts := countBy(snapshot.Terminals, func(terminal OrcaTerminal) string { return strings.TrimSpace(terminal.Handle) })
	ptyCounts := countBy(snapshot.Terminals, func(terminal OrcaTerminal) string { return strings.TrimSpace(terminal.PTYID) })
	taskCounts := countBy(snapshot.Tasks, func(task OrcaTask) string { return orcaTaskKey(task.RunID, task.ID) })
	dispatchCounts := countBy(snapshot.Dispatches, func(dispatch OrcaDispatch) string { return strings.TrimSpace(dispatch.ID) })
	gateCounts := countBy(snapshot.Gates, func(gate OrcaGate) string { return strings.TrimSpace(gate.ID) })
	gitPathCounts := countBy(snapshot.GitWorktrees, func(worktree GitWorktree) string { return clean(worktree.Path) })

	addDuplicateFindings(&builder, "cycle", cycleCounts)
	addDuplicateFindings(&builder, "worktree", worktreeCounts)
	addDuplicateFindings(&builder, "worktree_instance", instanceCounts)
	addDuplicateFindings(&builder, "terminal", terminalCounts)
	addDuplicateFindings(&builder, "pty", ptyCounts)
	addDuplicateFindings(&builder, "task", taskCounts)
	addDuplicateFindings(&builder, "dispatch", dispatchCounts)
	addDuplicateFindings(&builder, "gate", gateCounts)
	addDuplicateFindings(&builder, "git_worktree", gitPathCounts)
	validateLeaseHolderIndexes(&builder, snapshot.Cycles, snapshot.LeaseHolderIndexes)

	authorities := make(map[string]CycleAuthority, len(snapshot.Cycles))
	for _, cycle := range snapshot.Cycles {
		id := strings.TrimSpace(cycle.ID)
		authority := EvaluateCycleAuthority(cycle, opts)
		authorities[id] = authority
		runtimeMismatch := strings.TrimSpace(cycle.OrcaRuntimeID) != strings.TrimSpace(snapshot.OrcaRuntimeID)
		repoMismatch := clean(cycle.Repo) == clean(snapshot.RepoRoot) && strings.TrimSpace(cycle.OrcaRepoID) != strings.TrimSpace(snapshot.OrcaRepoID)
		if snapshot.OrcaObserved && (strings.TrimSpace(cycle.OrcaRuntimeID) != "" || strings.TrimSpace(cycle.OrcaRepoID) != "") &&
			(runtimeMismatch || repoMismatch) {
			builder.add(FindingInventoryUnknown, "cycle", id, "cycle Orca runtime or repository identity does not match the observed inventory", clean(cycle.WorktreePath))
		}
		if cycle.ExecutionFailurePresent {
			builder.add(FindingExecutionFailure, "cycle", id, "cycle has a durable execution failure; inspect issueops list and reconcile the failed operation", clean(cycle.WorktreePath))
		}
		if cycle.CleanupFailurePresent {
			builder.add(FindingCleanupFailure, "cycle", id, "cycle has a durable cleanup failure; inspect issueops list and resume cleanup from preview", clean(cycle.WorktreePath))
		}
		if cycle.IssueCreateFailurePresent {
			builder.add(FindingIssueCreateFailure, "cycle", id, "cycle has an ambiguous or failed durable issue creation; run issueops remote reconcile-issue", clean(cycle.WorktreePath))
		}
		switch {
		case authority == AuthorityUnknown:
			builder.add(FindingInventoryUnknown, "cycle", id, "cycle phase, execution lease, or durable identity is unsupported or incomplete", clean(cycle.WorktreePath))
		case authority == AuthorityDead && strings.TrimSpace(cycle.Phase) != "done":
			builder.add(FindingDeadOwner, "cycle", id, "cycle execution lease has no live or preserved authority", clean(cycle.WorktreePath))
		}
	}

	activeCycles := make([]Cycle, 0, len(snapshot.Cycles))
	activeRepoCycles := make([]Cycle, 0, len(snapshot.Cycles))
	for _, cycle := range snapshot.Cycles {
		authority := authorities[strings.TrimSpace(cycle.ID)]
		if authority == AuthorityLive || authority == AuthorityPreserved {
			cycle = resolveLegacyCycleRun(cycle, snapshot.Tasks)
			activeCycles = append(activeCycles, cycle)
			if clean(cycle.Repo) == clean(snapshot.RepoRoot) {
				activeRepoCycles = append(activeRepoCycles, cycle)
			}
		}
	}

	worktreeOwners := ownerIndex(activeRepoCycles, func(cycle Cycle) string { return strings.TrimSpace(cycle.OrcaWorktreeID) })
	terminalOwners := make(map[string][]string)
	for _, cycle := range activeCycles {
		addOwner(terminalOwners, cycleTerminalHandle(cycle, snapshot), cycle.ID)
	}
	for handle := range terminalOwners {
		sort.Strings(terminalOwners[handle])
	}
	taskOwners := ownerIndex(activeCycles, func(cycle Cycle) string { return orcaTaskKey(cycle.RunID, cycle.TaskID) })
	dispatchOwners := ownerIndex(activeCycles, func(cycle Cycle) string { return strings.TrimSpace(cycle.DispatchID) })
	gitPathOwners := ownerIndex(activeRepoCycles, func(cycle Cycle) string { return clean(cycle.WorktreePath) })
	for kind, owners := range map[string]map[string][]string{
		"worktree": worktreeOwners, "terminal": terminalOwners, "task": taskOwners,
		"dispatch": dispatchOwners, "git_worktree": gitPathOwners,
	} {
		for id, cycleIDs := range owners {
			if len(cycleIDs) > 1 {
				builder.add(FindingInventoryUnknown, kind, id, "resource is claimed by multiple active cycles: "+strings.Join(cycleIDs, ","), "")
			}
		}
	}

	for _, cycle := range activeCycles {
		validateCycleResources(&builder, snapshot, cycle, authorities[strings.TrimSpace(cycle.ID)], clean(cycle.Repo) == clean(snapshot.RepoRoot), gitPathCounts, worktreeCounts, terminalCounts, ptyCounts, taskCounts, dispatchCounts)
	}

	preservedTerminalCounts, invalidPreserveTerminals := normalizedSet(opts.PreserveTerminalHandles)
	for _, invalid := range invalidPreserveTerminals {
		builder.add(FindingInventoryUnknown, "preserve_terminal", invalid, "preserved terminal handles must be non-empty and unique", "")
	}
	for handle := range preservedTerminalCounts {
		if terminalCounts[handle] != 1 {
			builder.add(FindingInventoryUnknown, "terminal", handle, "preserved terminal must exist exactly once", "")
		}
	}
	_, invalidPreserveCycles := normalizedSet(opts.PreserveCycleIDs)
	for _, invalid := range invalidPreserveCycles {
		builder.add(FindingInventoryUnknown, "preserve_cycle", invalid, "preserved cycle ids must be non-empty and unique", "")
	}
	for id := range preserveSet(opts.PreserveCycleIDs) {
		if cycleCounts[id] != 1 {
			builder.add(FindingInventoryUnknown, "cycle", id, "preserved cycle must exist exactly once", "")
		}
	}

	for _, worktree := range snapshot.GitWorktrees {
		path := clean(worktree.Path)
		if worktree.Canonical || (path == clean(snapshot.RepoRoot) && strings.TrimSpace(worktree.Branch) == strings.TrimSpace(snapshot.CanonicalBranch)) {
			continue
		}
		if len(gitPathOwners[path]) == 1 {
			continue
		}
		builder.add(FindingWorktreeResidue, "git_worktree", path, "non-canonical Git worktree has no live or invocation-preserved owner", path)
	}
	for _, worktree := range snapshot.OrcaWorktrees {
		id := strings.TrimSpace(worktree.ID)
		if clean(worktree.Path) == clean(snapshot.RepoRoot) && strings.TrimSpace(worktree.Branch) == strings.TrimSpace(snapshot.CanonicalBranch) {
			continue
		}
		if len(worktreeOwners[id]) == 1 {
			continue
		}
		builder.add(FindingWorktreeResidue, "orca_worktree", id, "Orca worktree has no live or invocation-preserved owner", clean(worktree.Path))
	}
	for _, terminal := range snapshot.Terminals {
		handle := strings.TrimSpace(terminal.Handle)
		if len(terminalOwners[handle]) == 1 || preservedTerminalCounts[handle] {
			continue
		}
		// Interactively, an unowned terminal is a live tab the user opened
		// directly; only a terminal claimed by multiple cycles is a conflict.
		if opts.Profile == ProfileInteractive && len(terminalOwners[handle]) == 0 {
			continue
		}
		builder.add(FindingTerminalResidue, "terminal", handle, "terminal has no live or invocation-preserved owner", clean(terminal.WorktreePath))
	}
	for _, task := range snapshot.Tasks {
		id := strings.TrimSpace(task.ID)
		key := orcaTaskKey(task.RunID, task.ID)
		status := strings.TrimSpace(task.Status)
		if !knownTaskStatus(status) {
			builder.add(FindingInventoryUnknown, "task", id, "task status is unsupported: "+status, "")
			continue
		}
		if status == "ready" && (!task.CompletedAt.IsZero() || task.HasResult) {
			builder.add(FindingTaskResidue, "task", id, "ready task carries completion metadata", "")
			continue
		}
		// A settled task holds no resource: it is orchestration history, unlike
		// a worktree or terminal. cleanup finish deletes the record that owned
		// it, so requiring an owner would flag every finished task forever, and
		// Orca exposes no per-task delete command to clear one (only a global
		// reset). Failed tasks stay observable through the failed-task listing.
		if settledTaskStatus(status) {
			continue
		}
		if len(taskOwners[key]) != 1 {
			builder.add(FindingTaskResidue, "task", id, "task has no live or invocation-preserved owner", "")
		}
	}
	for _, dispatch := range snapshot.Dispatches {
		id := strings.TrimSpace(dispatch.ID)
		status := strings.TrimSpace(dispatch.Status)
		if !knownDispatchStatus(status) {
			builder.add(FindingInventoryUnknown, "dispatch", id, "dispatch status is unsupported: "+status, "")
			continue
		}
		// task와 같은 이유로 종결된 dispatch는 건너뛴다: 그것은 잔여물이 아니라
		// 이력이고, Orca에 per-dispatch 삭제 명령이 없어 owner를 요구하면 끝난
		// dispatch를 영원히 residue로 보고한다(#171).
		if settledDispatchStatus(status) {
			continue
		}
		if len(dispatchOwners[id]) != 1 {
			builder.add(FindingTaskResidue, "dispatch", id, "dispatch has no live or invocation-preserved owner", "")
		}
	}
	for _, gate := range snapshot.Gates {
		id := strings.TrimSpace(gate.ID)
		status := strings.TrimSpace(gate.Status)
		if !knownGateStatus(status) {
			builder.add(FindingInventoryUnknown, "gate", id, "gate status is unsupported: "+status, "")
			continue
		}
		// Resolved and timed-out gates are durable orchestration history. Orca
		// exposes no per-gate delete command, so only a pending gate is residue.
		if settledGateStatus(status) {
			continue
		}
		builder.add(FindingGateResidue, "gate", id, "orchestration gate remains present", "")
	}
	classifyMessages(&builder, snapshot.Messages, opts.Profile)

	branchResidue := make([]string, 0)
	activeBranches := map[string]struct{}{strings.TrimSpace(snapshot.CanonicalBranch): {}}
	for _, cycle := range activeRepoCycles {
		if branch := strings.TrimSpace(cycle.Branch); branch != "" {
			activeBranches[branch] = struct{}{}
		}
	}
	for _, ref := range append(append([]GitRef(nil), snapshot.LocalRefs...), snapshot.RemoteRefs...) {
		branch := strings.TrimSpace(ref.Branch)
		if _, ok := activeBranches[branch]; ok {
			continue
		}
		branchResidue = append(branchResidue, fmt.Sprintf("%s:%s@%s", strings.TrimSpace(ref.Location), strings.TrimSpace(ref.Name), strings.TrimSpace(ref.OID)))
	}
	if len(branchResidue) > 0 {
		sort.Strings(branchResidue)
		builder.add(FindingNonMainBranchResidue, "branch", "non_main", fmt.Sprintf("%d non-canonical refs have no live or invocation-preserved owner: %s", len(branchResidue), strings.Join(branchResidue, ",")), "")
	}
	for _, artifact := range snapshot.StateArtifacts {
		path := clean(artifact.Path)
		builder.add(FindingStateArtifactResidue, "state_artifact", path, "state directory contains an unexpected runtime or recovery artifact", path)
	}

	findings := builder.sorted()
	return Result{Healthy: len(findings) == 0, Findings: findings}
}
