package hookinput

import (
	"strings"
)

func PathsFromHookInput(input []byte) []string {
	obj := hookInputObject(input)
	seen := map[string]bool{}
	out := []string{}
	var walk func(any, bool)
	walk = func(v any, insideToolInput bool) {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				lk := strings.ToLower(k)
				childInsideToolInput := insideToolInput || lk == "tool_input"
				if !childInsideToolInput && (lk == "transcript_path" || lk == "agent_transcript_path") {
					continue
				}
				// tool_response/tool_result는 도구 실행 결과 echo다 — diff·patch
				// 텍스트 속 경로형 문자열이 mutation target으로 승격되면 소스
				// 체크아웃 misdirect 오탐이 된다(#95). tool_input 내부의 동명
				// 인자는 도구가 하려던 일이므로 보존한다.
				if !insideToolInput && (lk == "tool_response" || lk == "tool_result") {
					continue
				}
				filesystemAlias := insideToolInput && (lk == "source" || lk == "destination" || lk == "src" || lk == "dst" || lk == "target")
				if lk == "path" || strings.HasSuffix(lk, "_path") || lk == "file" || lk == "filename" || filesystemAlias {
					if s, ok := v.(string); ok {
						addHookPath(&out, seen, s)
					}
				}
				// 이 플래그는 "문자열 내용 heuristic 허용 범위"를 뜻한다(#100):
				// tool_input subtree에 더해 command/cmd 키 값도 허용한다.
				// CommandFromHookInput이 top-level command/cmd를 1순위로 읽으므로
				// 같은 문자열의 patch 스캔이 소리 없이 빠지면 legacy top-level
				// command 형상에서 worktreeguard가 base 폴백으로 약화된다.
				// 실제 host payload에서 command 값은 문자열뿐이라 map 부수효과
				// (alias 활성화)는 발생하지 않는다.
				walk(v, childInsideToolInput || lk == "command" || lk == "cmd")
			}
		case []any:
			for _, item := range x {
				walk(item, insideToolInput)
			}
		case string:
			// #100: 내용 heuristic(patch 스캔·인라인 경로)은 tool_input subtree
			// 또는 command/cmd 키 값에서만 수행한다. 그 밖의 stdin 위치 문자열은
			// mutation target으로 승격하지 않는다 — 블랙리스트 누적 클래스의
			// 구조적 제거.
			if !insideToolInput {
				return
			}
			if addPatchPathsFromHookString(&out, seen, x) {
				return
			}
			if !strings.Contains(x, "\n") && (strings.Contains(x, ".go") || strings.Contains(x, ".agent-harness") || strings.Contains(x, "testdata/")) {
				addHookPath(&out, seen, x)
			}
		}
	}
	walk(obj, false)
	return out
}

func addPatchPathsFromHookString(out *[]string, seen map[string]bool, value string) bool {
	if !strings.Contains(value, "*** Begin Patch") {
		return false
	}
	for _, line := range strings.Split(value, "\n") {
		for _, prefix := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "} {
			if strings.HasPrefix(line, prefix) {
				addHookPath(out, seen, strings.TrimSpace(strings.TrimPrefix(line, prefix)))
				break
			}
		}
	}
	return true
}

func addHookPath(out *[]string, seen map[string]bool, value string) {
	value = strings.TrimSpace(value)
	if value == "" || seen[value] {
		return
	}
	seen[value] = true
	*out = append(*out, value)
}
