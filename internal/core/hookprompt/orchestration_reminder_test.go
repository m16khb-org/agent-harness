package hookprompt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/workpool"
)

func TestOrchestrationReminderRendersChildrenAndPool(t *testing.T) {
	repo, parent := seedOrchestrationReminderFixture(t)

	got := orchestrationReminderValue(repo)
	wantChildren := "children: 1/3 done, 1 unvalidated - issueops child status --parent " + parent.ID
	wantPool := "pool fanout: 0 leased / 1 pending / 0 expired - workpool status"
	for _, want := range []string{wantChildren, wantPool} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected orchestration reminder to contain %q, got %q", want, got)
		}
	}
}

func TestOrchestrationReminderAbsentWithoutBoundCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	if got := orchestrationReminderValue(repo); got != "" {
		t.Fatalf("expected no orchestration reminder without a bound cycle, got %q", got)
	}
}

func TestOrchestrationReminderIgnoresDroppedChild(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	now := "2026-07-16T00:00:00Z"
	childID := issueops.NewIssueOpsID(repo, "dropped-child")
	parent := issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            issueops.NewIssueOpsID(repo, "parent-with-dropped-child"),
		Repo:          repo,
		Branch:        "parent-with-dropped-child",
		Phase:         issueops.IssueOpsPhasePR,
		ChildCycles: []issueops.IssueOpsChildCycleRef{{
			CycleID: childID, Branch: "dropped-child", Title: "Dropped child", CreatedAt: now,
			ValidationVerdict: "dropped", ValidationReason: "parent already contains the verified change", ValidatedAt: now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	writeIssueOpsRecordForReminderTest(t, parent)
	writeIssueOpsRecordForReminderTest(t, issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            childID,
		Repo:          repo,
		Branch:        "dropped-child",
		Phase:         issueops.IssueOpsPhaseProblem,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	activateOrchestrationExecutionForTest(t, parent.ID, repo)

	if got := StopOrchestrationRelayFacts(repo); got != "" {
		t.Fatalf("dropped child must not keep Stop relay blocked, got %q", got)
	}
	if got := orchestrationReminderValue(repo); got != "" {
		t.Fatalf("dropped child must not remain in the active child reminder, got %q", got)
	}
}

func TestOrchestrationReminderIgnoresDoneBoundCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	now := "2026-07-16T00:00:00Z"
	childID := issueops.NewIssueOpsID(repo, "stale-active-child")
	parent := issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            issueops.NewIssueOpsID(repo, "done-parent"),
		Repo:          repo,
		Branch:        "done-parent",
		Phase:         issueops.IssueOpsPhaseDone,
		ChildCycles:   []issueops.IssueOpsChildCycleRef{{CycleID: childID, Branch: "stale-active-child", CreatedAt: now}},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	writeIssueOpsRecordForReminderTest(t, parent)
	writeIssueOpsRecordForReminderTest(t, issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            childID,
		Repo:          repo,
		Branch:        "stale-active-child",
		Phase:         issueops.IssueOpsPhaseProblem,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	activateOrchestrationExecutionForTest(t, parent.ID, repo)

	if got := StopOrchestrationRelayFacts(repo); got != "" {
		t.Fatalf("done bound cycle must not keep Stop relay blocked, got %q", got)
	}
	if got := orchestrationReminderValue(repo); got != "" {
		t.Fatalf("done bound cycle must not inject orchestration reminders, got %q", got)
	}
}

func TestOrchestrationRelayBoundCountsNonDroppedChildren(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	now := "2026-07-16T00:00:00Z"
	refs := make([]issueops.IssueOpsChildCycleRef, 0, orchestrationChildReadLimit+1)
	for i := 0; i < orchestrationChildReadLimit; i++ {
		refs = append(refs, issueops.IssueOpsChildCycleRef{
			CycleID: issueops.NewIssueOpsID(repo, "dropped-child-"+string(rune('a'+i))),
			Branch:  "dropped-child", CreatedAt: now, ValidationVerdict: "dropped", ValidatedAt: now,
		})
	}
	activeID := issueops.NewIssueOpsID(repo, "active-child-after-dropped-prefix")
	refs = append(refs, issueops.IssueOpsChildCycleRef{CycleID: activeID, Branch: "active-child", CreatedAt: now})
	parent := issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            issueops.NewIssueOpsID(repo, "parent-with-bounded-dropped-prefix"),
		Repo:          repo,
		Branch:        "parent-with-bounded-dropped-prefix",
		Phase:         issueops.IssueOpsPhaseImplement,
		ChildCycles:   refs,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	writeIssueOpsRecordForReminderTest(t, parent)
	writeIssueOpsRecordForReminderTest(t, issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            activeID,
		Repo:          repo,
		Branch:        "active-child",
		Phase:         issueops.IssueOpsPhaseImplement,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	activateOrchestrationExecutionForTest(t, parent.ID, repo)

	got := StopOrchestrationRelayFacts(repo)
	if !strings.Contains(got, "child_incomplete:"+activeID) {
		t.Fatalf("dropped children must not consume the bounded active-child scan, got %q", got)
	}
}

func TestUserPromptHintsIncludeOrchestrationReminder(t *testing.T) {
	repo, parent := seedOrchestrationReminderFixture(t)

	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "continue", Repo: repo})
	if !got.ShouldInject {
		t.Fatalf("expected user prompt hints to inject dynamic context")
	}
	want := "children: 1/3 done, 1 unvalidated - issueops child status --parent " + parent.ID
	if !strings.Contains(got.AdditionalContext, want) {
		t.Fatalf("expected additional context to include orchestration reminder %q, got %q", want, got.AdditionalContext)
	}
}

func seedOrchestrationReminderFixture(t *testing.T) (string, issueops.IssueOpsRecord) {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	now := "2026-07-07T00:00:00Z"
	parentID := issueops.NewIssueOpsID(repo, "orchestrate-parent")
	childDoneID := issueops.NewIssueOpsID(repo, "child-done")
	childActiveID := issueops.NewIssueOpsID(repo, "child-active")
	childAcceptedID := issueops.NewIssueOpsID(repo, "child-accepted")
	parent := issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            parentID,
		Repo:          repo,
		Branch:        "orchestrate-parent",
		Phase:         issueops.IssueOpsPhaseImplement,
		WorktreePath:  repo,
		ChildCycles: []issueops.IssueOpsChildCycleRef{
			{CycleID: childDoneID, Branch: "child-done", Title: "Done child", CreatedAt: now},
			{CycleID: childActiveID, Branch: "child-active", Title: "Active child", CreatedAt: now},
			{CycleID: childAcceptedID, Branch: "child-accepted", Title: "Accepted child", CreatedAt: now, ValidationVerdict: "accepted"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	writeIssueOpsRecordForReminderTest(t, parent)
	writeIssueOpsRecordForReminderTest(t, issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            childDoneID,
		Repo:          repo,
		Branch:        "child-done",
		Phase:         issueops.IssueOpsPhaseDone,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	writeIssueOpsRecordForReminderTest(t, issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            childActiveID,
		Repo:          repo,
		Branch:        "child-active",
		Phase:         issueops.IssueOpsPhaseImplement,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	writeIssueOpsRecordForReminderTest(t, issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            childAcceptedID,
		Repo:          repo,
		Branch:        "child-accepted",
		Phase:         issueops.IssueOpsPhaseImplement,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	activateOrchestrationExecutionForTest(t, parent.ID, repo)
	seedPoolManifestAndCorruptTaskForReminderTest(t, repo, parent.ID)
	return repo, parent
}

func writeIssueOpsRecordForReminderTest(t *testing.T, record issueops.IssueOpsRecord) {
	t.Helper()
	if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), record); err != nil {
		t.Fatalf("write issueops record %s: %v", record.ID, err)
	}
}

func activateOrchestrationExecutionForTest(t *testing.T, id, repo string) {
	t.Helper()
	record, err := issueops.ReadIssueOps(issueops.IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", record.Branch)
	record.WorktreePath = root
	record.Execution = &issueopsmodel.ExecutionV1{
		Mode: issueopsmodel.ExecutionModeDirect,
		Workspace: issueopsmodel.WorkspaceV1{
			SourceRoot: repo, Root: root, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueopsmodel.WriteLeaseV1{
			Generation: 1, Status: issueopsmodel.LeaseStatusActive, ClaimedAt: "2026-07-22T00:00:00Z",
			Holder: &issueopsmodel.NativeActorV1{
				Host: "codex", SessionID: "reminder-session",
				SessionProcess: &issueopsmodel.NativeProcessReceiptV1{PID: 1, StartedAt: "2026-07-22T00:00:00Z", Executable: "/usr/bin/codex"},
			},
		},
	}
	writeIssueOpsRecordForReminderTest(t, record)
}

func seedPoolManifestAndCorruptTaskForReminderTest(t *testing.T, repo, parentID string) {
	t.Helper()
	pool := workpool.WorkPool{
		SchemaVersion: workpool.WorkPoolCurrentSchemaVersion,
		ID:            "wp-reminder001",
		Repo:          repo,
		Name:          "fanout",
		ParentCycleID: parentID,
		Size:          3,
		LeaseTTL:      "30m",
		MaxAttempts:   2,
		Status:        "active",
		CreatedAt:     "2026-07-07T00:00:00Z",
		UpdatedAt:     "2026-07-07T00:00:00Z",
	}
	if err := os.MkdirAll(filepath.Join(workpool.StateRoot(), pool.ID), 0o700); err != nil {
		t.Fatalf("mkdir workpool fixture: %v", err)
	}
	b, err := json.MarshalIndent(pool, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workpool.StateRoot(), pool.ID+".json"), append(b, '\n'), 0o600); err != nil {
		t.Fatalf("write pool fixture: %v", err)
	}
	taskPath := filepath.Join(workpool.StateRoot(), pool.ID, "task-corrupt.json")
	if err := os.WriteFile(taskPath, []byte("{not-json\n"), 0o600); err != nil {
		t.Fatalf("write corrupt task fixture: %v", err)
	}
}
