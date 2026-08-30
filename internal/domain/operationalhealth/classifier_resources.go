package operationalhealth

import (
	"fmt"
	"sort"
	"strings"
)

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
