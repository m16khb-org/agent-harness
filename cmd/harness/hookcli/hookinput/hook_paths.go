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
				if lk == "path" || strings.HasSuffix(lk, "_path") || lk == "file" || lk == "filename" {
					if s, ok := v.(string); ok {
						addHookPath(&out, seen, s)
					}
				}
				walk(v, childInsideToolInput)
			}
		case []any:
			for _, item := range x {
				walk(item, insideToolInput)
			}
		case string:
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
