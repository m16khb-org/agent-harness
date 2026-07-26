package branchprepare

import (
	"strings"
	"testing"
)

// `gh issue develop --base <branch>`는 GitHub이 **그 시점** 브랜치 HEAD를 조회해
// `oid`로 쓴다. orca는 봉인된 base SHA에서 로컬 브랜치를 만들므로, 그 사이 base
// 브랜치가 진행하면 두 base가 갈리고 push가 non-fast-forward로 거부된다. 봉인
// 가드가 merge를, 안전 훅이 force push를, `sync-base`가 completion 이전 실행을
// 막으므로 발행 경로가 사라진다(이슈 #176, #147에서 실측).
//
// `CreateLinkedBranchInput.oid`는 필수 필드다(GraphQL 인트로스펙션 확인).
// `gh issue develop`이 그것을 숨기고 브랜치 HEAD로 채우는 것뿐이므로, 봉인 SHA를
// 직접 넘기면 갈림이 원리적으로 생기지 않는다 — 임의 SHA로 링크 브랜치가
// 만들어지는 것을 실측했다.
func TestGitHubStepsPinTheSealedBaseSHA(t *testing.T) {
	const baseSHA = "2a56f2cc4d2e6b7b4fa99e3cdd71e3673ae060d2"
	steps := Steps("github", "https://github.com/acme/repo/issues/16", "16-demo", "main", baseSHA)
	joined := ""
	for _, step := range steps {
		joined += step.Description + " " + strings.Join(step.Command, " ") + "\n"
	}
	if !strings.Contains(joined, baseSHA) {
		t.Fatalf("봉인 base SHA를 안내에 담지 않으면 GitHub이 그 시점 HEAD를 쓴다:\n%s", joined)
	}
	if !strings.Contains(joined, "createLinkedBranch") {
		t.Fatalf("oid를 넘길 수 있는 것은 GraphQL mutation뿐이다. gh issue develop --base는 브랜치 이름만 받는다:\n%s", joined)
	}
}

// node ID 조회가 별도 단계여야 한다. mutation 하나에 셸 치환을 넣으면 가드가
// 정적으로 분류할 수 없고, 이 저장소는 그런 명령을 거부한다.
func TestGitHubStepsResolveTheIssueNodeIDSeparately(t *testing.T) {
	steps := Steps("github", "https://github.com/acme/repo/issues/16", "16-demo", "main", "deadbeef")
	var nodeIDStep, mutationStep bool
	for _, step := range steps {
		command := strings.Join(step.Command, " ")
		if strings.Contains(command, "node_id") {
			nodeIDStep = true
		}
		if strings.Contains(command, "createLinkedBranch") {
			mutationStep = true
			if strings.Contains(command, "$(") || strings.Contains(command, "`") {
				t.Fatalf("셸 치환이 든 명령은 가드가 분류하지 못한다: %q", command)
			}
		}
	}
	if !nodeIDStep || !mutationStep {
		t.Fatalf("node ID 조회와 mutation이 각자 단계여야 한다: nodeID=%t mutation=%t", nodeIDStep, mutationStep)
	}
}

// base SHA가 없으면 oid를 고정할 수 없다. 그 경우 종전 경로로 떨어지되 왜
// 그런지 안내해야 한다 — 조용히 브랜치 이름을 쓰면 #147의 갈림이 재발한다.
func TestGitHubStepsFallBackWhenTheBaseSHAIsAbsent(t *testing.T) {
	steps := Steps("github", "https://github.com/acme/repo/issues/16", "16-demo", "main", "")
	joined := ""
	for _, step := range steps {
		joined += step.Description + " " + strings.Join(step.Command, " ") + "\n"
	}
	if strings.Contains(joined, "createLinkedBranch") {
		t.Fatalf("base SHA가 없으면 oid를 고정할 수 없다:\n%s", joined)
	}
	if !strings.Contains(joined, "gh issue develop") {
		t.Fatalf("종전 경로로 떨어져야 한다:\n%s", joined)
	}
	if !strings.Contains(joined, "base_sha") {
		t.Fatalf("왜 oid를 고정하지 못하는지 안내해야 한다:\n%s", joined)
	}
}

// GraphQL 변수는 셸에서 단일 인용해야 한다. 인용하지 않으면 가드의 파라미터
// 확장 검사가 명령을 거부한다(tokens.go의 `$` 검사, 단일 인용 안은 건너뛴다).
// 그 전제를 안내에 담지 않으면 실행 주체가 그대로 붙여넣어 막힌다.
func TestGitHubStepsWarnAboutQuotingTheQuery(t *testing.T) {
	steps := Steps("github", "https://github.com/acme/repo/issues/16", "16-demo", "main", "deadbeef")
	joined := ""
	for _, step := range steps {
		joined += step.Description + "\n"
	}
	if !strings.Contains(joined, "quote") {
		t.Fatalf("query 인용 전제를 안내해야 한다:\n%s", joined)
	}
}

// GitLab은 일반 브랜치 API를 쓴다. base 못박기는 `#180`이 같은 계약으로 맞췄지만
// 수단이 다르다 — GraphQL mutation도 node id 조회도 쓰지 않고 `ref`에 SHA를 넘긴다.
// 그 경로를 검증하는 것은 branch_prepare_gitlab_contract_test.go다.
func TestGitLabStepsAreUntouchedByTheOIDChange(t *testing.T) {
	steps := Steps("gitlab", "https://gitlab.example.com/acme/repo/-/issues/16", "16-demo", "main", "deadbeef")
	for _, step := range steps {
		command := strings.Join(step.Command, " ")
		if strings.Contains(command, "createLinkedBranch") || strings.Contains(command, "node_id") {
			t.Fatalf("gitlab 경로는 이 이슈의 대상이 아니다: %q", command)
		}
	}
}
