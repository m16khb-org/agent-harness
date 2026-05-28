package cli

import "fmt"

// Command describes a stable top-level CLI command exposed by the harness.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Commands returns the deterministic top-level CLI command catalog. The main
// package still owns subcommand execution; this adapter package owns the human
// command surface so usage and contract checks do not drift from routing.
func Commands() []Command {
	return []Command{
		{Name: "inspect", Description: "inspect harness installation and native integration"},
		{Name: "preflight", Description: "run read-only git preflight checks"},
		{Name: "docs", Description: "index harness guidance documents"},
		{Name: "policy", Description: "evaluate command policy, fake-run commands, and write audit records"},
		{Name: "contract", Description: "print or check CLI/MCP response compatibility contracts"},
		{Name: "state", Description: "read and write small agent state checkpoints"},
		{Name: "api-doc", Description: "run API documentation static and agent review gates"},
		{Name: "hook", Description: "run prompt-routing hooks"},
		{Name: "project", Description: "bootstrap and maintain project operating docs"},
		{Name: "install-native", Description: "install shared native skills and MCP config"},
		{Name: "daemon", Description: "manage the MCP backend daemon"},
		{Name: "worker", Description: "manage safe no-shell local worker jobs"},
		{Name: "self-verify", Description: "run harness verification gates"},
		{Name: "self-augment", Description: "plan self-augmentation candidates and lessons"},
		{Name: "mcp", Description: "serve the MCP stdio proxy"},
		{Name: "version", Description: "print harness version"},
	}
}

// Usage returns the canonical CLI usage text. Keeping this in the CLI adapter
// package makes command-surface changes testable without invoking main().
func Usage(version string) string {
	return fmt.Sprintf(`harness %s

Usage:
  harness inspect [--json] [--repo PATH]
  harness preflight [--json] [PATH]
  harness docs [index] [--json]
  harness policy check [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  harness policy fake-run [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  harness policy audit [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  harness contract schema [--json]
  harness contract check [--json]
  harness state write --key KEY (--value TEXT|--input FILE|--stdin) [--json]
  harness state read --key KEY [--json]
  harness state list [--json]
  harness state prune --max-age DURATION [--confirm] [--json]
  harness state doctor [--json]
  harness state migrate [--confirm] [--json]
  harness api-doc check|static-check|review [--repo PATH] [--all] [--json] [--] [FILES...]
  harness hook user-prompt [--prompt TEXT] [--json]
  harness project bootstrap [--repo PATH] [--write] [--json]
  harness project docs [--repo PATH] [--json]
  harness project route-docs [--repo PATH] [--task TEXT] [--json]
  harness daemon start|status|stop [--json]
  harness worker enqueue --kind KIND [--payload TEXT] [--json]
  harness worker status --id ID [--json]
  harness worker list [--json]
  harness worker cancel --id ID [--json]
  harness install-native [--project-local] [--dry-run] [--json]
  harness self-verify [--iterations=10] [--seed=N] [--target-score=95] [--progress=none|jsonl] [--save-state] [--state-key KEY] [--json]
  harness self-verify history [--prefix PREFIX] [--limit N] [--retention-limit N] [--prune-retention] [--confirm] [--json]
  harness self-verify compare --baseline-key KEY --candidate-key KEY [--max-elapsed-regression-pct N] [--fail-on-regression] [--json]
  harness self-verify promote --from-key KEY --baseline-key KEY [--confirm] [--json]
  harness self-verify candidates [--save-state] [--state-key KEY] [--json]
  harness self-augment [--cycles=1] [--target-score=95] [--save-state] [--state-key KEY] [--json]
  harness self-augment lesson [--candidate ID] --lesson TEXT --next-action TEXT [--source TEXT] [--severity info|warning|error] [--state-key KEY] [--json]
  harness mcp
  harness version
`, version)
}
