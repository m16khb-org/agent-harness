package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// 원격 이슈 반영은 provider 호출 전에 현재 holder identity를 검증해야 한다.
// 훅을 거치지 않는 CLI/MCP 직접 호출도 같은 core fence를 공유한다.
func TestReflectDevilsAdvocateFindingsRequiresCurrentHolderBeforeProviderCall(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	worktree := filepath.Join(repo+".worktrees", "2626-vertex-breaker-observability")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{
		Repo: repo, Branch: "2626-vertex-breaker-observability",
	})
	if err != nil {
		t.Fatal(err)
	}
	record.WorktreePath = worktree
	record.IssueURL = "https://gitlab.example.com/group/repo/-/issues/2626"
	record.DevilsAdvocateReview = &issueops.IssueOpsDevilsAdvocateReview{
		Verdict: "stop", Findings: []string{"breaker 상태 전이 관측 근거가 부족하다"},
		ReviewerPattern: "devils-advocate-review", RecordedAt: "2026-07-29T00:00:00Z",
	}
	record.Execution = issueOpsExecutionForTest(repo, worktree, record.Branch)
	if err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		_, writeErr := writeIssueOps(stateRoot, record)
		return writeErr
	}); err != nil {
		t.Fatal(err)
	}

	prov := &fakeCompletionProvider{
		updateRes: port.IssueProviderUpdateIssueBodySectionResult{OK: true, Preview: "[dry-run]"},
	}
	foreign := issueOpsActorForTest(worktree)
	foreign.SessionID = "other-session"
	if _, _, err := ReflectDevilsAdvocateFindingsWithActor(stateRoot, record.ID, false, prov, foreign); err == nil ||
		!strings.Contains(err.Error(), "current write lease holder") {
		t.Fatalf("비-holder 원격 반영은 core에서 차단돼야 한다: %v", err)
	}
	if prov.updateReq != nil {
		t.Fatalf("identity 검증 전에 provider가 호출됐다: %+v", prov.updateReq)
	}

	holder := issueOpsActorForTest(worktree)
	if _, result, err := ReflectDevilsAdvocateFindingsWithActor(stateRoot, record.ID, false, prov, holder); err != nil {
		t.Fatalf("현재 holder preview 실패: %v", err)
	} else if result.Preview == "" || prov.updateReq == nil || prov.updateReq.Confirm {
		t.Fatalf("현재 holder preview 요청이 잘못 전달됐다: result=%+v request=%+v", result, prov.updateReq)
	}

	prov.updateRes = port.IssueProviderUpdateIssueBodySectionResult{
		OK: true, Updated: true, URL: record.IssueURL,
	}
	if got, _, err := ReflectDevilsAdvocateFindingsWithActor(stateRoot, record.ID, true, prov, holder); err != nil {
		t.Fatalf("현재 holder confirm 실패: %v", err)
	} else if got.DevilsAdvocateReview == nil || got.DevilsAdvocateReview.IssueReflectedAt == "" {
		t.Fatalf("성공한 confirm이 reflected timestamp를 기록하지 않았다: %+v", got.DevilsAdvocateReview)
	}
}
