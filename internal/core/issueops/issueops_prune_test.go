package issueops

import (
	"path/filepath"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/model"
)

func writePrunableIssueOpsRecord(t *testing.T, stateRoot, id, phase, updatedAt string, lease *model.WriteLease) {
	t.Helper()
	record := IssueOpsRecord{
		OK: true, SchemaVersion: 1, ID: id,
		Repo: "/repo", Branch: "1-demo", Phase: model.IssueOpsPhase(phase),
		UpdatedAt: updatedAt,
	}
	if lease != nil {
		record.Execution = &model.Execution{
			Mode: model.ExecutionModeDirect,
			Workspace: model.Workspace{
				SourceRoot: "/repo", Root: "/repo.worktrees/1-demo", Branch: "1-demo",
				BaseHead: "0123456789abcdef0123456789abcdef01234567", Driver: "git",
				LinkedAt: "2026-07-01T00:00:00Z",
			},
			Lease: *lease,
		}
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatalf("write %s: %v", id, err)
	}
}

func TestPruneIssueOpsRemovesOnlyOldReleasedDoneCycles(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	old := time.Now().UTC().Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano)
	recent := time.Now().UTC().Format(time.RFC3339Nano)
	released := &model.WriteLease{Generation: 1, Status: model.LeaseStatusReleased}
	active := &model.WriteLease{
		Generation: 1, Status: model.LeaseStatusActive,
		ClaimedAt: "2026-07-01T00:00:00Z",
		Holder: &model.NativeActor{
			Host: "codex", SessionID: "session",
			SessionProcess: &model.NativeProcessReceipt{PID: 123, StartedAt: "2026-07-01T00:00:00Z", Executable: "/opt/codex"},
		},
	}

	writePrunableIssueOpsRecord(t, stateRoot, "io-aaaaaaaaaa01", "done", old, released)
	writePrunableIssueOpsRecord(t, stateRoot, "io-aaaaaaaaaa02", "done", recent, released)
	writePrunableIssueOpsRecord(t, stateRoot, "io-aaaaaaaaaa03", "implement", old, active)
	writePrunableIssueOpsRecord(t, stateRoot, "io-aaaaaaaaaa04", "done", old, active)

	preview, err := PruneIssueOps(stateRoot, 30*24*time.Hour, false)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !preview.DryRun || len(preview.Pruned) != 1 || preview.Pruned[0] != "io-aaaaaaaaaa01" {
		t.Fatalf("preview must select exactly the old released done cycle: %+v", preview)
	}
	if ids, _ := ListIssueOpsIDs(stateRoot); len(ids) != 4 {
		t.Fatalf("preview must not delete records: %v", ids)
	}

	result, err := PruneIssueOps(stateRoot, 30*24*time.Hour, true)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if result.DryRun || len(result.Pruned) != 1 {
		t.Fatalf("confirm must prune exactly one cycle: %+v", result)
	}
	ids, err := ListIssueOpsIDs(stateRoot)
	if err != nil || len(ids) != 3 {
		t.Fatalf("pruned store must keep 3 records: %v err=%v", ids, err)
	}
	for _, id := range ids {
		if id == "io-aaaaaaaaaa01" {
			t.Fatal("old released done cycle must be deleted")
		}
	}
}

func TestPruneIssueOpsRejectsNonPositiveMaxAge(t *testing.T) {
	if _, err := PruneIssueOps(filepath.Join(t.TempDir(), "issueops_v1"), 0, false); err == nil {
		t.Fatal("max age must be positive")
	}
}
