package operationalhealth

import (
	"testing"
	"time"
)

func TestEvaluateCycleAuthorityUsesExecutionLease(t *testing.T) {
	base := Cycle{
		ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: "implement",
		ExecutionMode: "direct", LeaseStatus: "active", Generation: 1,
		WorktreePath: "/repo.worktrees/69-v1", HolderHost: "codex", HolderSessionID: "session",
		HolderPID: 123, HolderStartedAt: "2026-07-22T00:00:00Z", HolderExecutable: "/opt/codex",
		HolderProcessStatus: ProcessStatusLive,
	}
	tests := []struct {
		name  string
		cycle Cycle
		want  CycleAuthority
	}{
		{name: "planning", cycle: Cycle{ID: "io-plan", Repo: "/repo", Branch: "69-plan", Phase: "plan"}, want: AuthorityPreserved},
		{name: "active", cycle: base, want: AuthorityLive},
		{name: "active dead process", cycle: withCycle(base, func(c *Cycle) { c.HolderProcessStatus = ProcessStatusDead }), want: AuthorityDead},
		{name: "active reused pid", cycle: withCycle(base, func(c *Cycle) { c.HolderProcessStatus = ProcessStatusIdentityMismatch }), want: AuthorityDead},
		{name: "active probe unknown", cycle: withCycle(base, func(c *Cycle) { c.HolderProcessStatus = ProcessStatusUnknown }), want: AuthorityUnknown},
		{name: "active missing process", cycle: withCycle(base, func(c *Cycle) { c.HolderPID = 0 }), want: AuthorityUnknown},
		{name: "claimable", cycle: withCycle(base, func(c *Cycle) { c.LeaseStatus = "claimable"; clearCycleHolder(c) }), want: AuthorityPreserved},
		{name: "revoking", cycle: withCycle(base, func(c *Cycle) { c.LeaseStatus = "revoking" }), want: AuthorityPreserved},
		{name: "released", cycle: withCycle(base, func(c *Cycle) { c.LeaseStatus = "released"; clearCycleHolder(c) }), want: AuthorityPreserved},
		{name: "done", cycle: withCycle(base, func(c *Cycle) {
			c.Phase = "done"
			c.LeaseStatus = "released"
			c.CompletionPresent = true
			clearCycleHolder(c)
		}), want: AuthorityDead},
		{name: "done active lease", cycle: withCycle(base, func(c *Cycle) { c.Phase = "done"; c.CompletionPresent = true }), want: AuthorityUnknown},
		{name: "done missing completion", cycle: withCycle(base, func(c *Cycle) { c.Phase = "done"; c.LeaseStatus = "released"; clearCycleHolder(c) }), want: AuthorityUnknown},
		{name: "invalid lease state", cycle: withCycle(base, func(c *Cycle) { c.LeaseStatus = "owner_active" }), want: AuthorityUnknown},
		{name: "invalid mode", cycle: withCycle(base, func(c *Cycle) { c.ExecutionMode = "inline" }), want: AuthorityUnknown},
		{name: "orca missing binding", cycle: withCycle(base, func(c *Cycle) { c.ExecutionMode = "orca" }), want: AuthorityUnknown},
		{name: "orca active", cycle: withCycle(base, func(c *Cycle) {
			c.ExecutionMode = "orca"
			c.OrcaRuntimeID, c.OrcaRepoID, c.OrcaWorktreeID = "runtime", "repo-id", "worktree-id"
			c.OrcaOwnerHost = "codex"
			c.TerminalPTYID, c.TaskID, c.DispatchID = "pty-id", "task-id", "dispatch-id"
		}), want: AuthorityLive},
		{name: "orca owner host mismatch", cycle: withCycle(base, func(c *Cycle) {
			c.ExecutionMode = "orca"
			c.OrcaRuntimeID, c.OrcaRepoID, c.OrcaWorktreeID = "runtime", "repo-id", "worktree-id"
			c.OrcaOwnerHost, c.TaskID, c.DispatchID = "claude", "task-id", "dispatch-id"
		}), want: AuthorityUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EvaluateCycleAuthority(test.cycle, Options{Now: time.Now()}); got != test.want {
				t.Fatalf("authority = %q, want %q", got, test.want)
			}
		})
	}
}

func TestClassifyRequiresExactLeaseHolderIndex(t *testing.T) {
	base := healthyDirectSnapshot()
	if result := Classify(base, Options{Now: time.Now()}); !result.Healthy {
		t.Fatalf("matched v1 inventory should be healthy: %#v", result.Findings)
	}

	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{name: "missing", mutate: func(snapshot *Snapshot) { snapshot.LeaseHolderIndexes = nil }},
		{name: "orphan", mutate: func(snapshot *Snapshot) {
			snapshot.LeaseHolderIndexes = append(snapshot.LeaseHolderIndexes, LeaseHolderIndex{Key: "orphan", LifecycleID: "io-orphan", Generation: 1, Host: "codex", SessionID: "other"})
		}},
		{name: "stale generation", mutate: func(snapshot *Snapshot) { snapshot.LeaseHolderIndexes[0].Generation++ }},
		{name: "wrong actor", mutate: func(snapshot *Snapshot) { snapshot.LeaseHolderIndexes[0].SessionID = "other" }},
		{name: "released retains index", mutate: func(snapshot *Snapshot) {
			snapshot.Cycles[0].LeaseStatus = "released"
			clearCycleHolder(&snapshot.Cycles[0])
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := base
			snapshot.Cycles = append([]Cycle(nil), base.Cycles...)
			snapshot.LeaseHolderIndexes = append([]LeaseHolderIndex(nil), base.LeaseHolderIndexes...)
			test.mutate(&snapshot)
			result := Classify(snapshot, Options{Now: time.Now()})
			if !hasFinding(result, FindingInventoryUnknown, "lease_holder") {
				t.Fatalf("findings = %#v, want lease-holder inventory failure", result.Findings)
			}
		})
	}
}

func TestClassifyAcceptsOmoNativeHolderAndIndex(t *testing.T) {
	snapshot := healthyDirectSnapshot()
	snapshot.Cycles[0].HolderHost = "omo"
	snapshot.Cycles[0].HolderExecutable = "omo"
	snapshot.LeaseHolderIndexes[0].Host = "omo"
	result := Classify(snapshot, Options{Now: time.Now()})
	if !result.Healthy {
		t.Fatalf("Omo native holder inventory should be healthy: %#v", result.Findings)
	}
}

func TestEvaluateCycleAuthorityAcceptsOmoAsOrcaOwner(t *testing.T) {
	cycle := healthyDirectSnapshot().Cycles[0]
	cycle.ExecutionMode = "orca"
	cycle.OrcaRuntimeID = "runtime"
	cycle.OrcaRepoID = "repo-id"
	cycle.OrcaWorktreeID = "worktree-id"
	cycle.OrcaOwnerHost = "omo"
	cycle.HolderHost = "omo"
	cycle.HolderExecutable = "omo"
	cycle.TaskID = "task-id"
	cycle.DispatchID = "dispatch-id"
	if got := EvaluateCycleAuthority(cycle, Options{Now: time.Now()}); got != AuthorityLive {
		t.Fatalf("Omo Orca owner authority = %q, want %q", got, AuthorityLive)
	}
}

func TestClassifyRejectsOneNativeSessionOwningTwoActiveCycles(t *testing.T) {
	snapshot := healthyDirectSnapshot()
	second := snapshot.Cycles[0]
	second.ID = "io-v2"
	second.Branch = "70-v1"
	second.WorktreePath = "/repo.worktrees/70-v1"
	snapshot.Cycles = append(snapshot.Cycles, second)
	snapshot.GitWorktrees = append(snapshot.GitWorktrees, GitWorktree{Path: second.WorktreePath, Branch: second.Branch, Head: snapshot.SourceHead, Clean: true})
	snapshot.LeaseHolderIndexes = append(snapshot.LeaseHolderIndexes, LeaseHolderIndex{Key: "second", LifecycleID: second.ID, Generation: second.Generation, Host: second.HolderHost, SessionID: second.HolderSessionID})
	result := Classify(snapshot, Options{Now: time.Now()})
	if !hasFinding(result, FindingInventoryUnknown, "lease_holder") {
		t.Fatalf("findings = %#v, want duplicate native-session finding", result.Findings)
	}
}

func TestClassifyAcceptsOrcaOptionalInstanceAndPTY(t *testing.T) {
	snapshot := healthyDirectSnapshot()
	cycle := &snapshot.Cycles[0]
	cycle.ExecutionMode = "orca"
	cycle.OrcaRuntimeID = "runtime"
	cycle.OrcaRepoID = "repo-id"
	cycle.OrcaWorktreeID = "worktree-id"
	cycle.OrcaOwnerHost = "codex"
	cycle.TaskID = "task-id"
	cycle.DispatchID = "dispatch-id"
	cycle.OrcaWorktreeInstanceID = ""
	cycle.TerminalPTYID = ""
	snapshot.OrcaObserved = true
	snapshot.OrcaRuntimeID = "runtime"
	snapshot.OrcaRepoID = "repo-id"
	snapshot.OrcaWorktrees = []OrcaWorktree{
		{RuntimeID: "runtime", RepoID: "repo-id", ID: "main-id", InstanceID: "main-instance", Repo: "/repo", Path: "/repo", Branch: "main", Head: snapshot.SourceHead},
		{RuntimeID: "runtime", RepoID: "repo-id", ID: "worktree-id", InstanceID: "observed-instance", Repo: "/repo", Path: cycle.WorktreePath, Branch: cycle.Branch, Head: snapshot.SourceHead},
	}
	snapshot.Terminals = []OrcaTerminal{{RuntimeID: "runtime", Handle: "terminal", PTYID: "observed-pty", WorktreeID: "worktree-id", WorktreePath: cycle.WorktreePath, Connected: true, Writable: true}}
	snapshot.Tasks = []OrcaTask{{RuntimeID: "runtime", RunID: "run-explicit", ID: "task-id", Status: "dispatched", DispatchID: "dispatch-id"}}
	snapshot.Dispatches = []OrcaDispatch{{RuntimeID: "runtime", RunID: "run-explicit", ID: "dispatch-id", TaskID: "task-id", AssigneeHandle: "terminal", Status: "dispatched"}}
	snapshot.Messages = MessagePresence{RuntimeID: "runtime", Empty: true, CompleteAbsence: true}
	if result := Classify(snapshot, Options{Now: time.Now()}); !result.Healthy {
		t.Fatalf("optional Orca identities should be healthy: %#v", result.Findings)
	}
}

func healthyDirectSnapshot() Snapshot {
	const head = "0123456789abcdef0123456789abcdef01234567"
	cycle := Cycle{
		ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: "implement",
		ExecutionMode: "direct", LeaseStatus: "active", Generation: 1,
		WorktreePath: "/repo.worktrees/69-v1", HolderHost: "codex", HolderSessionID: "session",
		HolderPID: 123, HolderStartedAt: "2026-07-22T00:00:00Z", HolderExecutable: "/opt/codex",
		HolderProcessStatus: ProcessStatusLive,
	}
	return Snapshot{
		RepoRoot: "/repo", CanonicalBranch: "main", SourceHead: head, SourceClean: true,
		Cycles:             []Cycle{cycle},
		LeaseHolderIndexes: []LeaseHolderIndex{{Key: "current", LifecycleID: cycle.ID, Generation: cycle.Generation, Host: cycle.HolderHost, SessionID: cycle.HolderSessionID}},
		GitWorktrees: []GitWorktree{
			{Path: "/repo", Branch: "main", Head: head, Clean: true, Canonical: true},
			{Path: cycle.WorktreePath, Branch: cycle.Branch, Head: head, Clean: true},
		},
		LocalRefs:  []GitRef{{Name: "refs/heads/main", Branch: "main", OID: head, Location: "local"}},
		RemoteRefs: []GitRef{{Name: "refs/remotes/origin/main", Branch: "main", OID: head, Location: "remote"}},
		Messages:   MessagePresence{Empty: true, CompleteAbsence: true},
	}
}

func hasFinding(result Result, code, kind string) bool {
	for _, finding := range result.Findings {
		if finding.Code == code && finding.ResourceKind == kind {
			return true
		}
	}
	return false
}

func TestKnownExecutionLeaseStatesRejectRemovedStates(t *testing.T) {
	for _, state := range []string{"", "claimable", "active", "revoking", "released"} {
		if !knownLeaseStatus(state) {
			t.Fatalf("v1 lease state %q rejected", state)
		}
	}
	for _, state := range []string{"owner_active", "ownership_dispatching", "closed"} {
		if knownLeaseStatus(state) {
			t.Fatalf("removed state %q remains accepted", state)
		}
	}
}

func clearCycleHolder(cycle *Cycle) {
	cycle.HolderHost = ""
	cycle.HolderSessionID = ""
	cycle.HolderAgentID = ""
	cycle.HolderPID = 0
	cycle.HolderStartedAt = ""
	cycle.HolderExecutable = ""
	cycle.HolderProcessStatus = ""
}

func withCycle(cycle Cycle, mutate func(*Cycle)) Cycle {
	mutate(&cycle)
	return cycle
}

func snapshotWithUserTerminalAndInboxHistory() Snapshot {
	snapshot := healthyDirectSnapshot()
	// A live Orca tab the user opened directly: no cycle references it, so it
	// has no invocation owner, but it is not residue on a live desktop.
	snapshot.Terminals = []OrcaTerminal{{RuntimeID: "runtime", Handle: "term_user_tab", WorktreePath: "/repo", Connected: true, Writable: true}}
	snapshot.Messages = MessagePresence{Count: 3}
	return snapshot
}

func TestClassifyInteractiveProfileAcceptsUserTerminalsAndInboxHistory(t *testing.T) {
	result := Classify(snapshotWithUserTerminalAndInboxHistory(), Options{Now: time.Now(), Profile: ProfileInteractive})
	if hasFinding(result, FindingTerminalResidue, "terminal") {
		t.Fatalf("interactive profile must not flag an unowned live terminal: %+v", result.Findings)
	}
	if hasFinding(result, FindingMessageResidue, "message") {
		t.Fatalf("interactive profile must not flag orchestration message history: %+v", result.Findings)
	}
	if !result.Healthy {
		t.Fatalf("interactive profile with only user terminals and message history must be healthy: %+v", result.Findings)
	}
}

// A settled task is orchestration history, not residue: cleanup finish deletes
// the record that owned it, so the owner index is permanently empty afterwards
// and Orca exposes no per-task delete command to reach it.
func TestClassifyExemptsSettledTasksFromOwnerRequirement(t *testing.T) {
	for _, status := range []string{"completed", "failed"} {
		snapshot := healthyDirectSnapshot()
		snapshot.Tasks = []OrcaTask{{RuntimeID: "runtime", ID: "task-settled", Status: status, CompletedAt: time.Now().UTC()}}
		result := Classify(snapshot, Options{Now: time.Now()})
		if hasFinding(result, FindingTaskResidue, "task") {
			t.Fatalf("settled %s task must not be task residue: %+v", status, result.Findings)
		}
	}
}

func TestClassifyStillFlagsUnsettledTasksWithoutOwner(t *testing.T) {
	for _, status := range []string{"ready", "dispatched"} {
		snapshot := healthyDirectSnapshot()
		snapshot.Tasks = []OrcaTask{{RuntimeID: "runtime", ID: "task-open", Status: status}}
		result := Classify(snapshot, Options{Now: time.Now()})
		if !hasFinding(result, FindingTaskResidue, "task") {
			t.Fatalf("unsettled %s task without an owner must stay flagged: %+v", status, result.Findings)
		}
	}
}

func TestClassifyKeepsReadyCompletionMetadataContradiction(t *testing.T) {
	snapshot := healthyDirectSnapshot()
	snapshot.Cycles[0].TaskID = "task-contradiction"
	snapshot.Tasks = []OrcaTask{{RuntimeID: "runtime", ID: "task-contradiction", Status: "ready", HasResult: true}}
	result := Classify(snapshot, Options{Now: time.Now()})
	if !hasFinding(result, FindingTaskResidue, "task") {
		t.Fatalf("ready task carrying completion metadata must stay flagged even with an owner: %+v", result.Findings)
	}
}

func TestClassifySealedProfileStillFlagsUnownedTerminalsAndMessages(t *testing.T) {
	for _, opts := range []Options{
		{Now: time.Now()},
		{Now: time.Now(), Profile: ProfileSealed},
	} {
		result := Classify(snapshotWithUserTerminalAndInboxHistory(), opts)
		if !hasFinding(result, FindingTerminalResidue, "terminal") {
			t.Fatalf("sealed/default profile must flag an unowned terminal: %+v", result.Findings)
		}
		if !hasFinding(result, FindingMessageResidue, "message") {
			t.Fatalf("sealed/default profile must flag inbox rows: %+v", result.Findings)
		}
	}
}
