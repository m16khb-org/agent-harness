package hookprompt

import (
	hookpromptcontract "agent-harness/internal/contract/hookprompt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestOrchestrationReminderRendersChildrenOnly(t *testing.T) {
	repo, parent := seedOrchestrationReminderFixture(t)

	got := orchestrationReminderValue(repo)
	wantChildren := "children: 1/3 done, 1 unvalidated - issueops child status --parent " + parent.ID
	if !strings.Contains(got, wantChildren) {
		t.Fatalf("expected orchestration reminder to contain %q, got %q", wantChildren, got)
	}
	retiredNamespace := strings.Join([]string{"work", "pool"}, "")
	for _, unwanted := range []string{retiredNamespace, "pool fanout"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("orchestration reminder must not contain %q, got %q", unwanted, got)
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

func TestBoundOrchestrationCycleUsesSingleBulkSnapshot(t *testing.T) {
	repo := t.TempDir()
	previousScan := ScanReadableIssueOps
	previousList := ListIssueOpsIDs
	defer func() {
		ScanReadableIssueOps = previousScan
		ListIssueOpsIDs = previousList
	}()
	scanCalls := 0
	ScanReadableIssueOps = func(string) ([]issueopscontract.IssueOpsRecord, error) {
		scanCalls++
		return []issueopscontract.IssueOpsRecord{{
			OK:    true,
			ID:    "io-bound",
			Phase: issueopscontract.IssueOpsPhasePlan,
			Execution: &issueopscontract.Execution{
				Workspace: issueopscontract.Workspace{Root: repo},
			},
		}}, nil
	}
	ListIssueOpsIDs = func(string) ([]string, error) {
		t.Fatal("bound orchestration lookup must not list IDs")
		return nil, nil
	}

	record, ok := boundOrchestrationCycle(repo)

	if !ok || record.ID != "io-bound" || scanCalls != 1 {
		t.Fatalf("ok=%t record=%+v scan calls=%d", ok, record, scanCalls)
	}
}

func TestOrchestrationReminderIgnoresDroppedChild(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	now := "2026-07-16T00:00:00Z"
	childID := issueops.NewIssueOpsID(repo, "dropped-child")
	parent := issueopscontract.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            issueops.NewIssueOpsID(repo, "parent-with-dropped-child"),
		Repo:          repo,
		Branch:        "parent-with-dropped-child",
		Phase:         issueops.IssueOpsPhasePR,
		ChildCycles: []issueopscontract.IssueOpsChildCycleRef{{
			CycleID: childID, Branch: "dropped-child", Title: "Dropped child", CreatedAt: now,
			ValidationVerdict: "dropped", ValidationReason: "parent already contains the verified change", ValidatedAt: now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	writeIssueOpsRecordForReminderTest(t, parent)
	writeIssueOpsRecordForReminderTest(t, issueopscontract.IssueOpsRecord{
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
	parent := issueopscontract.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            issueops.NewIssueOpsID(repo, "done-parent"),
		Repo:          repo,
		Branch:        "done-parent",
		Phase:         issueops.IssueOpsPhaseDone,
		ChildCycles:   []issueopscontract.IssueOpsChildCycleRef{{CycleID: childID, Branch: "stale-active-child", CreatedAt: now}},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	writeIssueOpsRecordForReminderTest(t, parent)
	writeIssueOpsRecordForReminderTest(t, issueopscontract.IssueOpsRecord{
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
	refs := make([]issueopscontract.IssueOpsChildCycleRef, 0, orchestrationChildReadLimit+1)
	for i := 0; i < orchestrationChildReadLimit; i++ {
		refs = append(refs, issueopscontract.IssueOpsChildCycleRef{
			CycleID: issueops.NewIssueOpsID(repo, "dropped-child-"+string(rune('a'+i))),
			Branch:  "dropped-child", CreatedAt: now, ValidationVerdict: "dropped", ValidatedAt: now,
		})
	}
	activeID := issueops.NewIssueOpsID(repo, "active-child-after-dropped-prefix")
	refs = append(refs, issueopscontract.IssueOpsChildCycleRef{CycleID: activeID, Branch: "active-child", CreatedAt: now})
	parent := issueopscontract.IssueOpsRecord{
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
	writeIssueOpsRecordForReminderTest(t, issueopscontract.IssueOpsRecord{
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

	got := BuildUserPromptMCPHints(hookpromptcontract.HookUserPromptRequest{Prompt: "continue", Repo: repo})
	if !got.ShouldInject {
		t.Fatalf("expected user prompt hints to inject dynamic context")
	}
	want := "children: 1/3 done, 1 unvalidated - issueops child status --parent " + parent.ID
	if !strings.Contains(got.AdditionalContext, want) {
		t.Fatalf("expected additional context to include orchestration reminder %q, got %q", want, got.AdditionalContext)
	}
}

func seedOrchestrationReminderFixture(t *testing.T) (string, issueopscontract.IssueOpsRecord) {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	now := "2026-07-07T00:00:00Z"
	parentID := issueops.NewIssueOpsID(repo, "orchestrate-parent")
	childDoneID := issueops.NewIssueOpsID(repo, "child-done")
	childActiveID := issueops.NewIssueOpsID(repo, "child-active")
	childAcceptedID := issueops.NewIssueOpsID(repo, "child-accepted")
	parent := issueopscontract.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            parentID,
		Repo:          repo,
		Branch:        "orchestrate-parent",
		Phase:         issueops.IssueOpsPhaseImplement,
		WorktreePath:  repo,
		ChildCycles: []issueopscontract.IssueOpsChildCycleRef{
			{CycleID: childDoneID, Branch: "child-done", Title: "Done child", CreatedAt: now},
			{CycleID: childActiveID, Branch: "child-active", Title: "Active child", CreatedAt: now},
			{CycleID: childAcceptedID, Branch: "child-accepted", Title: "Accepted child", CreatedAt: now, ValidationVerdict: "accepted"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	writeIssueOpsRecordForReminderTest(t, parent)
	writeIssueOpsRecordForReminderTest(t, issueopscontract.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            childDoneID,
		Repo:          repo,
		Branch:        "child-done",
		Phase:         issueops.IssueOpsPhaseDone,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	writeIssueOpsRecordForReminderTest(t, issueopscontract.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            childActiveID,
		Repo:          repo,
		Branch:        "child-active",
		Phase:         issueops.IssueOpsPhaseImplement,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	writeIssueOpsRecordForReminderTest(t, issueopscontract.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            childAcceptedID,
		Repo:          repo,
		Branch:        "child-accepted",
		Phase:         issueops.IssueOpsPhaseImplement,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	activateOrchestrationExecutionForTest(t, parent.ID, repo)
	seedStaleLegacyPoolEntry(t, parent.ID, "wp-reminder001")
	return repo, parent
}

func writeIssueOpsRecordForReminderTest(t *testing.T, record issueopscontract.IssueOpsRecord) {
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
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: repo, Root: root, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusActive, ClaimedAt: "2026-07-22T00:00:00Z",
			Holder: &issueopscontract.NativeActor{
				Host: "codex", SessionID: "reminder-session",
				SessionProcess: &issueopscontract.NativeProcessReceipt{PID: 1, StartedAt: "2026-07-22T00:00:00Z", Executable: "/usr/bin/codex"},
			},
		},
	}
	writeIssueOpsRecordForReminderTest(t, record)
}

func seedStaleLegacyPoolEntry(t *testing.T, parentID, id string) {
	t.Helper()
	root := filepath.Join(os.Getenv("HARNESS_STATE_DIR"), strings.Join([]string{"work", "pool"}, ""))
	if err := os.MkdirAll(filepath.Join(root, id), 0o700); err != nil {
		t.Fatalf("mkdir legacy fixture: %v", err)
	}
	manifest := `{"id":"` + id + `","name":"legacy","parent_cycle_id":"` + parentID + `","status":"active"}`
	if err := os.WriteFile(filepath.Join(root, id+".json"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write legacy manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, id, "task-stale.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write legacy task: %v", err)
	}
}
