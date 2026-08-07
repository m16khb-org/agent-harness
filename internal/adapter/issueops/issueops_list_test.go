package issueops

import (
	"context"
	"path/filepath"
	"testing"

	"agent-harness/internal/contract/issueops"
)

// AC-06: list가 다중 사이클을 span lock 없이 집계하고 비용을 노출한다.
func TestListIssueOpsCyclesAggregatesWithCost(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	otherRepo := t.TempDir()

	planning, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "84-planning"})
	if err != nil {
		t.Fatal(err)
	}
	claimable, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "84-claimable"})
	if err != nil {
		t.Fatal(err)
	}
	done, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "84-done"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: otherRepo, Branch: "84-elsewhere"}); err != nil {
		t.Fatal(err)
	}

	worktree := filepath.Join(t.TempDir(), "84-claimable")
	mutate := func(id string, apply func(*issueops.IssueOpsRecord)) {
		t.Helper()
		if err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
			rec, e := ReadIssueOps(stateRoot, id)
			if e != nil {
				return e
			}
			apply(&rec)
			_, e = writeIssueOps(stateRoot, rec)
			return e
		}); err != nil {
			t.Fatal(err)
		}
	}
	mutate(claimable.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = &issueops.Execution{
			Mode:      issueops.ExecutionModeOrca,
			Workspace: issueops.Workspace{SourceRoot: repo, Root: worktree, Branch: "84-claimable", BaseHead: "deadbeef", Driver: "orca", LinkedAt: "t"},
			Lease:     issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusClaimable, ClaimTokenSHA256: sha64ForListTest()},
			Orca: &issueops.OrcaBinding{
				RuntimeID: "rt", RepoID: "r", WorktreeID: "wt", OwnerHost: "codex",
				OwnerModel: "gpt-5.6-terra", TaskID: "task", DispatchID: "d",
			},
		}
	})
	mutate(done.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Phase = issueops.IssueOpsPhaseDone
		rec.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/1"}
	})

	result, err := ListIssueOpsCycles(stateRoot, repo)
	if err != nil || !result.OK {
		t.Fatalf("list must aggregate: %v %+v", err, result)
	}
	if result.ScannedRecords < 4 || result.GeneratedAt == "" {
		t.Fatalf("list must expose its scan cost: %+v", result)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("repo filter must keep 3 cycles: %+v", result.Entries)
	}
	byID := map[string]IssueOpsListEntry{}
	for _, entry := range result.Entries {
		byID[entry.ID] = entry
	}
	if entry := byID[planning.ID]; entry.Claimable || entry.CleanupCandidate {
		t.Fatalf("planning cycle must carry no flags: %+v", entry)
	}
	if entry := byID[claimable.ID]; !entry.Claimable || entry.OwnerModel != "gpt-5.6-terra" || entry.LeaseStatus != "claimable" {
		t.Fatalf("claimable cycle must surface owner model and lease: %+v", entry)
	}
	if entry := byID[done.ID]; !entry.CleanupCandidate || !entry.CompletionUnreflected {
		t.Fatalf("done cycle must surface cleanup and unreflected flags: %+v", entry)
	}
}

func TestIncrementIssueOpsSourceMisdirectAccumulates(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "84-misdirect"})
	if err != nil {
		t.Fatal(err)
	}
	for want := 1; want <= 2; want++ {
		count, err := IncrementIssueOpsSourceMisdirect(stateRoot, record.ID)
		if err != nil || count != want {
			t.Fatalf("counter must accumulate to %d: %v %d", want, err, count)
		}
	}
	ready := IssueOpsStrictPRReadinessWithState(stateRoot, mustReadForListTest(t, stateRoot, record.ID))
	found := false
	for _, warning := range ready.Warnings {
		if warning == "source_misdirect_warnings:2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("strict readiness must surface the misdirect warning key: %+v", ready.Warnings)
	}
}

func sha64ForListTest() string {
	out := make([]byte, 0, 64)
	for i := 0; i < 64; i++ {
		out = append(out, 'a')
	}
	return string(out)
}

func mustReadForListTest(t *testing.T, stateRoot, id string) issueops.IssueOpsRecord {
	t.Helper()
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	return record
}
