package lifecycle

import (
	"strings"

	"agent-harness/internal/core/lifecycle/doctarget"
)

func oneFlag(flags map[string][]string, name string) (string, bool) {
	values := flags[name]
	if len(values) != 1 {
		return "", false
	}
	return values[0], true
}

func explicitIssueOpsReadOnlyTool(tool string) bool {
	tool = strings.ToLower(strings.TrimSpace(tool))
	switch tool {
	case "read", "glob", "grep", "search", "list", "ls":
		return true
	}
	if doctarget.ExplicitReadOnlyFilesystemTool(tool) {
		return true
	}
	for _, suffix := range []string{
		"__read_file", "__read_text_file", "__list_directory", "__list_files", "__search_files",
		"__codegraph_explore", "__get_library_docs", "__resolve_library_id",
	} {
		if strings.HasSuffix(tool, suffix) {
			return true
		}
	}
	return false
}
