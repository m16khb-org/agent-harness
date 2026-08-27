package issueops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	issueops "agent-harness/internal/contract/issueops"
)

// TestCleanupAbandonClearsAsymmetricResidue는 #433을 고정한다.
//
// 예전에는 worktree와 branch 중 한쪽만 남으면 abandon이 거부했다. 그 비대칭
// 자체가 거부 사유였고, abandon 자신이 남긴 retry receipt가 있을 때만 예외가
// 열렸다. 그래서 worktree가 **다른 경로로** 사라진 lifecycle에는 typed 출구가
// 없었다 — cleanup finish는 머지 증거를 요구하고 cleanup orphan은 worktree를
// 요구하므로 둘 다 해당되지 않았다.
//
// 실측: io-268bd6ac6e7a는 worktree 부재 + branch 잔존 상태였고, 다른 모든
// 게이트를 통과했는데도 local_residue_pair 하나로 영구히 막혔다.
//
// 각 축의 안전 근거는 따로 있다 — worktree는 canonical·clean·head, branch는
// 다른 곳에 체크아웃되지 않았는지. 비대칭은 그 근거를 약화시키지 않는다.
func TestCleanupAbandonClearsAsymmetricResidue(t *testing.T) {
	t.Run("branch only", func(t *testing.T) {
		stateRoot, record := abandonTestRecord(t)
		absent := filepath.Join(t.TempDir(), "reclaimed-worktree")
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.Execution = abandonExecution(rec.Repo, absent, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
			rec.WorktreePath = absent
		})
		result, err := CleanupAbandon(context.Background(), stateRoot,
			abandonRequest(record.ID, false, "worktree was reclaimed elsewhere"),
			abandonDeps(&fakeAbandonGit{branchOID: "abc123"}, authoritativeZeroOrca()))
		if err != nil {
			t.Fatalf("branch만 남은 잔여물도 정리 가능해야 한다: %v missing=%v", err, result.Missing)
		}
		if result.Fingerprint == "" {
			t.Fatal("준비된 preview는 fingerprint를 발급해야 한다")
		}
		if result.WorktreePresent || !result.BranchPresent {
			t.Fatalf("관측이 결과에 그대로 남아야 한다: %#v", result)
		}
	})

	t.Run("worktree only", func(t *testing.T) {
		stateRoot, record := abandonTestRecord(t)
		root := filepath.Join(t.TempDir(), "canonical-worktree")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.Execution = abandonExecution(rec.Repo, root, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
			rec.WorktreePath = root
		})
		git := &asymmetricAbandonGit{root: root, branch: record.Branch, head: "abc123"}
		result, err := CleanupAbandon(context.Background(), stateRoot,
			abandonRequest(record.ID, false, "branch ref was removed elsewhere"),
			CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: git.run, Orca: authoritativeZeroOrca()})
		if err != nil {
			t.Fatalf("worktree만 남은 잔여물도 정리 가능해야 한다: %v missing=%v", err, result.Missing)
		}
		if !result.WorktreePresent || result.BranchPresent {
			t.Fatalf("관측이 결과에 그대로 남아야 한다: %#v", result)
		}
	})
}

// TestCleanupAbandonStillGatesEachAxisOnItsOwnEvidence는 완화가 축별 안전
// 근거까지 열지 않음을 고정한다. 비대칭을 허용하는 것과 검사를 빼는 것은
// 다르다.
func TestCleanupAbandonStillGatesEachAxisOnItsOwnEvidence(t *testing.T) {
	t.Run("worktree is not canonical", func(t *testing.T) {
		stateRoot, record := abandonTestRecord(t)
		root := filepath.Join(t.TempDir(), "live-worktree")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.Execution = abandonExecution(rec.Repo, root, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
		})
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
			abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca()))
		if err == nil || !containsString(result.Missing, "worktree_canonical") {
			t.Fatalf("registry가 인정하지 않는 worktree는 계속 막아야 한다: %v %v", err, result.Missing)
		}
	})

	t.Run("both present with divergent head", func(t *testing.T) {
		stateRoot, record := abandonTestRecord(t)
		root := filepath.Join(t.TempDir(), "canonical-worktree")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.Execution = abandonExecution(rec.Repo, root, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
			rec.WorktreePath = root
		})
		git := &asymmetricAbandonGit{root: root, branch: record.Branch, head: "aaa", branchOID: "bbb"}
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
			CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: git.run, Orca: authoritativeZeroOrca()})
		if err == nil || !containsString(result.Missing, "local_branch_head") {
			t.Fatalf("쌍이 있는데 head가 갈리면 계속 막아야 한다: %v %v", err, result.Missing)
		}
	})
}

// asymmetricAbandonGit는 worktree 축까지 관측 가능한 fake다. 기본 fake는
// branch rev-parse만 답하므로 worktree 쪽 게이트를 통과시킬 수 없다.
type asymmetricAbandonGit struct {
	root      string
	branch    string
	head      string
	branchOID string
}

func (g *asymmetricAbandonGit) run(dir string, args ...string) (int, string) {
	switch {
	case len(args) >= 2 && args[0] == "rev-parse" && args[1] == "--verify":
		if g.branchOID == "" {
			return 1, ""
		}
		return 0, g.branchOID
	case len(args) >= 2 && args[0] == "worktree" && args[1] == "list":
		return 0, "worktree " + g.root + "\nbranch refs/heads/" + g.branch + "\n"
	case len(args) >= 2 && args[0] == "rev-parse" && args[1] == "--show-toplevel":
		return 0, g.root
	case len(args) >= 1 && args[0] == "symbolic-ref":
		return 0, g.branch
	case len(args) >= 2 && args[0] == "rev-parse" && args[1] == "HEAD":
		return 0, g.head
	case len(args) >= 1 && args[0] == "status":
		return 0, ""
	}
	return 0, ""
}
