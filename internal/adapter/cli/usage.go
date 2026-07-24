package cli

import "fmt"

const issueOpsDevilsAdvocateUsage = "  agent-harness " + "issueops devils-advocate review --id ID --verdict pass|revise|stop [--finding TEXT]... [--waive --waiver-rationale TEXT] [--json]"

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
		{Name: "status", Description: "summarize doctor, daemon, state, worker, and verification status"},
		{Name: "doctor", Description: "diagnose harness installation, state, hooks, MCP, daemon, and project docs"},
		{Name: "docs", Description: "index harness guidance documents"},
		{Name: "policy", Description: "evaluate command policy, fake-run commands, and write audit records"},
		{Name: "guard", Description: "check language-agnostic code and test anti-patterns"},
		{Name: "quality", Description: "inspect quality signals and next improvement candidates"},
		{Name: "verify-work", Description: "run a lightweight evidence matrix for current work"},
		{Name: "trace", Description: "analyze trace-like verification and lifecycle evidence"},
		{Name: "contract", Description: "print or check CLI/MCP response compatibility contracts"},
		{Name: "state", Description: "read and write small agent state checkpoints"},
		{Name: "issueops", Description: "track issue-driven IssueOps work cycles"},
		{Name: "api-doc", Description: "run API documentation static and agent review gates"},
		{Name: "hook", Description: "run prompt-routing hooks"},
		{Name: "project", Description: "bootstrap and maintain project operating docs"},
		{Name: "install", Description: "install shared native skills and MCP config"},
		{Name: "install-native", Description: "compatibility alias for install"},
		{Name: "update", Description: "rebuild and refresh user-level integrations"},
		{Name: "bootstrap", Description: "set up user-level integrations"},
		{Name: "daemon", Description: "manage the MCP backend daemon"},
		{Name: "worker", Description: "manage safe local worker jobs and read-only command evidence"},
		{Name: "loop", Description: "track durable verify-until-done loop contracts"},
		{Name: "web-fetch", Description: "fetch public web pages with resilient validation and run deterministic web-fetch benchmarks"},
		{Name: "self-verify", Description: "run harness verification gates"},
		{Name: "self-augment", Description: "plan self-augmentation candidates and lessons"},
		{Name: "mcp", Description: "serve the MCP stdio proxy and clean up proxy processes"},
		{Name: "version", Description: "print agent-harness version"},
	}
}

// Usage returns the canonical CLI usage text. Keeping this in the CLI adapter
// package makes command-surface changes testable without invoking main().
func Usage(version string) string {
	return fmt.Sprintf(`agent-harness %s

Usage:
  agent-harness inspect [--json] [--repo PATH]
  agent-harness preflight [--json] [PATH]
  agent-harness status [--repo PATH] [--json]
  agent-harness doctor [--repo PATH] [--preserve-cycle ID]... [--preserve-terminal HANDLE]... [--json]
  agent-harness docs [index] [--json]
  agent-harness policy check [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  agent-harness policy fake-run [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  agent-harness policy run --read-only [--workspace-root PATH] [--cwd PATH] [--json] -- ARGV...
  agent-harness policy audit [--workspace-root PATH] [--cwd PATH] [--write] [--network] [--json] -- ARGV...
  agent-harness guard check [--repo PATH] [--staged] [--all] [--json] [--] [FILES...]
  agent-harness quality inspect [--repo PATH] [--json]
  agent-harness verify-work [--repo PATH] [--all] [--json] [--] [READ_ONLY_ARGV...]
  agent-harness trace analyze --input <jsonl|state-key> [--json]
  agent-harness contract schema [--json]
  agent-harness contract check [--json]
  agent-harness state write --key KEY (--value TEXT|--input FILE|--stdin) [--json]
  agent-harness state read --key KEY [--json]
  agent-harness state list [--json]
  agent-harness state prune --max-age DURATION [--confirm] [--json]
  agent-harness state doctor [--json]
  agent-harness state migrate [--confirm] [--json]
  agent-harness issueops start --repo PATH [--branch NAME] [--json]
  agent-harness issueops status --id ID [--json]
  agent-harness issueops intent record --id ID --raw-request TEXT --interpreted-intent TEXT --success-criteria TEXT [--constraint TEXT] [--ambiguity TEXT] [--non-goal TEXT] [--json]
  agent-harness issueops link-issue --id ID --issue-url URL [--json]
  agent-harness issueops link-child --id ID --child-url URL [--title TEXT] [--json]
  agent-harness issueops child start --parent ID --branch BRANCH --title TEXT --scope TEXT --acceptance TEXT [--acceptance TEXT...] [--child-issue-url URL] [--json]
  agent-harness issueops child status --parent ID [--repair] [--json]
  agent-harness issueops child list --parent ID [--json]
  agent-harness issueops child accept --parent ID --child ID --evidence TEXT [--evidence TEXT...] [--json]
  agent-harness issueops child reject --parent ID --child ID --reason REASON [--json]
  agent-harness issueops child drop --parent ID --child ID --reason REASON [--json]
  agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--base-sha SHA] [--remote-branch-url URL] [--link-verified] [--json]
  agent-harness issueops link-worktree --id ID --worktree-path PATH [--json]
  agent-harness issueops design review --id ID --problem-summary TEXT --proposed-design TEXT --verification TEXT [--refactor-plan TEXT] [--alternative TEXT] [--risk TEXT] [--open-question TEXT] [--approved] [--json]
  agent-harness issueops compatibility review --id ID --backward-compatibility TEXT --side-effect TEXT --rollback-plan TEXT --verification TEXT [--blocker TEXT] [--approved] [--json]
  agent-harness issueops link-plan --id ID --plan-path PATH [--json]
  agent-harness issueops execution prepare --id ID --mode auto|direct|orca --owner-host codex|claude --owner-model MODEL [--owner-effort EFFORT] ACTOR_FLAGS [--confirm] [--json]
  agent-harness issueops execution status --id ID [--json]
  agent-harness issueops execution claim --id ID --generation N --claim-token-file PATH [--issue-body-sha256 SHA256 --context-packet-sha256 SHA256] ACTOR_FLAGS [--json]
  agent-harness issueops execution release --id ID --generation N ACTOR_FLAGS [--json]
  agent-harness issueops execution replace --id ID --expected-generation N (--preview|--revoke|--finalize-preview|--finalize|--reseed) [fingerprint/reason flags] ACTOR_FLAGS [--confirm] [--json]
  agent-harness issueops execution reconcile --id ID (--preview|--confirm) ACTOR_FLAGS [--json]
  agent-harness issueops execution complete --id ID --generation N --final-head SHA --turing-report PATH --remote-artifact-url URL --verification TEXT... ACTOR_FLAGS --confirm [--json]
  agent-harness issueops reset-legacy --target-schema 1 (--preview|--status|--reconcile-remote --id ID --claim-id CLAIM --confirm|--drain-cycle --id ID --confirm|--confirm) [--expected-fingerprint SHA256] [--json]
  agent-harness issueops feedback add --id ID --source TEXT --body TEXT [--json]
  agent-harness issueops feedback mark-issue-updated --id ID [--json]
  agent-harness issueops pr-readiness --id ID [--strict] [--json]
  agent-harness issueops cleanup status --id ID [--merged] [--json]
  agent-harness issueops cleanup close-children --id ID --merged [--confirm] [--json]
  agent-harness issueops remote score --input PATH [--judge none|file] [--judge-file PATH] [--json]
  agent-harness issueops remote render-template --kind issue|child|pr --template KIND --title TEXT --provider github|gitlab --field key=value... [--score-file PATH] [--json]
  agent-harness issueops remote create-issue --id ID --title TEXT [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... [--confirm] [--json]
  agent-harness issueops remote create-child --id ID --title TEXT [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... [--confirm] [--json]
  agent-harness issueops remote create-pr --id ID --expected-generation N --title TEXT --head BRANCH --base BRANCH [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... ACTOR_FLAGS [--confirm] [--json]
  agent-harness issueops benchmark run --fixtures PATH [--judge none|file] [--judge-file PATH] [--json]
  agent-harness issueops benchmark compare --baseline KEY --candidate KEY [--json]
  agent-harness issueops benchmark gate --baseline KEY --candidate KEY --candidate-file PATH [--changed-path PATH]... [--json]
  agent-harness api-doc check|static-check|review [--repo PATH] [--all] [--json] [--] [FILES...]
  agent-harness hook user-prompt|pre-tool-use|post-tool-use|pre-compact|post-compact|session-start|stop [--json]
  agent-harness project bootstrap [--repo PATH] [--sync] [--dry-run] [--json]
  agent-harness project docs [--repo PATH] [--json]
  agent-harness project route-docs [--repo PATH] [--task TEXT] [--json]
  agent-harness daemon start|status|stop [--json]
  agent-harness worker enqueue --kind KIND [--payload TEXT] [--json]
  agent-harness worker run --read-only --kind KIND [--payload TEXT] [--workspace-root PATH] [--cwd PATH] [--json] -- ARGV...
  agent-harness worker status --id ID [--json]
  agent-harness worker list [--json]
  agent-harness worker cleanup-stuck [--json]
  agent-harness worker cancel --id ID [--json]
  agent-harness loop start --repo PATH --name NAME --goal TEXT [--max-attempts N] [--json] -- [VERIFY_ARGV...]
  agent-harness loop record-attempt --id ID --verdict pass|fail --evidence TEXT [--evidence TEXT...] [--json]
  agent-harness loop status (--id ID | --repo PATH --name NAME) [--json]
  agent-harness loop stop --id ID (--success | --reason TEXT) [--json]
  agent-harness web-fetch fetch --url URL [--timeout 30s] [--max-chars N] [--json]
  agent-harness web-fetch benchmark --fixtures PATH [--live] [--compare-baseline PATH] [--json]
  agent-harness install [--interactive] [--project-local] [--path-mode=auto|manual|skip] [--dry-run] [--json]
  agent-harness install-native [--interactive] [--project-local] [--path-mode=auto|manual|skip] [--dry-run] [--json]  # compatibility alias
  agent-harness update [--path-mode=auto|manual|skip] [--dry-run] [--json]
  agent-harness bootstrap [--interactive] [--sync] [--path-mode=auto|manual|skip] [--dry-run] [--json]
  agent-harness self-verify [--full] [--iterations=10] [--seed=N] [--target-score=95] [--progress=none|jsonl] [--llm-eval] [--llm-eval-mode=advisory|gate] [--save-state] [--state-key KEY] [--json]
  agent-harness self-verify history [--prefix PREFIX] [--limit N] [--retention-limit N] [--prune-retention] [--confirm] [--json]
  agent-harness self-verify compare --baseline-key KEY --candidate-key KEY [--max-elapsed-regression-pct N] [--fail-on-regression] [--json]
  agent-harness self-verify promote --from-key KEY --baseline-key KEY [--confirm] [--json]
  agent-harness self-verify candidates [--save-state] [--state-key KEY] [--json]
  agent-harness self-augment [--cycles=1] [--target-score=95] [--save-state] [--state-key KEY] [--json]
  agent-harness self-augment lesson [--candidate ID] --lesson TEXT --next-action TEXT [--source TEXT] [--severity info|warning|error] [--state-key KEY] [--json]
  agent-harness mcp [cleanup [--dry-run|--apply] [--json]]
  agent-harness version
%s
`, version, issueOpsDevilsAdvocateUsage)
}
