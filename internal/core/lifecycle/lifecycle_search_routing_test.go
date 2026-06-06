package lifecycle

import (
	"testing"
)

func TestPreToolUseSearchRoutingBlocksRawStructuralSourceSearch(t *testing.T) {
	for _, command := range []string{
		`/usr/bin/rg -n "func Run" cmd/internal`,
		`rg "func Run"`,
		`rg "func Run" .`,
		`git grep "func Run"`,
		`rg "func Run" docs/ cmd/`,
		`rg "func Run" controllers/`,
		`rg "func Run" cmd/ # codegraph`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "zsh",
			Command:              command,
			EnforceSearchRouting: true,
		})
		if got.Decision != "block" {
			t.Fatalf("expected command to be blocked: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseSearchRoutingAllowsRawExactLiteralSearch(t *testing.T) {
	for _, command := range []string{
		`rg "DATABASE_URL" internal config`,
		`rg "PostToolUseFailure" internal/core`,
		`rg "Cannot read property" src`,
		`rg "snapshot_manager.go" internal`,
		`rg "pattern" snapshot_manager.go`,
		`rg "pattern" ./snapshot_manager.go`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "zsh",
			Command:              command,
			EnforceSearchRouting: true,
		})
		if got.Decision != "allow" {
			t.Fatalf("expected exact literal search to be allowed: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseSearchRoutingBlocksCodexShellToolNames(t *testing.T) {
	for _, tool := range []string{"shell_command", "unified_exec", "exec_command"} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 tool,
			Command:              `rg "type Hook" internal/core`,
			EnforceSearchRouting: true,
		})
		if got.Decision != "block" {
			t.Fatalf("expected Codex shell tool %q to be blocked, got %+v", tool, got)
		}
	}
}

func TestPreToolUseSearchRoutingAllowsDocsLiteralCodeNames(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Tool:                 "Bash",
		Command:              `rg "main.go" docs/ README.md`,
		EnforceSearchRouting: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected docs literal search to be allowed: %+v", got)
	}
}

func TestPreToolUseSearchRoutingAllowsExternalAbsoluteTargets(t *testing.T) {
	repo := t.TempDir()
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:                 repo,
		Tool:                 "Bash",
		Command:              `grep -R "PostToolUse" -n /Applications/Codex.app/Contents/Resources`,
		EnforceSearchRouting: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected external absolute target search to be allowed: %+v", got)
	}
}

func TestPreToolUseSearchRoutingBlocksCodeGraphForExactSearch(t *testing.T) {
	for _, query := range []string{"DATABASE_URL", "PostToolUseFailure", "Cannot read property", "snapshot_manager.go", "TODO"} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "codegraph_context",
			Command:              query,
			EnforceSearchRouting: true,
		})
		if got.Decision != "block" {
			t.Fatalf("expected exact CodeGraph query to be blocked: %q -> %+v", query, got)
		}
	}
}

func TestPreToolUseSearchRoutingAllowsCodeGraphForStructuralSearch(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Tool:                 "codegraph_trace",
		Command:              "impact of changing BuildLifecyclePreToolUseDecision",
		EnforceSearchRouting: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected structural CodeGraph query to be allowed: %+v", got)
	}
}
