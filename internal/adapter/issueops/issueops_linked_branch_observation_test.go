package issueops

import (
	"context"
	"strings"
	"testing"
)

// TestObserveGitHubLinkedBranchesReadsANullRefAsAnOrphanShape는 provider
// 응답 해석을 고정한다. `ref:null`은 이름도 OID도 없는 노드로 읽혀야 하고,
// 그 부재가 곧 분류기가 고아를 알아보는 표식이다.
func TestObserveGitHubLinkedBranchesReadsANullRefAsAnOrphanShape(t *testing.T) {
	var seen []string
	observe := ObserveGitHubLinkedBranches(func(_ context.Context, name string, args ...string) (string, error) {
		seen = append([]string{name}, args...)
		return `{"data":{"repository":{"issue":{"linkedBranches":{"totalCount":2,"nodes":[
			{"id":"LB_orphan","ref":null},
			{"id":"LB_live","ref":{"name":"304-branch","target":{"oid":"abc123"}}}]}}}}}`, nil
	})

	observation, err := observe(context.Background(), "https://github.com/m16khb/agent-harness/issues/304")
	if err != nil {
		t.Fatal(err)
	}
	if observation.TotalCount != 2 || len(observation.Nodes) != 2 {
		t.Fatalf("observation=%#v", observation)
	}
	if !observation.Nodes[0].RefNull() || observation.Nodes[0].ID != "LB_orphan" {
		t.Fatalf("null ref는 이름 없는 노드로 읽혀야 한다: %#v", observation.Nodes[0])
	}
	if observation.Nodes[1].RefName != "304-branch" || observation.Nodes[1].RefOID != "abc123" {
		t.Fatalf("살아 있는 링크는 이름과 OID를 그대로 실어야 한다: %#v", observation.Nodes[1])
	}
	// 좌표를 문자열 보간이 아니라 별도 필드로 넘기는지 고정한다.
	joined := strings.Join(seen, " ")
	for _, needle := range []string{"owner=m16khb", "repo=agent-harness", "number=304"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("질의가 %q를 인자로 실어야 한다: %s", needle, joined)
		}
	}
}

// TestGitHubIssueSelectorRefusesAnythingButAnIssuePath는 좌표 추출이 추측하지
// 않음을 고정한다. 잘못된 좌표는 남의 이슈를 읽거나 지우는 경로가 된다.
func TestGitHubIssueSelectorRefusesAnythingButAnIssuePath(t *testing.T) {
	for _, url := range []string{
		"", "https://github.com/m16khb/agent-harness",
		"https://github.com/m16khb/agent-harness/pull/304",
		"https://github.com/m16khb/agent-harness/issues/not-a-number",
		"https://github.com/m16khb/agent-harness/issues/",
	} {
		if _, _, _, err := githubIssueSelector(url); err == nil {
			t.Fatalf("이슈 경로가 아닌 %q를 받아들이면 안 된다", url)
		}
	}
	owner, repo, number, err := githubIssueSelector("https://github.com/m16khb/agent-harness/issues/304/")
	if err != nil || owner != "m16khb" || repo != "agent-harness" || number != "304" {
		t.Fatalf("%s/%s#%s err=%v", owner, repo, number, err)
	}
}

// TestDeleteGitHubLinkedBranchTakesOnlyANodeID는 삭제 표면이 이름을 받지
// 않음을 고정한다. 이름으로 지울 수 있으면 ref 있는 링크도 지울 수 있게 된다.
func TestDeleteGitHubLinkedBranchTakesOnlyANodeID(t *testing.T) {
	var argv []string
	del := DeleteGitHubLinkedBranch(func(_ context.Context, name string, args ...string) (string, error) {
		argv = append([]string{name}, args...)
		return "{}", nil
	})
	if err := del(context.Background(), "https://github.com/m16khb/agent-harness/issues/304", "LB_orphan"); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "id=LB_orphan") || !strings.Contains(joined, "deleteLinkedBranch") {
		t.Fatalf("argv=%s", joined)
	}
	if strings.Contains(joined, "304-") {
		t.Fatalf("삭제 호출에 브랜치 이름이 실리면 안 된다: %s", joined)
	}

	if err := del(context.Background(), "https://github.com/m16khb/agent-harness/issues/304", "  "); err == nil {
		t.Fatal("빈 노드 id는 거부해야 한다")
	}
}
