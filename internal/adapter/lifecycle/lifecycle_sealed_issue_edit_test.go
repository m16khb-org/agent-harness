package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"

	issueopscore "agent-harness/internal/adapter/issueops"
)

const sealedIssueEditURL = "https://github.com/example/repo/issues/77"

// sealedOrcaCycleRepo는 봉인이 끝난 활성 orca 사이클을 만든다. packet 파일을
// 실제로 두는 것이 중요하다 — 가드는 봉인이 일어난 뒤에만 발동해야 한다.
func sealedOrcaCycleRepo(t *testing.T, sealPacket bool) (string, string) {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(t.TempDir(), "77-sealed")
	if err := os.MkdirAll(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	record, err := StartIssueOps(IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "77-sealed"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = sealedIssueEditURL
	record.WorktreePath = worktree
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeOrca,
		Workspace: issueopscontract.Workspace{
			SourceRoot: repo, Root: worktree, Branch: "77-sealed",
			BaseHead: "0123456789abcdef0123456789abcdef01234567", Driver: "orca",
			LinkedAt: "2026-07-25T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusClaimable,
			ClaimTokenSHA256: strings.Repeat("a", 64),
		},
		Orca: &issueopscontract.OrcaBinding{
			RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "worktree-1",
			OwnerHost: "claude", OwnerModel: "model-1", TaskID: "task-1", DispatchID: "dispatch-1",
		},
	}
	if _, err := issueopscore.WriteIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	if sealPacket {
		packet := issueopscore.SealedOwnerContextPacketPath(record)
		if strings.TrimSpace(packet) == "" {
			t.Fatal("sealed packet path is unavailable")
		}
		if err := os.MkdirAll(filepath.Dir(packet), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(packet, []byte(`{"schema_version":1}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return repo, record.ID
}

func TestPreToolUseBlocksEditingASealedIssueBody(t *testing.T) {
	repo, id := sealedOrcaCycleRepo(t, true)

	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo:              repo,
		Tool:              "bash",
		Command:           `gh issue edit ` + sealedIssueEditURL + ` --body "봉인된 이슈의 본문을 개정하려는 시도입니다."`,
		EnforceVCSLinking: true,
	})

	if got.Decision != "block" {
		t.Fatalf("editing a sealed issue body must be blocked, got %+v", got)
	}
	if !strings.Contains(got.Reason, id) || !strings.Contains(got.Reason, "--reseed") {
		t.Fatalf("block reason must name the sealed lifecycle and the reseal command: %q", got.Reason)
	}
}

func TestPreToolUseBlocksSealedIssueEditByNumber(t *testing.T) {
	repo, _ := sealedOrcaCycleRepo(t, true)

	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo:              repo,
		Tool:              "bash",
		Command:           `gh issue edit 77 --body "봉인된 이슈의 본문을 번호로 개정하려는 시도입니다."`,
		EnforceVCSLinking: true,
	})

	if got.Decision != "block" {
		t.Fatalf("number-form sealed issue edit must be blocked, got %+v", got)
	}
}

func TestPreToolUseAllowsIssueEditsThatDoNotTouchSealedContext(t *testing.T) {
	sealedRepo, _ := sealedOrcaCycleRepo(t, true)
	unsealedRepo, _ := sealedOrcaCycleRepo(t, false)

	cases := []struct {
		name    string
		repo    string
		command string
	}{
		{
			name:    "unrelated issue",
			repo:    sealedRepo,
			command: `gh issue edit 999 --body "봉인과 무관한 다른 이슈의 본문을 고칩니다."`,
		},
		{
			name:    "cycle without a sealed packet",
			repo:    unsealedRepo,
			command: `gh issue edit ` + sealedIssueEditURL + ` --body "아직 봉인되지 않은 사이클의 이슈 본문을 고칩니다."`,
		},
		{
			name:    "edit target is not resolvable",
			repo:    sealedRepo,
			command: `gh issue edit --body "대상을 해석할 수 없는 편집 명령은 통과해야 합니다."`,
		},
		{
			name:    "issue create is not an edit",
			repo:    sealedRepo,
			command: `gh issue create --title "새 이슈" --body "봉인 보호는 생성 경로를 막지 않습니다." --label bug --assignee sample`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
				Repo: tc.repo, Tool: "bash", Command: tc.command, EnforceVCSLinking: true,
			})
			if got.Decision == "block" && strings.Contains(got.Reason, "--reseed") {
				t.Fatalf("sealed-issue guard must not fire here: %+v", got)
			}
		})
	}
}

func TestPreToolUseAllowsSealedIssueEditAfterTheCycleIsReleased(t *testing.T) {
	repo, id := sealedOrcaCycleRepo(t, true)
	record, err := issueopscore.ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Lease.Status = issueopscontract.LeaseStatusReleased
	record.Execution.Lease.ClaimTokenSHA256 = ""
	record.Execution.Lease.ReleasedAt = "2026-07-25T01:00:00Z"
	if _, err := issueopscore.WriteIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo:              repo,
		Tool:              "bash",
		Command:           `gh issue edit ` + sealedIssueEditURL + ` --body "완료된 사이클의 이슈 본문은 자유롭게 고칠 수 있습니다."`,
		EnforceVCSLinking: true,
	})

	if got.Decision == "block" && strings.Contains(got.Reason, "--reseed") {
		t.Fatalf("released cycle must not keep its issue sealed: %+v", got)
	}
}
