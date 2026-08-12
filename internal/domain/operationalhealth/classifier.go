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

func validateCanonicalSource(builder *findingBuilder, snapshot Snapshot) {
	repo := clean(snapshot.RepoRoot)
	branch := strings.TrimSpace(snapshot.CanonicalBranch)
	head := strings.TrimSpace(snapshot.SourceHead)
	if repo == "" || branch == "" || head == "" {
		builder.add(FindingInventoryUnknown, "source", "source", "canonical source repository, branch, and HEAD identity must be complete", repo)
		return
	}
	if !snapshot.SourceClean {
		builder.add(FindingInventoryUnknown, "source", repo, "canonical source checkout is not clean", repo)
	}
	canonicalWorktrees := make([]GitWorktree, 0, 1)
	for _, worktree := range snapshot.GitWorktrees {
		if worktree.Canonical || clean(worktree.Path) == repo {
			canonicalWorktrees = append(canonicalWorktrees, worktree)
		}
	}
	if len(canonicalWorktrees) != 1 || clean(canonicalWorktrees[0].Path) != repo || !canonicalWorktrees[0].Canonical || strings.TrimSpace(canonicalWorktrees[0].Branch) != branch || strings.TrimSpace(canonicalWorktrees[0].Head) != head || !canonicalWorktrees[0].Clean {
		builder.add(FindingInventoryUnknown, "source_worktree", repo, "canonical Git worktree must occur once and match the clean source branch and HEAD", repo)
	}
	validateCanonicalRef(builder, snapshot.LocalRefs, "local", branch, head)
	validateCanonicalRef(builder, snapshot.RemoteRefs, "remote", branch, head)
}

func validateCanonicalOrcaSource(builder *findingBuilder, snapshot Snapshot) {
	repo := clean(snapshot.RepoRoot)
	branch := strings.TrimSpace(snapshot.CanonicalBranch)
	head := strings.TrimSpace(snapshot.SourceHead)
	runtimeID := strings.TrimSpace(snapshot.OrcaRuntimeID)
	repoID := strings.TrimSpace(snapshot.OrcaRepoID)
	matches := make([]OrcaWorktree, 0, 1)
	for _, worktree := range snapshot.OrcaWorktrees {
		if clean(worktree.Path) == repo {
			matches = append(matches, worktree)
		}
	}
	if len(matches) != 1 || strings.TrimSpace(matches[0].RuntimeID) != runtimeID || strings.TrimSpace(matches[0].RepoID) != repoID || clean(matches[0].Repo) != repo || strings.TrimSpace(matches[0].Branch) != branch || strings.TrimSpace(matches[0].Head) != head {
		builder.add(FindingInventoryUnknown, "orca_source_worktree", repo, "canonical Orca worktree must occur once and match the source runtime, repository, branch, and HEAD", repo)
	}
}

func validateCanonicalRef(builder *findingBuilder, refs []GitRef, location, branch, head string) {
	matches := make([]GitRef, 0, 1)
	for _, ref := range refs {
		if strings.TrimSpace(ref.Location) == location && strings.TrimSpace(ref.Branch) == branch {
			matches = append(matches, ref)
		}
	}
	if len(matches) != 1 || strings.TrimSpace(matches[0].OID) != head {
		builder.add(FindingInventoryUnknown, "branch", location+":"+branch, fmt.Sprintf("canonical %s ref must occur once at source HEAD %s", location, head), "")
	}
}

func knownPhase(value string) bool {
	switch value {
	case "problem", "grill", "plan", "compatibility-review", "implement", "ai-slop-clean", "feedback", "pr", "done":
		return true
	default:
		return false
	}
}

func knownLeaseStatus(value string) bool {
	switch value {
	case "", "claimable", "active", "revoking", "released":
		return true
	default:
		return false
	}
}

func hasExecutionProjection(cycle Cycle) bool {
	return strings.TrimSpace(cycle.ExecutionMode) != "" || cycle.Generation != 0 || strings.TrimSpace(cycle.WorktreePath) != "" ||
		!holderIdentityEmpty(cycle) || hasOrcaIdentity(cycle)
}

func executionIdentityComplete(cycle Cycle) bool {
	if strings.TrimSpace(cycle.Branch) == "" || strings.TrimSpace(cycle.WorktreePath) == "" || cycle.Generation == 0 {
		return false
	}
	switch strings.TrimSpace(cycle.ExecutionMode) {
	case "direct":
		return !hasOrcaIdentity(cycle)
	case "orca":
		return strings.TrimSpace(cycle.OrcaRuntimeID) != "" && strings.TrimSpace(cycle.OrcaRepoID) != "" &&
			strings.TrimSpace(cycle.OrcaWorktreeID) != "" && validOrcaOwnerHost(cycle.OrcaOwnerHost) &&
			strings.TrimSpace(cycle.TaskID) != "" && strings.TrimSpace(cycle.DispatchID) != ""
	default:
		return false
	}
}

func activeHolderIdentityComplete(cycle Cycle) bool {
	if !holderIdentityComplete(cycle) {
		return false
	}
	return strings.TrimSpace(cycle.ExecutionMode) != "orca" || strings.TrimSpace(cycle.OrcaOwnerHost) == strings.TrimSpace(cycle.HolderHost)
}

func retainedLeaseIdentityComplete(cycle Cycle) bool {
	switch strings.TrimSpace(cycle.LeaseStatus) {
	case "claimable", "released":
		return holderIdentityEmpty(cycle)
	case "revoking":
		return holderIdentityComplete(cycle)
	default:
		return false
	}

}

func holderIdentityComplete(cycle Cycle) bool {
	host := strings.TrimSpace(cycle.HolderHost)
	return validNativeHost(host) && strings.TrimSpace(cycle.HolderSessionID) != "" &&
		cycle.HolderPID > 0 && strings.TrimSpace(cycle.HolderStartedAt) != "" && strings.TrimSpace(cycle.HolderExecutable) != ""
}

func validNativeHost(host string) bool {
	host = strings.TrimSpace(host)
	return host == "codex" || host == "claude" || host == "omo"
}

func validOrcaOwnerHost(host string) bool {
	host = strings.TrimSpace(host)
	return host == "codex" || host == "claude" || host == "omo"
}

func holderIdentityEmpty(cycle Cycle) bool {
	return strings.TrimSpace(cycle.HolderHost) == "" && strings.TrimSpace(cycle.HolderSessionID) == "" &&
		strings.TrimSpace(cycle.HolderAgentID) == "" && cycle.HolderPID == 0 && strings.TrimSpace(cycle.HolderStartedAt) == "" &&
		strings.TrimSpace(cycle.HolderExecutable) == ""
}

func hasOrcaIdentity(cycle Cycle) bool {
	return strings.TrimSpace(cycle.OrcaRuntimeID) != "" || strings.TrimSpace(cycle.OrcaRepoID) != "" ||
		strings.TrimSpace(cycle.OrcaWorktreeID) != "" || strings.TrimSpace(cycle.OrcaWorktreeInstanceID) != "" ||
		strings.TrimSpace(cycle.OrcaOwnerHost) != "" || strings.TrimSpace(cycle.TerminalPTYID) != "" ||
		strings.TrimSpace(cycle.TaskID) != "" || strings.TrimSpace(cycle.DispatchID) != ""
}

// knownTaskStatus reports whether a status belongs to Orca's task vocabulary.
//
// The source is the Orca CLI itself, not another Go definition:
//
//	$ orca orchestration task-update --help
//	Notes:
//	  Valid --status values: pending, ready, dispatched, completed, failed, blocked.
//
// #136 matched this list against the adapter's defensive set instead, and both
// definitions ended up wrong together (#145). The adapter's mirror of this
// vocabulary lives in executionTerminalTaskStatus; the two must agree, and the
// CLI decides which values they agree on.
func knownTaskStatus(value string) bool {
	switch value {
	case "pending", "ready", "dispatched", "blocked":
		return true
	default:
		return settledTaskStatus(value)
	}
}

// settledTaskStatus reports whether a task reached a terminal state and can no
// longer be dispatched or hold a worker.
//
// Only these two of Orca's six statuses qualify. pending waits to be dispatched,
// blocked waits on a dependency, ready can be dispatched, and dispatched is
// running — all four can still hold or acquire a worker, so a task left in one
// of them without an owner is genuine residue.
//
// #136 widened this to six by mirroring the adapter's defensive set, which
// included values Orca rejects outright. #121's original pair was right (#145).
func settledTaskStatus(value string) bool {
	return value == "completed" || value == "failed"
}

// knownDispatchStatus는 Orca가 dispatch_contexts.status에 담을 수 있는 값을
// 판정한다. 어휘의 출처는 Orca 소스이지 우리 추론이 아니다:
//
//	src/main/runtime/orchestration/types.ts
//	export type DispatchStatus = 'pending' | 'dispatched' | 'completed' | 'failed' | 'circuit_broken'
//
//	src/main/runtime/orchestration/db.ts (dispatch_contexts 스키마)
//	CHECK(status IN ('pending', 'dispatched', 'completed', 'failed', 'circuit_broken'))
//
// CHECK 제약이 있으므로 그 밖의 값은 스키마 마이그레이션 없이는 저장될 수 없다.
// 그래도 미지 값을 unknown으로 떨어뜨리는 것은 유지한다 — Orca가 어휘를 넓히면
// 우리가 먼저 알아야 하고, 조용히 통과시키면 그 시점을 놓친다.
//
// dispatched 하나만 유효로 보던 동안 유효 어휘 5개 중 4개가 "unsupported"로
// 보고됐다. 유효 상태가 unknown으로 오면 진짜 미지 값이 그 사이에 묻힌다(#171).
func knownDispatchStatus(value string) bool {
	switch value {
	case "pending", "dispatched":
		return true
	default:
		return settledDispatchStatus(value)
	}
}

// settledDispatchStatus는 dispatch가 더 이상 워커를 잡거나 획득할 수 없는
// 상태인지 판정한다.
//
// failed가 여기 있는 이유는 db.ts의 failDispatch다:
//
//	const newStatus: DispatchStatus = newFailureCount >= 3 ? 'circuit_broken' : 'failed'
//	const taskStatus: TaskStatus = newStatus === 'circuit_broken' ? 'failed' : 'ready'
//
// dispatch가 failed면 그 **시도**가 끝난 것이고, 작업은 task가 ready로 돌아가
// 재dispatch를 기다린다 — 그 대기는 task 축(settledTaskStatus)이 본다. 재dispatch는
// 새 dispatch ID를 만들므로 이 dispatch로 돌아오지 않는다.
//
// circuit_broken은 coordinator.ts가 재dispatch하지 않고 failedTasks에 넣는
// 종착점이다. 워커를 다시 붙일 경로가 없다.
func settledDispatchStatus(value string) bool {
	return value == "completed" || value == "failed" || value == "circuit_broken"
}

// knownGateStatus의 어휘도 Orca 소스가 정한다:
//
//	src/main/runtime/orchestration/types.ts
//	export type GateStatus = 'pending' | 'resolved' | 'timeout'
//
// timeout이 빠져 있던 동안 시간 초과된 gate가 "unsupported"로 보고됐다(#171).
func knownGateStatus(value string) bool {
	switch value {
	case "pending":
		return true
	default:
		return settledGateStatus(value)
	}
}

func settledGateStatus(value string) bool {
	return value == "resolved" || value == "timeout"
}

func preservableIdentityComplete(cycle Cycle) bool {
	if strings.TrimSpace(cycle.ID) == "" || strings.TrimSpace(cycle.Repo) == "" || strings.TrimSpace(cycle.Branch) == "" {
		return false
	}
	status := strings.TrimSpace(cycle.LeaseStatus)
	if status == "" {
		return !hasExecutionProjection(cycle)
	}
	if !knownLeaseStatus(status) || !executionIdentityComplete(cycle) {
		return false
	}
	if status == "active" {
		return activeHolderIdentityComplete(cycle) && strings.TrimSpace(cycle.HolderProcessStatus) == ProcessStatusLive
	}
	return retainedLeaseIdentityComplete(cycle)
}

func validateCycleResources(builder *findingBuilder, snapshot Snapshot, cycle Cycle, authority CycleAuthority, repoScoped bool, gitPathCounts, worktreeCounts, terminalCounts, ptyCounts, taskCounts, dispatchCounts map[string]int) {
	var gitWorktree GitWorktree
	gitWorktreeOK := false
	if repoScoped && (authority == AuthorityLive || strings.TrimSpace(cycle.WorktreePath) != "") {
		gitWorktree, gitWorktreeOK = uniqueBy(snapshot.GitWorktrees, cycle.WorktreePath, func(value GitWorktree) string { return clean(value.Path) })
		if !gitWorktreeOK || gitPathCounts[clean(cycle.WorktreePath)] != 1 || strings.TrimSpace(gitWorktree.Branch) != strings.TrimSpace(cycle.Branch) {
			builder.add(FindingInventoryUnknown, "git_worktree", clean(cycle.WorktreePath), "cycle identity does not match exactly one Git worktree", clean(cycle.WorktreePath))
		}
	}
	isOrca := strings.TrimSpace(cycle.ExecutionMode) == "orca"
	if repoScoped && (isOrca || strings.TrimSpace(cycle.OrcaWorktreeID) != "") {
		worktree, ok := uniqueBy(snapshot.OrcaWorktrees, cycle.OrcaWorktreeID, func(value OrcaWorktree) string { return value.ID })
		headMismatch := gitWorktreeOK && strings.TrimSpace(worktree.Head) != strings.TrimSpace(gitWorktree.Head)
		instanceMismatch := strings.TrimSpace(cycle.OrcaWorktreeInstanceID) != "" && strings.TrimSpace(worktree.InstanceID) != strings.TrimSpace(cycle.OrcaWorktreeInstanceID)
		if !ok || worktreeCounts[strings.TrimSpace(cycle.OrcaWorktreeID)] != 1 || strings.TrimSpace(worktree.RuntimeID) != strings.TrimSpace(cycle.OrcaRuntimeID) || strings.TrimSpace(worktree.RepoID) != strings.TrimSpace(cycle.OrcaRepoID) || instanceMismatch || clean(worktree.Repo) != clean(cycle.Repo) || clean(worktree.Path) != clean(cycle.WorktreePath) || strings.TrimSpace(worktree.Branch) != strings.TrimSpace(cycle.Branch) || headMismatch {
			builder.add(FindingInventoryUnknown, "worktree", cycle.OrcaWorktreeID, "cycle worktree identity does not match exactly one Orca worktree", clean(cycle.WorktreePath))
		}
	}
	var dispatch OrcaDispatch
	dispatchOK := false
	if isOrca || strings.TrimSpace(cycle.DispatchID) != "" {
		dispatch, dispatchOK = uniqueBy(snapshot.Dispatches, cycle.DispatchID, func(value OrcaDispatch) string { return value.ID })
		statusMismatch := authority == AuthorityLive && strings.TrimSpace(dispatch.Status) != "dispatched"
		if !dispatchOK || dispatchCounts[cycle.DispatchID] != 1 || strings.TrimSpace(dispatch.RuntimeID) != strings.TrimSpace(cycle.OrcaRuntimeID) ||
			orcaTaskKey(dispatch.RunID, dispatch.TaskID) != orcaTaskKey(cycle.RunID, cycle.TaskID) || statusMismatch {
			builder.add(FindingInventoryUnknown, "dispatch", cycle.DispatchID, "cycle dispatch identity does not match exactly one expected dispatch", "")
		}
	}
	if isOrca || strings.TrimSpace(cycle.TerminalPTYID) != "" {
		handle := strings.TrimSpace(dispatch.AssigneeHandle)
		var terminal OrcaTerminal
		terminalOK := false
		if strings.TrimSpace(cycle.TerminalPTYID) != "" {
			terminal, terminalOK = uniqueBy(snapshot.Terminals, cycle.TerminalPTYID, func(value OrcaTerminal) string { return value.PTYID })
		} else if handle != "" {
			terminal, terminalOK = uniqueBy(snapshot.Terminals, handle, func(value OrcaTerminal) string { return value.Handle })
		}
		liveMismatch := authority == AuthorityLive && (!terminal.Connected || !terminal.Writable)
		countMismatch := terminalCounts[strings.TrimSpace(terminal.Handle)] != 1 || (strings.TrimSpace(cycle.TerminalPTYID) != "" && ptyCounts[cycle.TerminalPTYID] != 1)
		if !terminalOK || countMismatch || strings.TrimSpace(terminal.RuntimeID) != strings.TrimSpace(cycle.OrcaRuntimeID) ||
			(dispatchOK && strings.TrimSpace(terminal.Handle) != handle) || strings.TrimSpace(terminal.WorktreeID) != strings.TrimSpace(cycle.OrcaWorktreeID) ||
			clean(terminal.WorktreePath) != clean(cycle.WorktreePath) || liveMismatch {
			builder.add(FindingInventoryUnknown, "terminal", firstNonEmpty(handle, cycle.TerminalPTYID), "cycle terminal identity or liveness does not match exactly one expected terminal", clean(cycle.WorktreePath))
		}
	}
	if isOrca || strings.TrimSpace(cycle.TaskID) != "" {
		taskKey := orcaTaskKey(cycle.RunID, cycle.TaskID)
		task, ok := uniqueBy(snapshot.Tasks, taskKey, func(value OrcaTask) string { return orcaTaskKey(value.RunID, value.ID) })
		if !ok || taskCounts[taskKey] != 1 || strings.TrimSpace(task.RuntimeID) != strings.TrimSpace(cycle.OrcaRuntimeID) || (authority == AuthorityLive && strings.TrimSpace(task.Status) != "dispatched") || (strings.TrimSpace(task.DispatchID) != "" && strings.TrimSpace(task.DispatchID) != strings.TrimSpace(cycle.DispatchID)) {
			builder.add(FindingInventoryUnknown, "task", cycle.TaskID, "cycle task identity or status does not match exactly one task", "")
		}
	}
}

func orcaTaskKey(runID, taskID string) string {
	runID, taskID = strings.TrimSpace(runID), strings.TrimSpace(taskID)
	if taskID == "" {
		return ""
	}
	return runID + "\x00" + taskID
}

func resolveLegacyCycleRun(cycle Cycle, tasks []OrcaTask) Cycle {
	if strings.TrimSpace(cycle.RunID) != "" || strings.TrimSpace(cycle.TaskID) == "" {
		return cycle
	}
	var candidate *OrcaTask
	for index := range tasks {
		if strings.TrimSpace(tasks[index].ID) != strings.TrimSpace(cycle.TaskID) {
			continue
		}
		if candidate != nil {
			return cycle
		}
		candidate = &tasks[index]
	}
	if candidate != nil {
		cycle.RunID = strings.TrimSpace(candidate.RunID)
	}
	return cycle
}

func validateLeaseHolderIndexes(builder *findingBuilder, cycles []Cycle, indexes []LeaseHolderIndex) {
	active := make([]Cycle, 0, len(cycles))
	holderOwners := make(map[string][]string)
	for _, cycle := range cycles {
		if strings.TrimSpace(cycle.LeaseStatus) != "active" {
			continue
		}
		active = append(active, cycle)
		addOwner(holderOwners, nativeHolderIdentity(cycle.HolderHost, cycle.HolderSessionID, cycle.HolderAgentID), cycle.ID)
	}
	for identity, owners := range holderOwners {
		if identity != "" && len(owners) > 1 {
			sort.Strings(owners)
			builder.add(FindingInventoryUnknown, "lease_holder", identity, "native session is recorded as holder of multiple active cycles: "+strings.Join(owners, ","), "")
		}
	}
	keyCounts := countBy(indexes, func(index LeaseHolderIndex) string { return strings.TrimSpace(index.Key) })
	addDuplicateFindings(builder, "lease_holder", keyCounts)
	for _, cycle := range active {
		matches := 0
		for _, index := range indexes {
			if leaseHolderIndexMatchesCycle(index, cycle) {
				matches++
			}
		}
		if matches != 1 {
			builder.add(FindingInventoryUnknown, "lease_holder", strings.TrimSpace(cycle.ID), "active cycle must match exactly one lease-holder reverse index", "")
		}
	}
	for _, index := range indexes {
		valid := strings.TrimSpace(index.Key) != "" && strings.TrimSpace(index.LifecycleID) != "" && index.Generation > 0 &&
			validNativeHost(index.Host) && strings.TrimSpace(index.SessionID) != ""
		matches := 0
		for _, cycle := range active {
			if leaseHolderIndexMatchesCycle(index, cycle) {
				matches++
			}
		}
		if !valid || matches != 1 {
			builder.add(FindingInventoryUnknown, "lease_holder", firstNonEmpty(strings.TrimSpace(index.Key), strings.TrimSpace(index.LifecycleID), "index"), "lease-holder reverse index must match exactly one active cycle", "")
		}
	}
}

func leaseHolderIndexMatchesCycle(index LeaseHolderIndex, cycle Cycle) bool {
	return strings.TrimSpace(index.LifecycleID) == strings.TrimSpace(cycle.ID) && index.Generation == cycle.Generation &&
		strings.TrimSpace(index.Host) == strings.TrimSpace(cycle.HolderHost) &&
		strings.TrimSpace(index.SessionID) == strings.TrimSpace(cycle.HolderSessionID) &&
		strings.TrimSpace(index.AgentID) == strings.TrimSpace(cycle.HolderAgentID)
}

func nativeHolderIdentity(host, sessionID, agentID string) string {
	host, sessionID, agentID = strings.TrimSpace(host), strings.TrimSpace(sessionID), strings.TrimSpace(agentID)
	if host == "" || sessionID == "" {
		return ""
	}
	return strings.ToLower(host) + "\x00" + sessionID + "\x00" + agentID
}

func classifyMessages(builder *findingBuilder, messages MessagePresence, profile string) {
	if messages.Count < 0 {
		builder.add(FindingInventoryUnknown, "message", "inbox", "message count is invalid", "")
		return
	}
	// Interactively, the orchestration inbox is durable message history the
	// CLI cannot purge; rows only prove past coordination, not residue.
	if profile == ProfileInteractive {
		return
	}
	if messages.Count > 0 || !messages.Empty {
		builder.add(FindingMessageResidue, "message", "inbox", fmt.Sprintf("orchestration inbox returned %d message rows", messages.Count), "")
		return
	}
	if !messages.CompleteAbsence {
		builder.add(FindingInventoryUnknown, "message", "inbox", "bounded inbox observation does not prove absence", "")
	}
}

func preserveContains(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want && want != "" {
			return true
		}
	}
	return false
}

func preserveSet(values []string) map[string]bool {
	result, _ := normalizedSet(values)
	return result
}

func normalizedSet(values []string) (map[string]bool, []string) {
	result := make(map[string]bool, len(values))
	invalid := make([]string, 0)
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || result[value] {
			invalid = append(invalid, value)
			continue
		}
		result[value] = true
	}
	return result, invalid
}

func ownerIndex(cycles []Cycle, identity func(Cycle) string) map[string][]string {
	owners := make(map[string][]string)
	for _, cycle := range cycles {
		addOwner(owners, identity(cycle), cycle.ID)
	}
	for id := range owners {
		sort.Strings(owners[id])
	}
	return owners
}

func addOwner(owners map[string][]string, resourceID, cycleID string) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return
	}
	owners[resourceID] = append(owners[resourceID], strings.TrimSpace(cycleID))
}

func cycleTerminalHandle(cycle Cycle, snapshot Snapshot) string {
	if ptyID := strings.TrimSpace(cycle.TerminalPTYID); ptyID != "" {
		if terminal, ok := uniqueBy(snapshot.Terminals, ptyID, func(value OrcaTerminal) string { return value.PTYID }); ok {
			return strings.TrimSpace(terminal.Handle)
		}
	}
	if dispatchID := strings.TrimSpace(cycle.DispatchID); dispatchID != "" {
		if dispatch, ok := uniqueBy(snapshot.Dispatches, dispatchID, func(value OrcaDispatch) string { return value.ID }); ok {
			return strings.TrimSpace(dispatch.AssigneeHandle)
		}
	}
	return ""
}

func countBy[T any](values []T, identity func(T) string) map[string]int {
	counts := make(map[string]int)
	for _, value := range values {
		if id := identity(value); id != "" {
			counts[id]++
		}
	}
	return counts
}

func addDuplicateFindings(builder *findingBuilder, kind string, counts map[string]int) {
	for id, count := range counts {
		if count > 1 {
			builder.add(FindingInventoryUnknown, kind, id, fmt.Sprintf("%s identity occurs %d times", kind, count), "")
		}
	}
}

func uniqueBy[T any](values []T, want string, identity func(T) string) (T, bool) {
	var zero T
	var found T
	count := 0
	for _, value := range values {
		if strings.TrimSpace(identity(value)) == strings.TrimSpace(want) {
			found = value
			count++
		}
	}
	if count != 1 {
		return zero, false
	}
	return found, true
}

func clean(path string) string {
	return strings.TrimSpace(path)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

type findingBuilder struct {
	findings []Finding
	seen     map[string]struct{}
}

func (builder *findingBuilder) add(code, kind, id, summary, path string) {
	finding := Finding{Code: code, ResourceKind: kind, ResourceID: id, Summary: summary, Path: path}
	key := strings.Join([]string{code, kind, id, path, summary}, "\x00")
	if _, exists := builder.seen[key]; exists {
		return
	}
	builder.seen[key] = struct{}{}
	builder.findings = append(builder.findings, finding)
}

func (builder *findingBuilder) sorted() []Finding {
	sort.Slice(builder.findings, func(i, j int) bool {
		left := builder.findings[i]
		right := builder.findings[j]
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.ResourceKind != right.ResourceKind {
			return left.ResourceKind < right.ResourceKind
		}
		if left.ResourceID != right.ResourceID {
			return left.ResourceID < right.ResourceID
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Summary < right.Summary
	})
	return builder.findings
}
