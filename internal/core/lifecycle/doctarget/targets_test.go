package doctarget

import (
	"testing"

	"agent-harness/internal/core/lifecycle/model"
)

func TestForToolUseSkipsReadOnlyBashOutputPaths(t *testing.T) {
	targets := ForToolUse(model.HookToolUseLifecycleRequest{
		Tool:    "Bash",
		Command: "rg -n \"PostCompact|OPEN_API_SPEC\" .",
		Paths:   []string{"cmd/harness/hook_user_prompt.go", ".agent-harness/OPEN_API_SPEC.md"},
		Source:  "test",
	})
	if len(targets) != 0 {
		t.Fatalf("read-only Bash should not queue doc upkeep targets: %+v", targets)
	}
}

func TestForToolUseSkipsQuotedRedirectInReadOnlyBash(t *testing.T) {
	targets := ForToolUse(model.HookToolUseLifecycleRequest{
		Tool:    "Bash",
		Command: "rg -n 'a > b' internal/core/hook_prompt.go",
		Paths:   []string{"internal/core/hook_prompt.go"},
		Source:  "test",
	})
	if len(targets) != 0 {
		t.Fatalf("quoted redirect in read-only Bash should not queue doc upkeep targets: %+v", targets)
	}
}

func TestForToolUseAllowsMutatingBashCommand(t *testing.T) {
	targets := ForToolUse(model.HookToolUseLifecycleRequest{
		Tool:    "Bash",
		Command: "gofmt -w internal/core/lifecycle_state.go",
		Source:  "test",
	})
	if !containsString(targets, "OPERATIONS.md") {
		t.Fatalf("mutating Bash should queue lifecycle doc upkeep targets: %+v", targets)
	}
}

func TestBashCommandMutationFamilies(t *testing.T) {
	mutating := []string{
		"git add .",
		"git commit -m test",
		"git reset --hard HEAD",
		"git restore internal/x.go",
		"git checkout -- internal/x.go",
		"git switch feature",
		"git push origin HEAD",
		"git rebase origin/main",
		"git merge feature",
		"git worktree remove ../worktree",
		"git -C /tmp/repo add .",
		"git -C=/tmp/repo commit -m test",
		"go test ./... -count=1",
		"go -C . test ./... -count=1",
		"go build ./...",
		"go test -bench=. ./...",
		"go mod tidy",
		"go generate ./...",
		"go -C /tmp/repo mod tidy",
		"go -C=/tmp/repo generate ./...",
		"npm install",
		"pnpm add example",
		"yarn remove example",
		"bun install",
		"cargo fmt",
		"bash -c 'touch internal/x.go'",
		"bash -lc 'touch internal/x.go'",
		"sh -c 'printf x > internal/x.go'",
		"sh -ec 'printf x > internal/x.go'",
		"zsh -c 'rm internal/x.go'",
		"zsh -lc 'rm internal/x.go'",
		`python -c 'open("internal/x.go", "w").write("x")'`,
		`node -e 'require("fs").writeFileSync("internal/x.go", "x")'`,
	}
	for _, command := range mutating {
		t.Run(command, func(t *testing.T) {
			if !bashCommandMayMutate(command) {
				t.Fatalf("representative mutation family was not classified: %q", command)
			}
		})
	}
}

func TestBashCommandReadOnlyFamilies(t *testing.T) {
	for _, command := range []string{
		"git status --short",
		"git diff --stat",
		"git log -1 --oneline",
		"rg -n 'handoff' internal",
	} {
		t.Run(command, func(t *testing.T) {
			if bashCommandMayMutate(command) {
				t.Fatalf("representative read-only command was classified as mutating: %q", command)
			}
		})
	}
}

func TestToolUseMutationClassificationUsesShellAliases(t *testing.T) {
	for _, tool := range []string{"Bash", "shell_command", "exec_command", "unified_exec"} {
		t.Run(tool, func(t *testing.T) {
			if !ToolUseMayMutateLifecycleFiles(tool, "git add .") {
				t.Fatal("shell alias did not classify a known mutation")
			}
			if ToolUseMayMutateLifecycleFiles(tool, "git status --short") {
				t.Fatal("shell alias classified a representative read as mutation")
			}
		})
	}
}

func TestFilesystemMCPMutationClassificationFailsClosedOutsideExplicitReaders(t *testing.T) {
	for _, tool := range []string{
		"mcp__filesystem__create_folder",
		"mcp__filesystem__rename_file",
		"mcp__filesystem__append_file",
		"mcp__filesystem__unknown_operation",
	} {
		if !ToolUseMayMutateLifecycleFiles(tool, "") {
			t.Fatalf("filesystem MCP operation must fail closed as mutation: %s", tool)
		}
	}
	for _, tool := range []string{
		"mcp__filesystem__read_file",
		"mcp__filesystem__read_text_file",
		"mcp__filesystem__list_directory",
		"mcp__filesystem__list_files",
		"mcp__filesystem__search_files",
	} {
		if ToolUseMayMutateLifecycleFiles(tool, "") {
			t.Fatalf("explicit filesystem reader was classified as mutation: %s", tool)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
