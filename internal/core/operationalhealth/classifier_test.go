package operationalhealth

import (
	"testing"
	"time"
)

func TestClassifyOperationalHealthRejectsBoundCycleWithoutFreshHeartbeat(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	snapshot := Snapshot{
		Cycles: []Cycle{{
			ID:              "io-dead-owner",
			Repo:            "/repo",
			Branch:          "1-dead-owner",
			Phase:           "implement",
			HandoffState:    "claimed",
			Attempt:         1,
			OwnershipEpoch:  "epoch-1",
			ContextSHA256:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			WorkerSessionID: "session-1",
			WorkerAgentID:   "agent-1",
			TaskID:          "task-1",
		}},
		Bindings: []Binding{{
			CycleID: "io-dead-owner",
			Repo:    "/repo",
			Branch:  "1-dead-owner",
		}},
		Tasks: []OrcaTask{{ID: "task-1", Status: "ready"}},
	}

	result := Classify(snapshot, Options{Now: now})

	if result.Healthy {
		t.Fatalf("bound cycle without a fresh heartbeat classified healthy: %#v", result)
	}
	if !hasFinding(result.Findings, FindingDeadOwner, "io-dead-owner") {
		t.Fatalf("missing %s finding for dead owner: %#v", FindingDeadOwner, result.Findings)
	}
}

func TestClassifyOperationalHealthAcceptsFreshClaimedExactResources(t *testing.T) {
	now := operationalTestNow()

	result := Classify(healthyOperationalSnapshot(now), Options{Now: now})

	if !result.Healthy || len(result.Findings) != 0 {
		t.Fatalf("fresh exact claimed cycle should be healthy: %#v", result)
	}
}

func TestClassifyOperationalHealthRejectsCanonicalSourceDrift(t *testing.T) {
	now := operationalTestNow()
	for _, test := range []struct {
		name   string
		mutate func(*Snapshot)
		id     string
	}{
		{name: "dirty source", id: "/repo", mutate: func(snapshot *Snapshot) { snapshot.SourceClean = false }},
		{name: "source worktree head", id: "/repo", mutate: func(snapshot *Snapshot) { snapshot.GitWorktrees[0].Head = "different" }},
		{name: "local canonical ref", id: "local:main", mutate: func(snapshot *Snapshot) { snapshot.LocalRefs[0].OID = "different" }},
		{name: "missing remote canonical ref", id: "remote:main", mutate: func(snapshot *Snapshot) { snapshot.RemoteRefs = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := minimalOperationalSnapshot()
			test.mutate(&snapshot)

			result := Classify(snapshot, Options{Now: now})

			if !hasFinding(result.Findings, FindingInventoryUnknown, test.id) {
				t.Fatalf("canonical source drift was accepted: %#v", result.Findings)
			}
		})
	}
}

func TestClassifyOperationalHealthUsesGlobalOrcaOwnersAcrossRepos(t *testing.T) {
	now := operationalTestNow()
	snapshot := minimalOperationalSnapshot()
	snapshot.Cycles = []Cycle{{
		ID:                     "io-foreign-live",
		Repo:                   "/other",
		Branch:                 "2-foreign-live",
		Phase:                  "implement",
		HandoffState:           "claimed",
		Attempt:                1,
		OwnershipEpoch:         "epoch-foreign",
		ContextSHA256:          "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		WorkerSessionID:        "session-foreign",
		WorkerAgentID:          "agent-foreign",
		WorktreePath:           "/other.wt/2-foreign-live",
		OrcaWorktreeID:         "wt-foreign",
		OrcaWorktreeInstanceID: "instance-foreign",
		TerminalHandle:         "term-foreign",
		PTYID:                  "pty-foreign",
		TaskID:                 "task-foreign",
		DispatchID:             "dispatch-foreign",
		LastHeartbeatAt:        now.Add(-time.Minute),
	}}
	snapshot.Terminals = []OrcaTerminal{{
		Handle: "term-foreign", PTYID: "pty-foreign", WorktreeID: "wt-foreign",
		WorktreePath: "/other.wt/2-foreign-live", Connected: true, Writable: true,
	}}
	snapshot.Tasks = []OrcaTask{{ID: "task-foreign", Status: "dispatched", DispatchID: "dispatch-foreign"}}
	snapshot.Dispatches = []OrcaDispatch{{
		ID: "dispatch-foreign", TaskID: "task-foreign", AssigneeHandle: "term-foreign", Status: "dispatched",
	}}

	result := Classify(snapshot, Options{Now: now})

	if !result.Healthy || len(result.Findings) != 0 {
		t.Fatalf("foreign live cycle should own global Orca resources without requiring foreign repo worktree inventory: %#v", result)
	}
}

func TestClassifyOperationalHealthDoesNotLetForeignCycleOwnRequestedRepoResources(t *testing.T) {
	now := operationalTestNow()
	snapshot := minimalOperationalSnapshot()
	snapshot.Cycles = []Cycle{{
		ID: "io-foreign-preserved", Repo: "/other", Branch: "orphan", Phase: "plan",
		WorktreePath: "/repo.wt/orphan", OrcaWorktreeID: "wt-orphan", OrcaWorktreeInstanceID: "instance-orphan",
	}}
	snapshot.GitWorktrees = append(snapshot.GitWorktrees, GitWorktree{
		Path: "/repo.wt/orphan", Branch: "orphan", Head: "head-orphan",
	})
	snapshot.OrcaWorktrees = append(snapshot.OrcaWorktrees, OrcaWorktree{
		ID: "wt-orphan", InstanceID: "instance-orphan", Repo: "/repo",
		Path: "/repo.wt/orphan", Branch: "orphan", Head: "head-orphan",
	})
	snapshot.LocalRefs = append(snapshot.LocalRefs, GitRef{
		Name: "refs/heads/orphan", Branch: "orphan", OID: "head-orphan", Location: "local",
	})

	result := Classify(snapshot, Options{Now: now, PreserveCycleIDs: []string{"io-foreign-preserved"}})

	for _, expected := range []struct {
		code string
		id   string
	}{
		{FindingWorktreeResidue, "/repo.wt/orphan"},
		{FindingWorktreeResidue, "wt-orphan"},
		{FindingNonMainBranchResidue, "non_main"},
	} {
		if !hasFinding(result.Findings, expected.code, expected.id) {
			t.Fatalf("foreign cycle hid requested-repo residue %s/%s: %#v", expected.code, expected.id, result.Findings)
		}
	}
}

func TestClassifyOperationalHealthPreservesExactPlanningCycleForInvocation(t *testing.T) {
	now := operationalTestNow()
	snapshot := minimalOperationalSnapshot()
	snapshot.Cycles = []Cycle{{ID: "io-planning", Repo: "/repo", Branch: "main", Phase: "plan"}}
	snapshot.Bindings = []Binding{{CycleID: "io-planning", Repo: "/repo", Branch: "main"}}

	withoutPreserve := Classify(snapshot, Options{Now: now})
	withPreserve := Classify(snapshot, Options{Now: now, PreserveCycleIDs: []string{"io-planning"}})

	if !hasFinding(withoutPreserve.Findings, FindingDeadOwner, "io-planning") {
		t.Fatalf("persistent binding must not preserve planning cycle: %#v", withoutPreserve)
	}
	if !withPreserve.Healthy || len(withPreserve.Findings) != 0 {
		t.Fatalf("exact invocation preserve should keep planning cycle: %#v", withPreserve)
	}
}

func TestClassifyOperationalHealthPreservesOnlyCompleteExactNonClaimedCycles(t *testing.T) {
	now := operationalTestNow()
	for _, state := range []string{"", "coordinator_preparing", "dispatched", "submitted", "recovery_required"} {
		t.Run(firstNonEmpty(state, "no-handoff"), func(t *testing.T) {
			snapshot := minimalOperationalSnapshot()
			cycle := Cycle{ID: "io-preserved", Repo: "/repo", Branch: "main", Phase: "plan", HandoffState: state}
			if state != "" {
				cycle.Attempt = 1
				cycle.OwnershipEpoch = "epoch-preserved"
			}
			snapshot.Cycles = []Cycle{cycle}

			result := Classify(snapshot, Options{Now: now, PreserveCycleIDs: []string{"io-preserved"}})

			if !result.Healthy {
				t.Fatalf("exact %q cycle should be invocation-preserved: %#v", state, result.Findings)
			}
		})
	}

	t.Run("missing durable branch", func(t *testing.T) {
		snapshot := minimalOperationalSnapshot()
		snapshot.Cycles = []Cycle{{ID: "io-incomplete", Repo: "/repo", Phase: "plan"}}

		result := Classify(snapshot, Options{Now: now, PreserveCycleIDs: []string{"io-incomplete"}})

		if !hasFinding(result.Findings, FindingInventoryUnknown, "io-incomplete") {
			t.Fatalf("incomplete preserved cycle must remain unknown: %#v", result.Findings)
		}
	})
}

func TestClassifyOperationalHealthFailsClosedOnDuplicateResourceIdentity(t *testing.T) {
	now := operationalTestNow()
	snapshot := healthyOperationalSnapshot(now)
	snapshot.Tasks = append(snapshot.Tasks, snapshot.Tasks[0])

	result := Classify(snapshot, Options{Now: now})

	if !hasFinding(result.Findings, FindingInventoryUnknown, "task-1") {
		t.Fatalf("duplicate task identity must be unknown: %#v", result.Findings)
	}
}

func TestClassifyOperationalHealthFailsClosedOnDuplicateCycleOwner(t *testing.T) {
	now := operationalTestNow()
	snapshot := healthyOperationalSnapshot(now)
	snapshot.Cycles = append(snapshot.Cycles, snapshot.Cycles[0])

	result := Classify(snapshot, Options{Now: now})

	if !hasFinding(result.Findings, FindingInventoryUnknown, "io-live") {
		t.Fatalf("duplicate cycle owner must be unknown: %#v", result.Findings)
	}
}

func TestClassifyOperationalHealthFailsClosedOnIncompleteInventory(t *testing.T) {
	now := operationalTestNow()
	tests := []struct {
		name   string
		mutate func(*Snapshot)
		id     string
	}{
		{
			name: "collector problem",
			mutate: func(snapshot *Snapshot) {
				snapshot.InventoryProblems = []InventoryProblem{{Source: "orca_tasks", Code: "count_mismatch", Detail: "task inventory is incomplete"}}
			},
			id: "count_mismatch",
		},
		{
			name: "missing git worktree",
			mutate: func(snapshot *Snapshot) {
				snapshot.GitWorktrees = snapshot.GitWorktrees[:1]
			},
			id: "/repo.wt/1-live",
		},
		{
			name: "mismatched orca branch",
			mutate: func(snapshot *Snapshot) {
				snapshot.OrcaWorktrees[1].Branch = "different"
			},
			id: "wt-1",
		},
		{
			name: "mismatched orca runtime",
			mutate: func(snapshot *Snapshot) {
				snapshot.OrcaObserved = true
				snapshot.OrcaRuntimeID = "runtime-current"
				for index := range snapshot.OrcaWorktrees {
					snapshot.OrcaWorktrees[index].RuntimeID = "runtime-current"
				}
				for index := range snapshot.Terminals {
					snapshot.Terminals[index].RuntimeID = "runtime-current"
				}
				snapshot.OrcaWorktrees[1].RuntimeID = "runtime-stale"
			},
			id: "wt-1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := healthyOperationalSnapshot(now)
			test.mutate(&snapshot)

			result := Classify(snapshot, Options{Now: now})

			if !hasFinding(result.Findings, FindingInventoryUnknown, test.id) {
				t.Fatalf("%s must fail closed: %#v", test.name, result.Findings)
			}
		})
	}
}

func TestClassifyOperationalHealthRejectsCompletedMetadataOnReadyTask(t *testing.T) {
	now := operationalTestNow()
	snapshot := healthyOperationalSnapshot(now)
	snapshot.Tasks[0].Status = "ready"
	snapshot.Tasks[0].CompletedAt = now.Add(-time.Minute)
	snapshot.Tasks[0].HasResult = true

	result := Classify(snapshot, Options{Now: now})

	if !hasFinding(result.Findings, FindingTaskResidue, "task-1") {
		t.Fatalf("ready task with completion metadata must be residue: %#v", result.Findings)
	}
}

func TestClassifyOperationalHealthTreatsDoneOwnerResourcesAsResidue(t *testing.T) {
	now := operationalTestNow()
	snapshot := healthyOperationalSnapshot(now)
	snapshot.Cycles[0].Phase = "done"
	snapshot.Cycles[0].HandoffState = "closed"

	result := Classify(snapshot, Options{Now: now})

	for _, expected := range []struct {
		code string
		id   string
	}{
		{FindingWorktreeResidue, "wt-1"},
		{FindingTerminalResidue, "term-1"},
		{FindingTaskResidue, "task-1"},
	} {
		if !hasFinding(result.Findings, expected.code, expected.id) {
			t.Fatalf("done owner missing %s/%s residue: %#v", expected.code, expected.id, result.Findings)
		}
	}
}

func TestClassifyOperationalHealthReportsUnmatchedResources(t *testing.T) {
	now := operationalTestNow()
	snapshot := minimalOperationalSnapshot()
	snapshot.GitWorktrees = append(snapshot.GitWorktrees, GitWorktree{Path: "/repo.wt/orphan", Branch: "orphan", Head: "head-orphan"})
	snapshot.OrcaWorktrees = append(snapshot.OrcaWorktrees, OrcaWorktree{ID: "wt-orphan", InstanceID: "instance-orphan", Repo: "/repo", Path: "/repo.wt/orphan", Branch: "orphan"})
	snapshot.Terminals = []OrcaTerminal{{Handle: "term-orphan", PTYID: "pty-orphan", WorktreeID: "wt-orphan", WorktreePath: "/repo.wt/orphan", Connected: true, Writable: true}}
	snapshot.Tasks = []OrcaTask{{ID: "task-orphan", Status: "failed"}}
	snapshot.Gates = []OrcaGate{{ID: "gate-orphan", TaskID: "task-orphan", Status: "pending"}}
	snapshot.Messages = MessagePresence{Count: 1, Empty: false, CompleteAbsence: false}

	result := Classify(snapshot, Options{Now: now})

	for _, code := range []string{
		FindingWorktreeResidue,
		FindingTerminalResidue,
		FindingTaskResidue,
		FindingGateResidue,
		FindingMessageResidue,
	} {
		if !hasFindingCode(result.Findings, code) {
			t.Fatalf("missing %s for unmatched resource set: %#v", code, result.Findings)
		}
	}
}

func TestClassifyOperationalHealthAggregatesNonMainBranchResidue(t *testing.T) {
	now := operationalTestNow()
	snapshot := minimalOperationalSnapshot()
	snapshot.LocalRefs = append(snapshot.LocalRefs, GitRef{Name: "refs/heads/orphan", Branch: "orphan", OID: "oid-local", Location: "local"})
	snapshot.RemoteRefs = append(snapshot.RemoteRefs, GitRef{Name: "refs/heads/orphan", Branch: "orphan", OID: "oid-remote", Location: "remote"})

	result := Classify(snapshot, Options{Now: now})

	if got := findingCount(result.Findings, FindingNonMainBranchResidue); got != 1 {
		t.Fatalf("branch residue finding count = %d, want 1: %#v", got, result.Findings)
	}
}

func TestClassifyOperationalHealthReportsUnexpectedStateArtifact(t *testing.T) {
	now := operationalTestNow()
	snapshot := minimalOperationalSnapshot()
	snapshot.StateArtifacts = []StateArtifact{{Path: "/state/recovery.patch", Code: "unexpected_file"}}

	result := Classify(snapshot, Options{Now: now})

	if !hasFinding(result.Findings, FindingStateArtifactResidue, "/state/recovery.patch") {
		t.Fatalf("unexpected state artifact must be residue: %#v", result.Findings)
	}
}

func TestClassifyOperationalHealthFailsClosedOnUnknownState(t *testing.T) {
	now := operationalTestNow()
	tests := []struct {
		name   string
		mutate func(*Snapshot)
		id     string
	}{
		{name: "phase", id: "io-live", mutate: func(snapshot *Snapshot) { snapshot.Cycles[0].Phase = "mystery" }},
		{name: "handoff", id: "io-live", mutate: func(snapshot *Snapshot) { snapshot.Cycles[0].HandoffState = "mystery" }},
		{name: "task", id: "task-1", mutate: func(snapshot *Snapshot) { snapshot.Tasks[0].Status = "mystery" }},
		{name: "gate", id: "gate-1", mutate: func(snapshot *Snapshot) { snapshot.Gates = []OrcaGate{{ID: "gate-1", Status: "mystery"}} }},
		{name: "non-contract gate", id: "gate-1", mutate: func(snapshot *Snapshot) { snapshot.Gates = []OrcaGate{{ID: "gate-1", Status: "approved"}} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := healthyOperationalSnapshot(now)
			test.mutate(&snapshot)

			result := Classify(snapshot, Options{Now: now})

			if !hasFinding(result.Findings, FindingInventoryUnknown, test.id) {
				t.Fatalf("unknown %s state must fail closed: %#v", test.name, result.Findings)
			}
		})
	}
}

func TestClassifyOperationalHealthUsesExactHeartbeatBoundary(t *testing.T) {
	now := operationalTestNow()
	atBoundary := healthyOperationalSnapshot(now)
	atBoundary.Cycles[0].LastHeartbeatAt = now.Add(-HeartbeatTTL)
	beyondBoundary := healthyOperationalSnapshot(now)
	beyondBoundary.Cycles[0].LastHeartbeatAt = now.Add(-HeartbeatTTL - time.Nanosecond)

	boundaryResult := Classify(atBoundary, Options{Now: now})
	beyondResult := Classify(beyondBoundary, Options{Now: now})

	if !boundaryResult.Healthy {
		t.Fatalf("heartbeat exactly at TTL should be live: %#v", boundaryResult)
	}
	if !hasFinding(beyondResult.Findings, FindingDeadOwner, "io-live") {
		t.Fatalf("heartbeat beyond TTL should be dead: %#v", beyondResult)
	}
}

func TestClassifyOperationalHealthRejectsMissingAndFutureHeartbeatWithExactResources(t *testing.T) {
	now := operationalTestNow()
	tests := []struct {
		name      string
		heartbeat time.Time
	}{
		{name: "missing"},
		{name: "future", heartbeat: now.Add(time.Nanosecond)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := healthyOperationalSnapshot(now)
			snapshot.Cycles[0].LastHeartbeatAt = test.heartbeat

			result := Classify(snapshot, Options{Now: now})

			if !hasFinding(result.Findings, FindingDeadOwner, "io-live") {
				t.Fatalf("%s heartbeat must not confer liveness: %#v", test.name, result.Findings)
			}
		})
	}
}

func TestClassifyOperationalHealthRejectsBlankAndDuplicatePreserveValues(t *testing.T) {
	now := operationalTestNow()
	tests := []struct {
		name string
		opts Options
	}{
		{name: "blank cycle", opts: Options{Now: now, PreserveCycleIDs: []string{" "}}},
		{name: "duplicate cycle", opts: Options{Now: now, PreserveCycleIDs: []string{"io-preserved", " io-preserved "}}},
		{name: "blank terminal", opts: Options{Now: now, PreserveTerminalHandles: []string{" "}}},
		{name: "duplicate terminal", opts: Options{Now: now, PreserveTerminalHandles: []string{"term-current", " term-current "}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := minimalOperationalSnapshot()
			snapshot.Cycles = []Cycle{{ID: "io-preserved", Repo: "/repo", Branch: "main", Phase: "plan"}}
			snapshot.Terminals = []OrcaTerminal{{Handle: "term-current"}}

			result := Classify(snapshot, test.opts)

			if !hasFindingCode(result.Findings, FindingInventoryUnknown) {
				t.Fatalf("invalid preserve value must fail closed: %#v", result.Findings)
			}
		})
	}
}

func TestClassifyOperationalHealthRequiresUniquePreservedTerminal(t *testing.T) {
	now := operationalTestNow()
	tests := []struct {
		name      string
		terminals []OrcaTerminal
	}{
		{name: "missing"},
		{name: "duplicate", terminals: []OrcaTerminal{{Handle: "term-current"}, {Handle: "term-current"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := minimalOperationalSnapshot()
			snapshot.Terminals = test.terminals

			result := Classify(snapshot, Options{Now: now, PreserveTerminalHandles: []string{"term-current"}})

			if !hasFinding(result.Findings, FindingInventoryUnknown, "term-current") {
				t.Fatalf("%s preserved terminal must be unknown: %#v", test.name, result.Findings)
			}
		})
	}
}

func operationalTestNow() time.Time {
	return time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
}

func minimalOperationalSnapshot() Snapshot {
	return Snapshot{
		RepoRoot:        "/repo",
		CanonicalBranch: "main",
		SourceHead:      "head-main",
		SourceClean:     true,
		GitWorktrees: []GitWorktree{{
			Path:      "/repo",
			Branch:    "main",
			Head:      "head-main",
			Clean:     true,
			Canonical: true,
		}},
		LocalRefs:  []GitRef{{Name: "refs/heads/main", Branch: "main", OID: "head-main", Location: "local"}},
		RemoteRefs: []GitRef{{Name: "refs/heads/main", Branch: "main", OID: "head-main", Location: "remote"}},
		OrcaWorktrees: []OrcaWorktree{{
			ID:         "wt-main",
			InstanceID: "instance-main",
			Repo:       "/repo",
			Path:       "/repo",
			Branch:     "main",
			Head:       "head-main",
		}},
		Messages: MessagePresence{Count: 0, Empty: true, CompleteAbsence: true},
	}
}

func healthyOperationalSnapshot(now time.Time) Snapshot {
	snapshot := minimalOperationalSnapshot()
	snapshot.Cycles = []Cycle{{
		ID:                     "io-live",
		Repo:                   "/repo",
		Branch:                 "1-live",
		Phase:                  "implement",
		HandoffState:           "claimed",
		Attempt:                1,
		OwnershipEpoch:         "epoch-1",
		ContextSHA256:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		WorkerSessionID:        "session-1",
		WorkerAgentID:          "agent-1",
		WorktreePath:           "/repo.wt/1-live",
		OrcaWorktreeID:         "wt-1",
		OrcaWorktreeInstanceID: "instance-1",
		TerminalHandle:         "term-1",
		PTYID:                  "pty-1",
		TaskID:                 "task-1",
		DispatchID:             "dispatch-1",
		LastHeartbeatAt:        now.Add(-5 * time.Minute),
	}}
	snapshot.Bindings = []Binding{{CycleID: "io-live", Repo: "/repo", Branch: "1-live", ExpectedWorktree: "/repo.wt/1-live"}}
	snapshot.GitWorktrees = append(snapshot.GitWorktrees, GitWorktree{Path: "/repo.wt/1-live", Branch: "1-live", Head: "head-live", Clean: false})
	snapshot.LocalRefs = append(snapshot.LocalRefs, GitRef{Name: "refs/heads/1-live", Branch: "1-live", OID: "head-live", Location: "local"})
	snapshot.RemoteRefs = append(snapshot.RemoteRefs, GitRef{Name: "refs/heads/1-live", Branch: "1-live", OID: "head-live", Location: "remote"})
	snapshot.OrcaWorktrees = append(snapshot.OrcaWorktrees, OrcaWorktree{ID: "wt-1", InstanceID: "instance-1", Repo: "/repo", Path: "/repo.wt/1-live", Branch: "1-live", Head: "head-live"})
	snapshot.Terminals = []OrcaTerminal{{Handle: "term-1", PTYID: "pty-1", WorktreeID: "wt-1", WorktreePath: "/repo.wt/1-live", Connected: true, Writable: true}}
	snapshot.Tasks = []OrcaTask{{ID: "task-1", Status: "dispatched", DispatchID: "dispatch-1"}}
	snapshot.Dispatches = []OrcaDispatch{{ID: "dispatch-1", TaskID: "task-1", AssigneeHandle: "term-1", Status: "dispatched"}}
	return snapshot
}

func hasFinding(findings []Finding, code, resourceID string) bool {
	for _, finding := range findings {
		if finding.Code == code && finding.ResourceID == resourceID {
			return true
		}
	}
	return false
}

func hasFindingCode(findings []Finding, code string) bool {
	return findingCount(findings, code) > 0
}

func findingCount(findings []Finding, code string) int {
	count := 0
	for _, finding := range findings {
		if finding.Code == code {
			count++
		}
	}
	return count
}
