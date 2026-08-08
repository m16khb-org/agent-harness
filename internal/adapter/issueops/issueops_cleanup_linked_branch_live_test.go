package issueops

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	linkedbranch "agent-harness/internal/domain/issueopslinkedbranch"
)

// TestCleanupLinkedBranchObservesTheLiveIssue는 #306의 dogfood 조건이다.
//
// fake runner는 내가 GitHub 응답이라고 *믿는 모양*을 고정할 뿐이다. 질의가
// 실제 스키마에 유효한지, 응답이 정말 그 모양으로 오는지, 좌표 추출이 실제
// URL에서 동작하는지는 실물에서만 확인된다.
//
// 이 테스트는 배포되는 관측 경로 그대로(LiveProviderCLI)를 쓴다. 삭제는
// 부르지 않는다 — 지울 대상이 없을 때 어떤 mutation도 시도하지 않는 것이
// 이 경로의 안전 근거이고, 그 사실 자체가 관측 대상이다.
//
// 기본은 skip이다. 실행:
//
//	HARNESS_GH_LIVE=1 go test ./internal/adapter/issueops -run LiveIssue -count=1 -v
func TestCleanupLinkedBranchObservesTheLiveIssue(t *testing.T) {
	if os.Getenv("HARNESS_GH_LIVE") != "1" {
		t.Skip("실물 GitHub이 필요하다: HARNESS_GH_LIVE=1로 실행한다")
	}
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("이 호스트에 gh CLI가 없다")
	}
	const issueURL = "https://github.com/m16khb/agent-harness/issues/304"

	observation, err := ObserveGitHubLinkedBranches(LiveProviderCLI)(context.Background(), issueURL)
	if err != nil {
		t.Fatalf("실제 GraphQL 질의가 유효해야 한다: %v", err)
	}
	if observation.TotalCount != len(observation.Nodes) {
		t.Fatalf("응답의 totalCount와 노드 수가 어긋난다: %#v", observation)
	}
	t.Logf("live observation: issue=%s totalCount=%d nodes=%d", issueURL, observation.TotalCount, len(observation.Nodes))

	// 관측을 그대로 분류기에 넣어 전체 경로가 이어지는지 본다. #304의 고아는
	// 이미 사라졌으므로(totalCount=0) absent가 기대값이고, 그것은 "정리할 것이
	// 없다"는 멱등 성공이다.
	observation.RequestedBranch = "304-completion-reseed-stale-receipt"
	observation.SealedBase = "5480568a4178d5ea46d5486b97d0ff5223f1c24c"
	state, target, reason := linkedbranch.Classify(observation)
	t.Logf("classified: state=%s target=%q reason=%q", state, target.ID, reason)
	if state == linkedbranch.StateOrphan {
		// 고아가 실제로 남아 있다면 그것은 이 이슈의 원래 정리 대상이다.
		// 테스트가 지우지는 않는다 — 삭제는 preview→apply+confirm 경로의 일이다.
		t.Logf("고아가 남아 있다. typed 경로로 정리할 것: node=%s", target.ID)
	}
}

// TestCleanupLinkedBranchLiveGraphQLRejectsABadSelector는 좌표 검증이 실물
// 호출 **이전에** 막는지 고정한다. 잘못된 좌표로 남의 이슈를 읽는 일이
// 없어야 하고, 그 방어는 네트워크에 닿기 전에 끝나야 한다.
func TestCleanupLinkedBranchLiveGraphQLRejectsABadSelector(t *testing.T) {
	invoked := false
	observe := ObserveGitHubLinkedBranches(func(context.Context, string, ...string) (string, error) {
		invoked = true
		return "", nil
	})
	if _, err := observe(context.Background(), "https://github.com/m16khb/agent-harness/pull/304"); err == nil {
		t.Fatal("이슈 경로가 아니면 거부해야 한다")
	}
	if invoked {
		t.Fatal("좌표가 어긋나면 provider를 부르기 전에 멈춰야 한다")
	}
}

// TestCleanupLinkedBranchGateBlocksBeforeAnyProviderCall는 게이트가 외부
// 호출 이전임을 한 번 더 고정한다. 준비되지 않은 record로 GitHub을 읽으면
// 안 된다.
func TestCleanupLinkedBranchGateBlocksBeforeAnyProviderCall(t *testing.T) {
	root := t.TempDir()
	record := issueopscontract.IssueOpsRecord{
		OK: true, SchemaVersion: 1, ID: "io-lbgate", Repo: t.TempDir(), Phase: "execution",
		CreatedAt: "2026-08-09T00:00:00Z", UpdatedAt: "2026-08-09T00:00:00Z",
	}
	if _, err := WriteIssueOps(root, record); err != nil {
		t.Fatal(err)
	}
	invoked := false
	_, err := CleanupLinkedBranch(context.Background(), root,
		issueopscontract.CleanupLinkedBranchRequest{ID: "io-lbgate"},
		CleanupLinkedBranchDeps{
			ObserveLinkedBranches: func(context.Context, string) (linkedbranch.Observation, error) {
				invoked = true
				return linkedbranch.Observation{}, nil
			},
			DeleteLinkedBranch: func(context.Context, string, string) error { return nil },
		})
	if err == nil || !strings.Contains(err.Error(), "branch_prepare_missing") {
		t.Fatalf("err=%v", err)
	}
	if invoked {
		t.Fatal("게이트에서 막힌 요청은 외부를 관측하지 않는다")
	}
}
