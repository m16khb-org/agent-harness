package branchprepare

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

// GitHub `createLinkedBranch`(= `gh issue develop`)는 `oid`에서 **새 브랜치를**
// 만든다. 이름이 원격에 이미 있으면 실패하지만(#159 실측:
// `API returned empty branch name`) **로컬에만 있으면 성공한다** — #159가 놓친
// 조건이고 #163에서 실측했다.
//
// Orca `worktree create`는 로컬 워크트리와 로컬 브랜치만 만들고 push하지 않는다.
// 따라서 Orca 모드는 순서를 뒤집으면 linked branch 추적을 잃지 않는다:
// prepare 기록 → Orca가 로컬 브랜치 생성 → `gh issue develop`으로 원격 생성·연결.
//
// 그 순서를 안내에 담지 않으면 운영자는 정식 순서(`gh issue develop` 먼저)를 따르고
// Orca가 이름 충돌로 막힌다(#149·#152·#154).
func TestGitHubStepsExplainTheOrcaOrdering(t *testing.T) {
	steps := Steps("github", "https://github.com/acme/repo/issues/16", "16-demo", "main", "deadbeef")
	if len(steps) == 0 {
		t.Fatal("github steps must exist")
	}
	joined := ""
	for _, step := range steps {
		joined += step.Description + " " + strings.Join(step.Command, " ") + "\n"
	}
	for _, want := range []string{"Orca", "local"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("github 안내가 %q를 설명해야 한다. Orca 모드에서 순서가 다르다는 것을 모르면 이름 충돌로 막힌다:\n%s", want, joined)
		}
	}
}

// GitLab은 일반 브랜치 API로 만들고 브랜치 이름 규칙이 연결 수단이다. 특수한
// linked branch 개념이 없으므로 순서 주의사항도 없다 — GitHub 안내를 복사해
// 붙이면 거짓이 된다.
func TestGitLabStepsDoNotClaimOrcaOrdering(t *testing.T) {
	steps := Steps("gitlab", "https://gitlab.example.com/acme/repo/-/issues/16", "16-demo", "main", "deadbeef")
	if len(steps) == 0 {
		t.Fatal("gitlab steps must exist")
	}
	for _, step := range steps {
		if strings.Contains(step.Description, "Orca") {
			t.Fatalf("gitlab은 브랜치 이름 규칙이 연결 수단이라 Orca 순서 주의가 필요 없다: %q", step.Description)
		}
	}
}

// 단계 구조의 불변식은 "첫 단계가 MCP 경로, 마지막 단계가 fail, 그 사이에 해당
// provider CLI를 쓰는 fallback_api가 있다"는 것이다. 단계 **수**는 불변식이
// 아니다 — GitHub은 base SHA를 못박기 위해 node id 조회 단계가 하나 더 있다(#176).
func TestStepsKeepTheirExistingShape(t *testing.T) {
	for _, test := range []struct {
		provider  string
		issueURL  string
		wantCmd   string
		wantSteps int
	}{
		// #306: 생성 뒤 두 readback 단계가 더해져 6단계다.
		{"github", "https://github.com/acme/repo/issues/16", "gh", 6},
		{"gitlab", "https://gitlab.example.com/acme/repo/-/issues/16", "glab", 3},
	} {
		t.Run(test.provider, func(t *testing.T) {
			steps := Steps(test.provider, test.issueURL, "16-demo", "main", "deadbeef")
			if len(steps) != test.wantSteps {
				t.Fatalf("%s steps 수가 %d여야 한다: %d", test.provider, test.wantSteps, len(steps))
			}
			if steps[len(steps)-1].Strategy != "fail" {
				t.Fatalf("%s 마지막 단계는 fail이어야 한다: %q", test.provider, steps[len(steps)-1].Strategy)
			}
			var fallback *model.IssueOpsBranchPrepareStep
			for index := range steps {
				if steps[index].Strategy == "fallback_api" {
					fallback = &steps[index]
				}
			}
			if fallback == nil {
				t.Fatalf("%s에 fallback_api 단계가 있어야 한다: %+v", test.provider, steps)
			}
			if len(fallback.Command) == 0 || fallback.Command[0] != test.wantCmd {
				t.Fatalf("%s fallback 명령이 %q로 시작해야 한다: %v", test.provider, test.wantCmd, fallback.Command)
			}
			// Order는 1부터 연속이어야 한다 — 안내를 순서대로 읽는 계약이다.
			for index, step := range steps {
				if step.Order != index+1 {
					t.Fatalf("%s step %d의 Order가 %d다: %+v", test.provider, index, step.Order, steps)
				}
			}
		})
	}
}
