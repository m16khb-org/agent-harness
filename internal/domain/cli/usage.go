package cli

import (
	"fmt"
	"strings"
)

// Command describes a stable top-level CLI command exposed by the harness.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Commands returns the deterministic top-level CLI command catalog. The main
// package still owns subcommand execution; this adapter package owns the human
// command surface so usage and contract checks do not drift from routing.
func Commands() []Command {
	commands := []Command{
		{Name: "inspect", Description: "inspect harness installation and native integration"},
		{Name: "preflight", Description: "run read-only git preflight checks"},
		{Name: "system-status", Description: "summarize doctor, daemon, state, worker, and verification status"},
		{Name: "doctor", Description: "diagnose harness installation, state, hooks, MCP, daemon, and project docs"},
		{Name: "docs", Description: "index harness guidance documents"},
		{Name: "policy", Description: "evaluate command policy, fake-run commands, and write audit records"},
		{Name: "guard", Description: "check language-agnostic code and test anti-patterns"},
		{Name: "quality", Description: "inspect quality signals and next improvement candidates"},
		{Name: "verify-work", Description: "run a lightweight evidence matrix for current work"},
		{Name: "trace", Description: "analyze trace-like verification and lifecycle evidence"},
		{Name: "contract", Description: "print or check CLI/MCP response compatibility contracts"},
		{Name: "state", Description: "read and write small agent state checkpoints"},
		{Name: "api-doc", Description: "run API documentation static and agent review gates"},
		{Name: "hook", Description: "run host lifecycle context hooks"},
		{Name: "project", Description: "bootstrap and maintain project operating docs"},
		{Name: "install", Description: "install shared native skills and MCP config"},
		{Name: "update", Description: "rebuild and refresh user-level integrations"},
		{Name: "bootstrap", Description: "set up user-level integrations"},
		{Name: "daemon", Description: "manage the MCP backend daemon"},
		{Name: "worker", Description: "manage safe local worker jobs and read-only command evidence"},
		{Name: "loop", Description: "track durable verify-until-done loop contracts"},
		{Name: "gates", Description: "evaluate unlazy-compatible task gate ledgers with policy-gated checks"},
		{Name: "channel", Description: "exchange durable cross-session messages through shared issueops state"},
		{Name: "web-fetch", Description: "fetch public web pages with resilient validation and run deterministic web-fetch benchmarks"},
		{Name: "self-verify", Description: "run harness verification gates"},
		{Name: "self-augment", Description: "plan self-augmentation candidates and lessons"},
		{Name: "mcp", Description: "serve the MCP stdio proxy and clean up proxy processes"},
		{Name: "version", Description: "print issueops version"},
	}
	return append(commands, LifecycleCommands()...)
}

// Usage returns the canonical CLI usage text. Keeping this in the CLI adapter
// package makes command-surface changes testable without invoking main().
//
// `issueops` 줄은 이 함수에 적지 않는다. `issueOpsUsageCatalog`가 유일한 원본이고
// 여기서는 축약 키로 걸러 렌더한다(#188). 두 곳에 손으로 적으면 양쪽에서 동시에
// 빠진 명령을 어떤 테스트도 잡지 못한다 — `execution switch-mode`가 그랬다.
func Usage(version string) string {
	return fmt.Sprintf(`issueops %s

Usage:
  issueops inspect [--json] [--repo PATH]
  issueops preflight [--json] [PATH]
  issueops system-status [--repo PATH] [--json]
  issueops doctor [--repo PATH] [--preserve-cycle ID]... [--preserve-terminal HANDLE]... [--json]
  issueops docs [index] [--json]
  issueops policy check [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  issueops policy fake-run [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  issueops policy run --read-only [--workspace-root PATH] [--cwd PATH] [--json] -- ARGV...
  issueops policy audit [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  issueops guard check [--repo PATH] [--staged] [--all] [--json] [--] [FILES...]
  issueops quality inspect [--repo PATH] [--json]
  issueops verify-work [--repo PATH] [--all] [--json] [--] [READ_ONLY_ARGV...]
  issueops trace analyze --input <jsonl|state-key> [--json]
  issueops contract schema [--json]
  issueops contract check [--json]
  issueops state write --key KEY (--value TEXT|--input FILE|--stdin) [--json]
  issueops state read --key KEY [--json]
  issueops state list [--json]
  issueops state prune --max-age DURATION [--confirm] [--json]
  issueops state doctor [--json]
%s
  issueops api-doc check|static-check|review [--repo PATH] [--all] [--json] [--] [FILES...]
  issueops hook session-start|post-compact [--repo PATH] [--host codex|claude] [--json]
  issueops project bootstrap [--repo PATH] [--sync] [--dry-run] [--json]
  issueops project docs [--repo PATH] [--json]
  issueops project route-docs [--repo PATH] [--task TEXT] [--json]
  issueops daemon start|status|stop [--json]
  issueops worker enqueue --kind KIND [--payload TEXT] [--json]
  issueops worker run --read-only --kind KIND [--payload TEXT] [--workspace-root PATH] [--cwd PATH] [--json] -- ARGV...
  issueops worker status --id ID [--json]
  issueops worker list [--json]
  issueops worker cleanup-stuck [--json]
  issueops worker cancel --id ID [--json]
  issueops loop start --repo PATH --name NAME --goal TEXT [--max-attempts N] [--json] -- [VERIFY_ARGV...]
  issueops loop record-attempt --id ID --verdict pass|fail --evidence TEXT [--evidence TEXT...] [--json]
  issueops loop status (--id ID | --repo PATH --name NAME) [--json]
  issueops loop stop --id ID (--success | --reason TEXT) [--json]
  issueops gates init [--file PATH] --scope TEXT --gate "ID: outcome | CHECK: cmd | EXPECT: expect" [--gate SPEC...] [--json]
  issueops gates check [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--timeout-seconds N] [--env NAME,NAME] [--write] [--network] [--json]
  issueops gates status [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--json]
  issueops gates report [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--json]
  issueops gates abandon --gate ID --reason TEXT [--file PATH] [--json]
  issueops channel send --channel NAME --from SESSION --message TEXT [--json]
  issueops channel recv --channel NAME [--since MSG_ID] [--wait] [--timeout-seconds N] [--limit N] [--json]
  issueops web-fetch fetch --url URL [--timeout 30s] [--max-chars N] [--json]
  issueops web-fetch benchmark --fixtures PATH [--live] [--compare-baseline PATH] [--json]
  issueops install [--interactive] [--project-local] [--path-mode=auto|manual|skip] [--dry-run] [--json]
  issueops update [--path-mode=auto|manual|skip] [--dry-run] [--json]
  issueops bootstrap [--interactive] [--sync] [--path-mode=auto|manual|skip] [--dry-run] [--json]
  issueops self-verify [--seed=N] [--target-score=95] [--progress=none|jsonl] [--llm-eval] [--llm-eval-mode=advisory|gate] [--save-state] [--state-key KEY] [--json]
  issueops self-verify history [--prefix PREFIX] [--limit N] [--retention-limit N] [--prune-retention] [--confirm] [--json]
  issueops self-verify compare --baseline-key KEY --candidate-key KEY [--max-elapsed-regression-pct N] [--fail-on-regression] [--json]
  issueops self-verify promote --from-key KEY --baseline-key KEY [--confirm] [--json]
  issueops self-verify candidates [--save-state] [--state-key KEY] [--json]
  issueops self-augment [--cycles=1] [--target-score=95] [--save-state] [--state-key KEY] [--json]
  issueops self-augment lesson [--candidate ID] --lesson TEXT --next-action TEXT [--source TEXT] [--severity info|warning|error] [--state-key KEY] [--json]
  issueops mcp [cleanup [--dry-run|--apply] [--json]]
  issueops version
%s

%s
`, version, strings.Join(IssueOpsUsageLines(), "\n"), "", IssueOpsActorFlagLegend)
}
