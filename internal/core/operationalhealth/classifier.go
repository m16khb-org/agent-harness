package operationalhealth

import (
	"fmt"
	"sort"
	"strings"
)

func EvaluateCycleAuthority(cycle Cycle, opts Options) CycleAuthority {
	phase := strings.TrimSpace(cycle.Phase)
	state := strings.TrimSpace(cycle.HandoffState)
	if !knownPhase(phase) || strings.TrimSpace(cycle.ID) == "" || strings.TrimSpace(cycle.Repo) == "" {
		return AuthorityUnknown
	}
	if !knownHandoffState(state) {
		return AuthorityUnknown
	}
	if phase == "done" || state == "closed" {
		return AuthorityDead
	}
	if state == "claimed" {
		if !claimedIdentityComplete(cycle) || cycle.LastHeartbeatAt.IsZero() || opts.Now.IsZero() {
			return AuthorityDead
		}
		age := opts.Now.Sub(cycle.LastHeartbeatAt)
		if age < 0 || age > HeartbeatTTL {
			return AuthorityDead
		}
		return AuthorityLive
	}
	if preserveContains(opts.PreserveCycleIDs, cycle.ID) {
		if !preservableIdentityComplete(cycle) {
			return AuthorityUnknown
		}
		return AuthorityPreserved
	}
	return AuthorityDead
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
	if snapshot.OrcaObserved {
		runtimeID := strings.TrimSpace(snapshot.OrcaRuntimeID)
		if runtimeID == "" {
			builder.add(FindingInventoryUnknown, "orca_runtime", "runtime", "observed Orca inventory has no runtime identity", "")
		}
		for _, worktree := range snapshot.OrcaWorktrees {
			if strings.TrimSpace(worktree.RuntimeID) != runtimeID {
				builder.add(FindingInventoryUnknown, "worktree", strings.TrimSpace(worktree.ID), "Orca worktree runtime identity does not match the observed runtime", clean(worktree.Path))
			}
		}
		for _, terminal := range snapshot.Terminals {
			if strings.TrimSpace(terminal.RuntimeID) != runtimeID {
				builder.add(FindingInventoryUnknown, "terminal", strings.TrimSpace(terminal.Handle), "Orca terminal runtime identity does not match the observed runtime", clean(terminal.WorktreePath))
			}
		}
	}

	cycleCounts := countBy(snapshot.Cycles, func(cycle Cycle) string { return strings.TrimSpace(cycle.ID) })
	worktreeCounts := countBy(snapshot.OrcaWorktrees, func(worktree OrcaWorktree) string { return strings.TrimSpace(worktree.ID) })
	instanceCounts := countBy(snapshot.OrcaWorktrees, func(worktree OrcaWorktree) string { return strings.TrimSpace(worktree.InstanceID) })
	terminalCounts := countBy(snapshot.Terminals, func(terminal OrcaTerminal) string { return strings.TrimSpace(terminal.Handle) })
	ptyCounts := countBy(snapshot.Terminals, func(terminal OrcaTerminal) string { return strings.TrimSpace(terminal.PTYID) })
	taskCounts := countBy(snapshot.Tasks, func(task OrcaTask) string { return strings.TrimSpace(task.ID) })
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

	authorities := make(map[string]CycleAuthority, len(snapshot.Cycles))
	cycleByID := make(map[string]Cycle, len(snapshot.Cycles))
	for _, cycle := range snapshot.Cycles {
		id := strings.TrimSpace(cycle.ID)
		if id != "" && cycleCounts[id] == 1 {
			cycleByID[id] = cycle
		}
		authority := EvaluateCycleAuthority(cycle, opts)
		authorities[id] = authority
		switch {
		case authority == AuthorityUnknown:
			builder.add(FindingInventoryUnknown, "cycle", id, "cycle phase, handoff state, or durable identity is unsupported or incomplete", clean(cycle.WorktreePath))
		case authority == AuthorityDead && strings.TrimSpace(cycle.Phase) != "done" && strings.TrimSpace(cycle.HandoffState) != "closed":
			builder.add(FindingDeadOwner, "cycle", id, "cycle owner has no fresh fenced heartbeat or invocation preservation", clean(cycle.WorktreePath))
		}
	}

	for _, binding := range snapshot.Bindings {
		cycle, ok := cycleByID[strings.TrimSpace(binding.CycleID)]
		if !ok || clean(binding.Repo) != clean(cycle.Repo) || (strings.TrimSpace(binding.Branch) != "" && strings.TrimSpace(binding.Branch) != strings.TrimSpace(cycle.Branch)) {
			builder.add(FindingInventoryUnknown, "binding", strings.TrimSpace(binding.CycleID), "session binding does not match one durable cycle", clean(binding.ExpectedWorktree))
		}
	}

	activeCycles := make([]Cycle, 0, len(snapshot.Cycles))
	for _, cycle := range snapshot.Cycles {
		authority := authorities[strings.TrimSpace(cycle.ID)]
		if authority == AuthorityLive || authority == AuthorityPreserved {
			activeCycles = append(activeCycles, cycle)
		}
	}

	worktreeOwners := ownerIndex(activeCycles, func(cycle Cycle) string { return strings.TrimSpace(cycle.OrcaWorktreeID) })
	terminalOwners := ownerIndex(activeCycles, func(cycle Cycle) string { return strings.TrimSpace(cycle.TerminalHandle) })
	taskOwners := ownerIndex(activeCycles, func(cycle Cycle) string { return strings.TrimSpace(cycle.TaskID) })
	dispatchOwners := ownerIndex(activeCycles, func(cycle Cycle) string { return strings.TrimSpace(cycle.DispatchID) })
	gitPathOwners := ownerIndex(activeCycles, func(cycle Cycle) string { return clean(cycle.WorktreePath) })
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
		validateCycleResources(&builder, snapshot, cycle, authorities[strings.TrimSpace(cycle.ID)], gitPathCounts, worktreeCounts, terminalCounts, taskCounts, dispatchCounts)
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
		builder.add(FindingTerminalResidue, "terminal", handle, "terminal has no live or invocation-preserved owner", clean(terminal.WorktreePath))
	}
	for _, task := range snapshot.Tasks {
		id := strings.TrimSpace(task.ID)
		status := strings.TrimSpace(task.Status)
		if !knownTaskStatus(status) {
			builder.add(FindingInventoryUnknown, "task", id, "task status is unsupported: "+status, "")
			continue
		}
		if status == "ready" && (!task.CompletedAt.IsZero() || task.HasResult) {
			builder.add(FindingTaskResidue, "task", id, "ready task carries completion metadata", "")
			continue
		}
		if len(taskOwners[id]) != 1 {
			builder.add(FindingTaskResidue, "task", id, "task has no live or invocation-preserved owner", "")
		}
	}
	for _, dispatch := range snapshot.Dispatches {
		id := strings.TrimSpace(dispatch.ID)
		if strings.TrimSpace(dispatch.Status) != "dispatched" {
			builder.add(FindingInventoryUnknown, "dispatch", id, "dispatch status is unsupported: "+strings.TrimSpace(dispatch.Status), "")
			continue
		}
		if len(dispatchOwners[id]) != 1 {
			builder.add(FindingTaskResidue, "dispatch", id, "dispatch has no live or invocation-preserved owner", "")
		}
	}
	for _, gate := range snapshot.Gates {
		id := strings.TrimSpace(gate.ID)
		if !knownGateStatus(strings.TrimSpace(gate.Status)) {
			builder.add(FindingInventoryUnknown, "gate", id, "gate status is unsupported: "+strings.TrimSpace(gate.Status), "")
			continue
		}
		builder.add(FindingGateResidue, "gate", id, "orchestration gate remains present", "")
	}
	classifyMessages(&builder, snapshot.Messages)

	branchResidue := make([]string, 0)
	activeBranches := map[string]struct{}{strings.TrimSpace(snapshot.CanonicalBranch): {}}
	for _, cycle := range activeCycles {
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

func claimedIdentityComplete(cycle Cycle) bool {
	return strings.TrimSpace(cycle.ID) != "" && strings.TrimSpace(cycle.Repo) != "" && strings.TrimSpace(cycle.Branch) != "" &&
		cycle.Attempt > 0 && strings.TrimSpace(cycle.OwnershipEpoch) != "" && len(strings.TrimSpace(cycle.ContextSHA256)) == 64 &&
		strings.TrimSpace(cycle.WorkerSessionID) != "" && strings.TrimSpace(cycle.WorkerAgentID) != "" &&
		strings.TrimSpace(cycle.WorktreePath) != "" && strings.TrimSpace(cycle.OrcaWorktreeID) != "" && strings.TrimSpace(cycle.OrcaWorktreeInstanceID) != "" &&
		strings.TrimSpace(cycle.TerminalHandle) != "" && strings.TrimSpace(cycle.PTYID) != "" && strings.TrimSpace(cycle.TaskID) != "" && strings.TrimSpace(cycle.DispatchID) != ""
}

func knownPhase(value string) bool {
	switch value {
	case "problem", "grill", "plan", "compatibility-review", "implement", "ai-slop-clean", "feedback", "pr", "done":
		return true
	default:
		return false
	}
}

func knownHandoffState(value string) bool {
	switch value {
	case "", "coordinator_preparing", "dispatched", "claimed", "submitted", "closed", "recovery_required":
		return true
	default:
		return false
	}
}

func knownTaskStatus(value string) bool {
	switch value {
	case "ready", "dispatched", "completed", "failed":
		return true
	default:
		return false
	}
}

func knownGateStatus(value string) bool {
	switch value {
	case "pending", "resolved":
		return true
	default:
		return false
	}
}

func preservableIdentityComplete(cycle Cycle) bool {
	if strings.TrimSpace(cycle.ID) == "" || strings.TrimSpace(cycle.Repo) == "" || strings.TrimSpace(cycle.Branch) == "" {
		return false
	}
	state := strings.TrimSpace(cycle.HandoffState)
	if state == "claimed" || state == "closed" {
		return false
	}
	if state != "" && (cycle.Attempt <= 0 || strings.TrimSpace(cycle.OwnershipEpoch) == "") {
		return false
	}
	return completeGroup(cycle.WorktreePath, cycle.OrcaWorktreeID, cycle.OrcaWorktreeInstanceID) &&
		completeGroup(cycle.TerminalHandle, cycle.PTYID) && completeGroup(cycle.TaskID, cycle.DispatchID)
}

func completeGroup(values ...string) bool {
	present := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			present++
		}
	}
	return present == 0 || present == len(values)
}

func validateCycleResources(builder *findingBuilder, snapshot Snapshot, cycle Cycle, authority CycleAuthority, gitPathCounts, worktreeCounts, terminalCounts, taskCounts, dispatchCounts map[string]int) {
	var gitWorktree GitWorktree
	gitWorktreeOK := false
	if authority == AuthorityLive || strings.TrimSpace(cycle.WorktreePath) != "" {
		gitWorktree, gitWorktreeOK = uniqueBy(snapshot.GitWorktrees, cycle.WorktreePath, func(value GitWorktree) string { return clean(value.Path) })
		if !gitWorktreeOK || gitPathCounts[clean(cycle.WorktreePath)] != 1 || strings.TrimSpace(gitWorktree.Branch) != strings.TrimSpace(cycle.Branch) {
			builder.add(FindingInventoryUnknown, "git_worktree", clean(cycle.WorktreePath), "cycle identity does not match exactly one Git worktree", clean(cycle.WorktreePath))
		}
	}
	if authority == AuthorityLive || strings.TrimSpace(cycle.OrcaWorktreeID) != "" {
		worktree, ok := uniqueBy(snapshot.OrcaWorktrees, cycle.OrcaWorktreeID, func(value OrcaWorktree) string { return value.ID })
		headMismatch := gitWorktreeOK && strings.TrimSpace(worktree.Head) != strings.TrimSpace(gitWorktree.Head)
		if !ok || worktreeCounts[strings.TrimSpace(cycle.OrcaWorktreeID)] != 1 || strings.TrimSpace(worktree.InstanceID) != strings.TrimSpace(cycle.OrcaWorktreeInstanceID) || clean(worktree.Repo) != clean(cycle.Repo) || clean(worktree.Path) != clean(cycle.WorktreePath) || strings.TrimSpace(worktree.Branch) != strings.TrimSpace(cycle.Branch) || headMismatch {
			builder.add(FindingInventoryUnknown, "worktree", cycle.OrcaWorktreeID, "cycle worktree identity does not match exactly one Orca worktree", clean(cycle.WorktreePath))
		}
	}
	if authority == AuthorityLive || strings.TrimSpace(cycle.TerminalHandle) != "" {
		terminal, ok := uniqueBy(snapshot.Terminals, cycle.TerminalHandle, func(value OrcaTerminal) string { return value.Handle })
		if !ok || terminalCounts[cycle.TerminalHandle] != 1 || strings.TrimSpace(terminal.PTYID) != strings.TrimSpace(cycle.PTYID) || strings.TrimSpace(terminal.WorktreeID) != strings.TrimSpace(cycle.OrcaWorktreeID) || clean(terminal.WorktreePath) != clean(cycle.WorktreePath) || !terminal.Connected || !terminal.Writable {
			builder.add(FindingInventoryUnknown, "terminal", cycle.TerminalHandle, "cycle terminal identity or liveness does not match exactly one writable connected terminal", clean(cycle.WorktreePath))
		}
	}
	if authority == AuthorityLive || strings.TrimSpace(cycle.TaskID) != "" {
		task, ok := uniqueBy(snapshot.Tasks, cycle.TaskID, func(value OrcaTask) string { return value.ID })
		if !ok || taskCounts[cycle.TaskID] != 1 || (authority == AuthorityLive && strings.TrimSpace(task.Status) != "dispatched") || (strings.TrimSpace(task.DispatchID) != "" && strings.TrimSpace(task.DispatchID) != strings.TrimSpace(cycle.DispatchID)) {
			builder.add(FindingInventoryUnknown, "task", cycle.TaskID, "cycle task identity or status does not match exactly one task", "")
		}
	}
	if authority == AuthorityLive || strings.TrimSpace(cycle.DispatchID) != "" {
		dispatch, ok := uniqueBy(snapshot.Dispatches, cycle.DispatchID, func(value OrcaDispatch) string { return value.ID })
		if !ok || dispatchCounts[cycle.DispatchID] != 1 || strings.TrimSpace(dispatch.TaskID) != strings.TrimSpace(cycle.TaskID) || strings.TrimSpace(dispatch.AssigneeHandle) != strings.TrimSpace(cycle.TerminalHandle) || strings.TrimSpace(dispatch.Status) != "dispatched" {
			builder.add(FindingInventoryUnknown, "dispatch", cycle.DispatchID, "cycle dispatch identity does not match exactly one active dispatch", "")
		}
	}
}

func classifyMessages(builder *findingBuilder, messages MessagePresence) {
	if messages.Count < 0 {
		builder.add(FindingInventoryUnknown, "message", "inbox", "message count is invalid", "")
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
		id := identity(cycle)
		if id == "" {
			continue
		}
		owners[id] = append(owners[id], cycle.ID)
	}
	for id := range owners {
		sort.Strings(owners[id])
	}
	return owners
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
