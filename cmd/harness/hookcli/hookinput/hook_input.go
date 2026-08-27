// Package hookinput reads the few host hook stdin fields the context hooks
// need. Hosts send the current working directory as cwd; explicit repo,
// workspace, and project_dir aliases plus a nested hook_input envelope are
// accepted so the same reader serves Codex, Claude Code, and direct CLI use.
package hookinput

import (
	"encoding/json"
	"strings"
)

var repoKeys = []string{"repo", "cwd", "workspace", "workspace_root", "project_dir"}

func RepoFromHookInput(input []byte) string {
	if len(strings.TrimSpace(string(input))) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err != nil {
		return ""
	}
	if repo := firstRepoKey(obj); repo != "" {
		return repo
	}
	if nested, ok := obj["hook_input"].(map[string]any); ok {
		return firstRepoKey(nested)
	}
	return ""
}

func firstRepoKey(obj map[string]any) string {
	for _, key := range repoKeys {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
