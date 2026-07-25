package hookinput

import "testing"

// #95: tool_response/tool_result는 도구 실행 "결과" echo다. 그 안의 diff·patch·
// 경로형 문자열이 mutation target으로 승격되면 canonical worktree 편집이 소스
// 체크아웃 misdirect로 오탐되고 doc-upkeep·lint-gate에도 노이즈가 전파된다.
// transcript_path 선례처럼 subtree 전체(값 추출 + 하위 재귀)를 제외하되,
// tool_input 내부의 동명 키는 도구가 하려던 일이므로 보존한다.
func TestPathsFromHookInputIgnoresToolResponseSubtree(t *testing.T) {
	input := []byte(`{
	  "cwd":"/repo",
	  "tool_name":"Edit",
	  "tool_input":{
	    "file_path":"/repo.worktrees/95-demo/plan.md",
	    "tool_result":"/repo.worktrees/95-demo/inside-input.go"
	  },
	  "tool_response":{
	    "filePath":"/repo.worktrees/95-demo/plan.md",
	    "structuredPatch":["+see cmd/harness/issueopscli/issueops.go for details"],
	    "nested":{"file_path":"/outside/echoed.go"},
	    "patch":"*** Begin Patch\n*** Add File: /outside/from-response.go\n*** End Patch"
	  },
	  "tool_result":"internal/core/echoed_result.go"
	}`)
	got := PathsFromHookInput(input)
	for _, unwanted := range []string{
		"+see cmd/harness/issueopscli/issueops.go for details",
		"/outside/echoed.go",
		"/outside/from-response.go",
		"internal/core/echoed_result.go",
	} {
		if containsString(got, unwanted) {
			t.Fatalf("tool response text %q must not become a mutation target: %#v", unwanted, got)
		}
	}
	for _, want := range []string{
		"/repo.worktrees/95-demo/plan.md",
		"/repo.worktrees/95-demo/inside-input.go",
	} {
		if !containsString(got, want) {
			t.Fatalf("tool input path %q missing from mutation targets: %#v", want, got)
		}
	}
}
