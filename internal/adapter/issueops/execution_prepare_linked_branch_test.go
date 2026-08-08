package issueops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

const linkedBranchName = "319-orca-linked-branch-order"

// TestOrcaPrepareAdoptsALinkedBranchStillAtItsSealedBase는 #319를 고정한다.
//
// documented flow는 `branch prepare --link-verified` → linked branch 생성 →
// `execution prepare --mode orca` 순서를 지시한다. 그 순서를 따르면 원격에
// 브랜치가 이미 있으므로 Orca prepare가 거부했다 — GitLab만 예외로 통과하고
// GitHub은 막혔다. 안전 근거는 provider 이름이 아니라 **원격 tip이 봉인된 base
// 그대로여서 잃을 작업이 없다**는 관측이다.
func TestOrcaPrepareAdoptsALinkedBranchStillAtItsSealedBase(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			repo := initLinkedBranchRepo(t)
			sealed := linkedBranchHead(t, repo)
			writeGitRef(t, repo, filepath.Join("refs", "remotes", "origin", linkedBranchName), sealed)
			record := linkedBranchRecord(repo, provider, sealed)

			if err := ensureOrcaBranchIsFree(record, linkedBranchName); err != nil {
				t.Fatalf("봉인된 base 그대로인 linked branch는 채택돼야 한다: %v", err)
			}
		})
	}
}

// TestOrcaPrepareStillBlocksBranchesThatHoldWork는 채택이 작업을 잃는 문을
// 열지 않음을 고정한다.
func TestOrcaPrepareStillBlocksBranchesThatHoldWork(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutate   func(*issueops.IssueOpsRecord)
		advanced bool
	}{
		{"원격 tip이 전진함", nil, true},
		{"link 미검증", func(r *issueops.IssueOpsRecord) { r.BranchPrepare.LinkVerified = false }, false},
		{"base SHA 미봉인", func(r *issueops.IssueOpsRecord) { r.BranchPrepare.BaseSHA = "" }, false},
		{"다른 브랜치 기록", func(r *issueops.IssueOpsRecord) { r.BranchPrepare.Branch = "other-branch" }, false},
		{"branch_prepare 없음", func(r *issueops.IssueOpsRecord) { r.BranchPrepare = nil }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := initLinkedBranchRepo(t)
			sealed := linkedBranchHead(t, repo)
			remoteOID := sealed
			if tc.advanced {
				remoteOID = linkedBranchExtraCommit(t, repo)
			}
			writeGitRef(t, repo, filepath.Join("refs", "remotes", "origin", linkedBranchName), remoteOID)
			record := linkedBranchRecord(repo, "github", sealed)
			if tc.mutate != nil {
				tc.mutate(&record)
			}
			err := ensureOrcaBranchIsFree(record, linkedBranchName)
			if err == nil {
				t.Fatal("작업을 잃을 수 있는 상태는 계속 차단돼야 한다")
			}
			if !strings.Contains(err.Error(), "already exists on origin") {
				t.Fatalf("차단 사유가 원격 충돌임을 밝혀야 한다: %v", err)
			}
		})
	}
}

// TestOrcaPrepareStillBlocksALocalBranch는 로컬 브랜치는 채택 대상이 아님을
// 고정한다. Orca는 로컬 브랜치를 만들므로 이름이 이미 로컬에 있으면 그대로
// 충돌한다.
func TestOrcaPrepareStillBlocksALocalBranch(t *testing.T) {
	repo := linkedBranchRepoWithLocalRef(t, linkedBranchName)
	record := linkedBranchRecord(repo, "github", linkedBranchHead(t, repo))

	err := ensureOrcaBranchIsFree(record, linkedBranchName)
	if err == nil || !strings.Contains(err.Error(), "already exists locally") {
		t.Fatalf("로컬 브랜치는 계속 차단돼야 한다: %v", err)
	}
}

func linkedBranchRecord(repo, provider, baseSHA string) issueops.IssueOpsRecord {
	return issueops.IssueOpsRecord{
		ID: "io-linked", Repo: repo, Branch: linkedBranchName,
		BranchPrepare: &issueops.IssueOpsBranchPrepare{
			Provider: provider, Branch: linkedBranchName, BaseBranch: "main",
			BaseSHA: baseSHA, LinkVerified: true,
		},
	}
}

func linkedBranchRepoWithLocalRef(t *testing.T, branch string) string {
	t.Helper()
	repo := initLinkedBranchRepo(t)
	writeGitRef(t, repo, filepath.Join("refs", "heads", branch), linkedBranchHead(t, repo))
	return repo
}

func initLinkedBranchRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runLinkedBranchGit(t, repo, "init", "--initial-branch=main")
	runLinkedBranchGit(t, repo, "config", "user.email", "test@example.com")
	runLinkedBranchGit(t, repo, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runLinkedBranchGit(t, repo, "add", "seed.txt")
	runLinkedBranchGit(t, repo, "commit", "-m", "seed")
	return repo
}

func linkedBranchExtraCommit(t *testing.T, repo string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "extra.txt"), []byte("extra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runLinkedBranchGit(t, repo, "add", "extra.txt")
	runLinkedBranchGit(t, repo, "commit", "-m", "extra")
	return linkedBranchHead(t, repo)
}

func linkedBranchHead(t *testing.T, repo string) string {
	t.Helper()
	return strings.TrimSpace(runLinkedBranchGit(t, repo, "rev-parse", "HEAD"))
}

func writeGitRef(t *testing.T, repo, relative, oid string) {
	t.Helper()
	path := filepath.Join(repo, ".git", relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(oid+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runLinkedBranchGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = repo
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
