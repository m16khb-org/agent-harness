package core

import (
	"path/filepath"
	"strings"
)

func lifecycleDocTargetsForToolUse(req HookToolUseLifecycleRequest) []string {
	if !toolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return nil
	}

	targets := []string{}
	for _, path := range req.Paths {
		targets = append(targets, docTargetsForLifecyclePath(path)...)
	}
	if command := strings.TrimSpace(req.Command); command != "" {
		targets = append(targets, docTargetsForLifecyclePath(command)...)
	}
	return normalizeTargetDocs(targets)
}

func toolUseMayMutateLifecycleFiles(tool, command string) bool {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "apply_patch", "edit", "write", "multiedit":
		return true
	case "bash":
		return bashCommandMayMutate(command)
	default:
		return false
	}
}

func bashCommandMayMutate(command string) bool {
	c := strings.ToLower(strings.TrimSpace(command))
	if c == "" {
		return false
	}
	if hasShellRedirect(c) {
		return true
	}
	for _, needle := range []string{
		"tee ", "sed -i", "perl -pi", "gofmt -w", "goimports -w",
		"git apply", "touch ", "mkdir ", "rm ", "mv ", "cp ", "chmod ", "chown ",
	} {
		if strings.Contains(c, needle) {
			return true
		}
	}
	return false
}

func hasShellRedirect(command string) bool {
	inSingle := false
	inDouble := false
	escaped := false
	for _, r := range command {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			escaped = true
			continue
		}
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '>':
			if !inSingle && !inDouble {
				return true
			}
		}
	}
	return false
}

func docTargetsForLifecyclePath(path string) []string {
	p := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	if p == "" {
		return nil
	}
	out := []string{}
	if strings.Contains(p, "hook") || strings.Contains(p, "install") || strings.Contains(p, "daemon") || strings.Contains(p, "mcp") || strings.Contains(p, "doctor") || strings.Contains(p, "lifecycle_state") || strings.Contains(p, "state.go") {
		out = append(out, "OPERATIONS.md", "CONVENTIONS.md", "ARCHITECTURE.md")
	}
	if strings.Contains(p, "_test.go") || strings.Contains(p, "testdata/") || strings.Contains(p, "golden") {
		out = append(out, "TESTING.md")
	}
	if strings.Contains(p, "api_doc") || strings.Contains(p, "openapi") || strings.Contains(p, "swagger") {
		out = append(out, "OPEN_API_SPEC.md")
	}
	return out
}

func uniqueDocUpkeepEvents(events []DocUpkeepEvent) []DocUpkeepEvent {
	unique := make([]DocUpkeepEvent, 0, len(events))
	seen := map[string]bool{}
	for _, event := range events {
		key := strings.Join(normalizeTargetDocs(event.TargetDocs), ",") + "\x00" + strings.TrimSpace(event.Summary)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, event)
	}
	return unique
}
