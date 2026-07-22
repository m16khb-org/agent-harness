package issueops

import (
	"context"
	"reflect"
	"testing"

	"agent-harness/internal/port"
)

func TestWorkspacePreparationActorRequiresExactSourceSession(t *testing.T) {
	_, record := handoffPrepareRecord(t)
	session, err := validateWorkspacePreparationActor(record, "codex", IssueOpsActor{Host: "codex", SessionID: "session-1", AgentID: "agent-1", CWD: record.Repo})
	if err != nil {
		t.Fatal(err)
	}
	if session.Host != "codex" || session.SessionID != "session-1" || session.AgentID != "agent-1" {
		t.Fatalf("session = %#v", session)
	}
	for _, actor := range []IssueOpsActor{
		{Host: "", SessionID: "session-1", CWD: record.Repo},
		{Host: "codex", SessionID: "", CWD: record.Repo},
		{Host: "claude", SessionID: "session-1", CWD: record.Repo},
		{Host: "codex", SessionID: "session-1", CWD: record.Repo + ".worktrees/other"},
	} {
		if _, err := validateWorkspacePreparationActor(record, "codex", actor); err == nil {
			t.Fatalf("invalid actor unexpectedly authorized: %#v", actor)
		}
	}
}

func TestReadyWorkspaceWorktreeToolsRequireExactPreparationActor(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"}, create: port.OrcaWorktree{ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1)}}
	materializePrepareWorktreeOnCreate(t, client, worktree)
	if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "prepare-1", AgentID: "agent-1", SourceCWD: record.Repo, Confirm: true}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	prep := IssueOpsWorktreeToolPreparation{OK: true, WorktreePath: worktree}
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if _, err := RecordIssueOpsWorktreeTools(stateRoot, record.ID, prep); err == nil {
		t.Fatal("actorless workspace recorder unexpectedly succeeded")
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, before) {
		t.Fatal("actorless workspace recorder mutated the record")
	}
	if _, err := RecordIssueOpsWorktreeToolsWithActor(stateRoot, record.ID, IssueOpsActor{Host: "codex", SessionID: "other", AgentID: "agent-1", CWD: worktree}, prep); err == nil {
		t.Fatal("different preparation actor unexpectedly succeeded")
	}
	persisted, err := RecordIssueOpsWorktreeToolsWithActor(stateRoot, record.ID, IssueOpsActor{Host: "codex", SessionID: "prepare-1", AgentID: "agent-1", CWD: worktree}, prep)
	if err != nil {
		t.Fatal(err)
	}
	workspace := currentIssueOpsWorkspace(persisted)
	if persisted.WorktreeTools == nil || currentIssueOpsHandoff(persisted) != nil || workspace == nil || workspace.State != "ready" {
		t.Fatalf("exact preparation actor did not record worktree tools: %#v", persisted)
	}
	before = rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(persisted.Phase)); err == nil {
		t.Fatal("actorless ready-workspace phase recorder unexpectedly succeeded")
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, before) {
		t.Fatal("actorless ready-workspace phase recorder mutated the record")
	}
	if _, err := AdvanceIssueOpsPhaseWithActor(stateRoot, record.ID, string(persisted.Phase), IssueOpsActor{Host: "codex", SessionID: "prepare-1", AgentID: "agent-1", CWD: worktree}); err != nil {
		t.Fatalf("exact preparation actor phase recorder: %v", err)
	}
	before = rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	planPrep := IssueOpsPlanPrepRequest{PriorDecisions: IssueOpsPlanPrepItemRequest{WaiveReason: "not-needed"}, RelatedIssues: IssueOpsPlanPrepItemRequest{WaiveReason: "not-needed"}, WebResearch: IssueOpsPlanPrepItemRequest{WaiveReason: "not-needed"}}
	if _, err := RecordIssueOpsPlanPrep(stateRoot, record.ID, planPrep); err == nil {
		t.Fatal("actorless ready-workspace plan-prep unexpectedly succeeded")
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, before) {
		t.Fatal("actorless ready-workspace plan-prep mutated the record")
	}
	if _, err := RecordIssueOpsPlanPrepWithActor(stateRoot, record.ID, planPrep, IssueOpsActor{Host: "codex", SessionID: "prepare-1", AgentID: "agent-1", CWD: worktree}); err != nil {
		t.Fatalf("exact preparation actor plan-prep recorder: %v", err)
	}
}

func TestReadyWorkspaceRejectsActorlessPreparationMutators(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"}, create: port.OrcaWorktree{ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1)}}
	materializePrepareWorktreeOnCreate(t, client, worktree)
	if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "prepare-1", SourceCWD: record.Repo, Confirm: true}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	for name, mutate := range map[string]func() error{
		"intent": func() error {
			_, err := RecordIssueOpsIntent(stateRoot, record.ID, IssueOpsIntentRecordRequest{RawRequest: "request", InterpretedIntent: "intent", SuccessCriteria: []string{"criterion"}})
			return err
		},
		"domain-review": func() error {
			_, err := RecordIssueOpsDomainReview(stateRoot, record.ID, IssueOpsDomainReviewRequest{ModelFit: "fit"})
			return err
		},
		"decision": func() error {
			_, err := AddIssueOpsDecision(stateRoot, record.ID, IssueOpsDecisionRecordRequest{Title: "decision", Body: "body", Kind: "scope"})
			return err
		},
		"routing": func() error {
			_, err := RecordIssueOpsRouting(stateRoot, record.ID, "plan", "karpathy")
			return err
		},
		"link-related": func() error {
			_, err := LinkIssueOpsRelated(stateRoot, record.ID, "blocks", "https://github.com/acme/repo/issues/99", "related")
			return err
		},
	} {
		if err := mutate(); err == nil {
			t.Fatalf("actorless %s mutation unexpectedly succeeded", name)
		}
		if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, before) {
			t.Fatalf("actorless %s mutation changed the record", name)
		}
	}
	actor := IssueOpsActor{Host: "codex", SessionID: "prepare-1", CWD: worktree}
	if _, err := RecordIssueOpsIntentWithActor(stateRoot, record.ID, IssueOpsIntentRecordRequest{RawRequest: "request", InterpretedIntent: "intent", SuccessCriteria: []string{"criterion"}}, actor); err != nil {
		t.Fatalf("exact actor intent mutation: %v", err)
	}
}
