package hookcli

import (
	"flag"
	"fmt"
	"os"

	"agent-harness/cmd/harness/hookcli/hookcatalog"
	"agent-harness/cmd/harness/hookcli/hookenv"
)

// hookDisabled reports whether HARNESS_DISABLE_HOOKS turns this invocation into
// a no-op. The switch exists so a single host-level hook registration can stay
// installed while the agent works in repositories the harness does not own.
func hookDisabled() bool {
	return hookenv.Bool("HARNESS_DISABLE_HOOKS")
}

// runHook dispatches the host lifecycle context hooks. Only SessionStart and
// PostCompact exist: both read the static project-doc catalog and emit a
// host-compatible context payload. They never touch durable harness state,
// telemetry, or IssueOps authority (ADR 2026-08-10, 2026-08-27).
func runHook(args []string) error {
	if hookDisabled() {
		return nil
	}
	if len(args) == 0 {
		hookUsage()
		return fmt.Errorf("missing hook subcommand")
	}
	switch args[0] {
	case "--help", "-h", "help":
		hookUsage()
		return flag.ErrHelp
	case "session-start":
		return hookcatalog.RunSessionStart(args[1:], hookCatalogConfig())
	case "post-compact":
		return hookcatalog.RunPostCompact(args[1:], hookCatalogConfig())
	default:
		hookUsage()
		return fmt.Errorf("unknown hook subcommand %q", args[0])
	}
}

func hookCatalogConfig() hookcatalog.Config {
	return hookcatalog.Config{ResolveTarget: ResolveTarget, PrintJSON: printJSON}
}

func hookUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness hook session-start [--repo PATH] [--host codex|claude] [--json]
  agent-harness hook post-compact [--repo PATH] [--host codex|claude] [--json]

session-start renders the static project-doc catalog for every SessionStart
source, including the post-compaction re-run. post-compact keeps the same
catalog reachable for hosts without a SessionStart re-run (Omo) and for
diagnosis. HARNESS_DISABLE_HOOKS=1 turns both into a no-op.
`)
}
