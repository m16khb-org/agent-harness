package hookcli

import (
	"strings"
)

func pathsFromHookInput(input []byte) []string {
	obj := hookInputObject(input)
	seen := map[string]bool{}
	out := []string{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				lk := strings.ToLower(k)
				if lk == "path" || strings.HasSuffix(lk, "_path") || lk == "file" || lk == "filename" {
					if s, ok := v.(string); ok {
						addHookPath(&out, seen, s)
					}
				}
				walk(v)
			}
		case []any:
			for _, item := range x {
				walk(item)
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
	walk(obj)
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
