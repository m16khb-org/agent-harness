package hookinput

import "testing"

// #100 계약: stdin 문자열 값의 내용 heuristic(`*** Begin Patch` 스캔,
// `.go`/`.agent-harness`/`testdata/` 단일행 추출)은 tool_input subtree 또는
// command/cmd 키 아래에서만 수행된다. 그 밖의 위치(top-level note/patch,
// 임의 중간 맵)의 문자열은 mutation target으로 승격되지 않는다.
// 키 기반 추출(path·`_path` suffix·file·filename·filesystem alias)은 이 계약과
// 무관하게 그대로 유지된다.
func TestPathsFromHookInputIgnoresContentHeuristicOutsideToolInputAndCommand(t *testing.T) {
	input := []byte(`{
	  "note":"internal/core/foo.go",
	  "patch":"*** Begin Patch\n*** Add File: /outside/x.go\n*** End Patch",
	  "metadata":{"summary":"see cmd/harness/foo.go"}
	}`)
	got := PathsFromHookInput(input)
	for _, unwanted := range []string{
		"internal/core/foo.go",
		"/outside/x.go",
		"see cmd/harness/foo.go",
		"cmd/harness/foo.go",
	} {
		if containsString(got, unwanted) {
			t.Fatalf("non tool_input/command string %q must not become a mutation target: %#v", unwanted, got)
		}
	}
	if len(got) != 0 {
		t.Fatalf("expected no mutation targets from content heuristic, got %#v", got)
	}
}

// #100 carve-out: CommandFromHookInput이 top-level `command`/`cmd`를 여전히
// 1순위로 읽으므로, 같은 문자열의 patch 스캔·내용 heuristic도 보존해 legacy
// top-level command 형상에서 worktreeguard가 약화되지 않게 한다.
func TestPathsFromHookInputKeepsContentHeuristicUnderCommandKeys(t *testing.T) {
	for _, key := range []string{"command", "cmd"} {
		t.Run(key, func(t *testing.T) {
			input := []byte(`{"` + key + `":"*** Begin Patch\n*** Add File: /repo/from-command.go\n*** End Patch"}`)
			got := PathsFromHookInput(input)
			if !containsString(got, "/repo/from-command.go") {
				t.Fatalf("%s patch path missing from mutation targets: %#v", key, got)
			}

			inline := []byte(`{"` + key + `":"go test ./internal/core/inline.go"}`)
			gotInline := PathsFromHookInput(inline)
			if !containsString(gotInline, "go test ./internal/core/inline.go") {
				t.Fatalf("%s inline .go string missing from mutation targets: %#v", key, gotInline)
			}
		})
	}
}

// #100 보존: tool_input subtree 내부의 patch 문자열·내용 heuristic 추출은 불변.
func TestPathsFromHookInputKeepsContentHeuristicInsideToolInput(t *testing.T) {
	input := []byte(`{
	  "tool_input":{
	    "patch":"*** Begin Patch\n*** Add File: /repo/inside-add.go\n*** Update File: /repo/.agent-harness/inside.md\n*** End Patch",
	    "note":"/repo/testdata/inside.json",
	    "nested":{"deep":"/repo/internal/core/deep.go"}
	  }
	}`)
	got := PathsFromHookInput(input)
	for _, want := range []string{
		"/repo/inside-add.go",
		"/repo/.agent-harness/inside.md",
		"/repo/testdata/inside.json",
		"/repo/internal/core/deep.go",
	} {
		if !containsString(got, want) {
			t.Fatalf("tool_input content path %q missing from mutation targets: %#v", want, got)
		}
	}
}
