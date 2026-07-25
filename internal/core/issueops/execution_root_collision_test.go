package issueops

import (
	"context"
	"os"
	"strings"
	"testing"

	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/core/sqlstore"
)

// AC: leaf 인코딩이 비단사라 "72/fix"와 "72-fix"가 같은 canonical worktree로
// 접히지만 lifecycle ID는 브랜치로 해시되어 서로 다른 레코드가 된다. prepare는
// 두 번째 사이클이 이미 선점된 root를 집으려는 순간 preview에서부터 거부하고
// 선점 lifecycle ID·브랜치·다음 행동을 그대로 에코해야 한다.
func executionRootCollisionFixture(t *testing.T) (string, string, string) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	if baseHead == "" {
		t.Fatal("fixture repo must expose a base HEAD")
	}
	return stateRoot, repo, baseHead
}

func executionRootCollisionRecord(t *testing.T, stateRoot, repo, baseHead, branch string) IssueOpsRecord {
	t.Helper()
	issueURL := "https://github.com/acme/repo/issues/72"
	record := IssueOpsRecord{
		OK: true, SchemaVersion: IssueOpsCurrentSchemaVersion,
		ID: NewIssueOpsID(repo, branch), Repo: repo, Branch: branch, Phase: IssueOpsPhasePlan,
		IssueURL:     issueURL,
		DesignReview: &IssueOpsDesignReview{Approved: true, ReviewedAt: "2026-07-24T00:00:00Z"},
		BranchPrepare: &IssueOpsBranchPrepare{
			Provider: "github", IssueURL: issueURL, Branch: branch,
			BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true, CreatedAt: "2026-07-24T00:00:00Z",
		},
		CreatedAt: "2026-07-24T00:00:00Z", UpdatedAt: "2026-07-24T00:00:00Z",
	}
	written, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return written
}

// executionRootClaimingRecord는 branch에서 파생된 canonical root를 소유하는
// 레코드를 영속한다 — withExecution=false는 linking만 끝난(Execution 부재)
// 레코드도 root를 점유한다는 사실을 고정한다.
func executionRootClaimingRecord(t *testing.T, stateRoot, repo, baseHead, branch string, withExecution bool) (IssueOpsRecord, string) {
	t.Helper()
	record := executionRootCollisionRecord(t, stateRoot, repo, baseHead, branch)
	root := issueOpsWorktreePathForTest(repo, strings.ReplaceAll(branch, "/", "-"))
	record.WorktreePath = root
	if withExecution {
		record.Execution = issueOpsExecutionForTest(repo, root, branch)
	}
	written, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return written, root
}

func TestPrepareExecutionRejectsCanonicalRootClaimedByAnotherLifecycle(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		name := "preview"
		if confirm {
			name = "confirm"
		}
		t.Run(name, func(t *testing.T) {
			stateRoot, repo, baseHead := executionRootCollisionFixture(t)
			claimed, root := executionRootClaimingRecord(t, stateRoot, repo, baseHead, "72/fix", true)
			other := executionRootCollisionRecord(t, stateRoot, repo, baseHead, "72-fix")
			if other.ID == claimed.ID {
				t.Fatalf("branches %q and %q must map to distinct lifecycle ids", claimed.Branch, other.Branch)
			}
			_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
				ID: other.ID, Mode: "direct", CWD: repo, Confirm: confirm,
				Actor: executionActor("codex", "root-collision-session"),
			}, ExecutionPrepareDependencies{Direct: gitworktree.New()})
			if err == nil {
				t.Fatalf("prepare must reject canonical root %q already claimed by %s", root, claimed.ID)
			}
			for _, want := range []string{
				claimed.ID, claimed.Branch, root,
				"agent-harness issueops cleanup finish --id " + claimed.ID,
			} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("collision error must echo %q: %v", want, err)
				}
			}
			persisted, readErr := ReadIssueOps(stateRoot, other.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if persisted.Execution != nil || persisted.WorktreePath != "" {
				t.Fatalf("rejected prepare must not persist execution state: %#v", persisted)
			}
			if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
				t.Fatalf("rejected prepare must not create the contested worktree: %v", statErr)
			}
		})
	}
}

func TestPrepareExecutionRejectsCanonicalRootClaimedByLinkOnlyRecord(t *testing.T) {
	stateRoot, repo, baseHead := executionRootCollisionFixture(t)
	linked, root := executionRootClaimingRecord(t, stateRoot, repo, baseHead, "72/fix", false)
	if linked.Execution != nil {
		t.Fatalf("fixture must stay execution-free: %#v", linked.Execution)
	}
	other := executionRootCollisionRecord(t, stateRoot, repo, baseHead, "72-fix")
	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: other.ID, Mode: "direct", CWD: repo,
		Actor: executionActor("codex", "link-only-session"),
	}, ExecutionPrepareDependencies{Direct: gitworktree.New()})
	if err == nil || !strings.Contains(err.Error(), linked.ID) || !strings.Contains(err.Error(), root) {
		t.Fatalf("a link-only record must still own its canonical root: %v", err)
	}
}

func TestPrepareExecutionKeepsDistinctRootsAndSelfRepreparation(t *testing.T) {
	stateRoot, repo, baseHead := executionRootCollisionFixture(t)
	claimed, root := executionRootClaimingRecord(t, stateRoot, repo, baseHead, "72/fix", true)

	free := executionRootCollisionRecord(t, stateRoot, repo, baseHead, "73-free")
	got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: free.ID, Mode: "direct", CWD: repo,
		Actor: executionActor("codex", "free-root-session"),
	}, ExecutionPrepareDependencies{Direct: gitworktree.New()})
	if err != nil {
		t.Fatalf("a non-colliding canonical root must still preview: %v", err)
	}
	if got.Workspace.Root != issueOpsWorktreePathForTest(repo, "73-free") {
		t.Fatalf("unexpected canonical root: %#v", got.Workspace)
	}

	again, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: claimed.ID, Mode: "direct", CWD: repo,
		Actor: executionActor("codex", "self-reprepare-session"),
	}, ExecutionPrepareDependencies{Direct: gitworktree.New()})
	if err != nil {
		t.Fatalf("self re-prepare must stay idempotent: %v", err)
	}
	if !again.OK || again.Execution == nil || again.Workspace.Root != root {
		t.Fatalf("self re-prepare must return the persisted execution: %#v", again)
	}
}

func TestEnsureExecutionRootUnclaimedFailsClosedOnUnreadableRecord(t *testing.T) {
	stateRoot, repo, baseHead := executionRootCollisionFixture(t)
	_ = executionRootCollisionRecord(t, stateRoot, repo, baseHead, "74-guard")
	freeRoot := issueOpsWorktreePathForTest(repo, "74-free")
	if err := ensureExecutionRootUnclaimed(stateRoot, "io-dddddddddddd", freeRoot); err != nil {
		t.Fatalf("a clean state root must not deny a free canonical root: %v", err)
	}

	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, "io-cccccccccccc", []byte(`{"schema_version":9,"id":"io-cccccccccccc"}`)); err != nil {
		t.Fatal(err)
	}
	err = ensureExecutionRootUnclaimed(stateRoot, "io-dddddddddddd", freeRoot)
	if err == nil || !strings.Contains(err.Error(), "io-cccccccccccc") {
		t.Fatalf("an unreadable record must fail closed and name the record: %v", err)
	}
}
