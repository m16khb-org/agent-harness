package doctarget

import (
	"path/filepath"
	"strings"

	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	"agent-harness/internal/core/lifecycle/docupkeep"
	"agent-harness/internal/domain/commandparse"
	"agent-harness/internal/domain/searchrouting"
)

func ForToolUse(req lifecyclecontract.HookToolUseLifecycleRequest) []string {
	if !ToolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return nil
	}

	targets := []string{}
	for _, path := range req.Paths {
		targets = append(targets, docTargetsForLifecyclePath(path)...)
	}
	if command := strings.TrimSpace(req.Command); command != "" {
		targets = append(targets, docTargetsForLifecyclePath(command)...)
	}
	return docupkeep.NormalizeTargetDocs(targets)
}

func ToolUseMayMutateLifecycleFiles(tool, command string) bool {
	normalizedTool := strings.ToLower(strings.TrimSpace(tool))
	switch normalizedTool {
	case "apply_patch", "edit", "write", "multiedit", "notebookedit", "notebook_edit":
		return true
	}
	if strings.Contains(normalizedTool, "filesystem") {
		return !ExplicitReadOnlyFilesystemTool(normalizedTool)
	}
	if searchrouting.IsShellTool(tool) {
		return bashCommandMayMutate(command)
	}
	return false
}

func ExplicitReadOnlyFilesystemTool(tool string) bool {
	tool = strings.ToLower(strings.TrimSpace(tool))
	if !strings.Contains(tool, "filesystem") {
		return false
	}
	for _, suffix := range []string{
		"__read_file", "__read_text_file", "__list_directory", "__list_files", "__search_files",
	} {
		if strings.HasSuffix(tool, suffix) {
			return true
		}
	}
	return false
}

func bashCommandMayMutate(command string) bool {
	c := strings.ToLower(strings.TrimSpace(command))
	if c == "" {
		return false
	}
	if commandparse.HasActiveOutputRedirect(command) {
		return true
	}
	if commandTokensMayMutate(commandparse.SplitCommandTokens(command)) {
		return true
	}
	for _, needle := range []string{
		"tee ", "sed -i", "perl -pi", "gofmt -w", "goimports -w",
		"git apply", "git checkout -b", "git switch -c", "git worktree add",
		"touch ", "mkdir ", "rm ", "mv ", "cp ", "chmod ", "chown ",
	} {
		if strings.Contains(c, needle) {
			return true
		}
	}
	return false
}

func commandTokensMayMutate(tokens []string) bool {
	for i, token := range tokens {
		name := searchrouting.SearchTokenName(token)
		switch name {
		case "git":
			if mutationSubcommandAfterDirectoryFlag(tokens, i+1, map[string]bool{
				"add": true, "apply": true, "branch": true, "checkout": true, "cherry-pick": true,
				"clean": true, "commit": true, "merge": true, "push": true, "rebase": true,
				"reset": true, "restore": true, "revert": true, "switch": true, "tag": true, "worktree": true,
			}) {
				return true
			}
		case "go":
			subcommand, at := nextCommandTokenAfterDirectoryFlag(tokens, i+1)
			if map[string]bool{
				"build": true, "clean": true, "generate": true, "install": true,
				"run": true, "test": true, "tool": true, "vet": true,
			}[subcommand] || subcommand == "mod" && at+1 < len(tokens) && searchrouting.SearchTokenName(tokens[at+1]) == "tidy" {
				return true
			}
		case "npm", "pnpm", "yarn", "bun":
			if mutationSubcommand(tokens, i+1, map[string]bool{"install": true, "add": true, "remove": true}) {
				return true
			}
		case "cargo":
			if mutationSubcommand(tokens, i+1, map[string]bool{"fmt": true}) && !containsCommandToken(tokens[i+1:], "--check") {
				return true
			}
		case "bash", "sh", "zsh":
			if containsShellCommandFlag(tokens[i+1:]) {
				return true
			}
		case "python", "python3":
			if containsCommandToken(tokens[i+1:], "-c") {
				return true
			}
		case "node":
			if containsCommandToken(tokens[i+1:], "-e") || containsCommandToken(tokens[i+1:], "--eval") {
				return true
			}
		}
	}
	return false
}

func mutationSubcommandAfterDirectoryFlag(tokens []string, start int, mutating map[string]bool) bool {
	command, _ := nextCommandTokenAfterDirectoryFlag(tokens, start)
	return mutating[command]
}

func nextCommandTokenAfterDirectoryFlag(tokens []string, start int) (string, int) {
	for i := start; i < len(tokens); i++ {
		value := strings.TrimSpace(tokens[i])
		if value == "-C" {
			i++
			continue
		}
		if value == "" || strings.HasPrefix(value, "-") {
			continue
		}
		return searchrouting.SearchTokenName(value), i
	}
	return "", -1
}

func containsShellCommandFlag(tokens []string) bool {
	for _, token := range tokens {
		value := strings.TrimSpace(token)
		if len(value) > 1 && strings.HasPrefix(value, "-") && !strings.HasPrefix(value, "--") && strings.Contains(value[1:], "c") {
			return true
		}
	}
	return false
}

func mutationSubcommand(tokens []string, start int, mutating map[string]bool) bool {
	command, _ := nextCommandToken(tokens, start)
	return mutating[command]
}

func nextCommandToken(tokens []string, start int) (string, int) {
	for i := start; i < len(tokens); i++ {
		value := strings.TrimSpace(tokens[i])
		if value == "" || strings.HasPrefix(value, "-") {
			continue
		}
		return searchrouting.SearchTokenName(value), i
	}
	return "", -1
}

func containsCommandToken(tokens []string, want string) bool {
	for _, token := range tokens {
		if strings.TrimSpace(token) == want {
			return true
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

func UniqueEvents(events []lifecyclecontract.DocUpkeepEvent) []lifecyclecontract.DocUpkeepEvent {
	unique := make([]lifecyclecontract.DocUpkeepEvent, 0, len(events))
	seen := map[string]bool{}
	for _, event := range events {
		key := strings.Join(docupkeep.NormalizeTargetDocs(event.TargetDocs), ",") + "\x00" + strings.TrimSpace(event.Summary)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, event)
	}
	return unique
}
