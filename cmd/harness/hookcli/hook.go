package hookcli

import (
	"fmt"
	"io"
	"os"
	"time"

	"agent-harness/cmd/harness/hookcli/hookcatalog"
	"agent-harness/cmd/harness/hookcli/hookfailure"
	"agent-harness/internal/core"
)

func runHook(args []string) error {
	stdin, restoreStdin, stdinErr := captureReplayableHookStdin()
	if restoreStdin != nil {
		defer restoreStdin()
	}
	if stdinErr != nil {
		return stdinErr
	}
	started := time.Now()
	err := runHookDispatch(args)
	if err != nil {
		hookfailure.Record(args, stdin, err)
	}
	// Best-effort latency telemetry for real hook events (quality program
	// Q2 phase 2); meta subcommands (failures/metrics) are not hook events.
	if len(args) > 0 && args[0] != "failures" && args[0] != "metrics" {
		_ = core.RecordHookMetricEvent(core.HookMetricEvent{
			Hook:       args[0],
			Host:       hookfailure.ArgValue(args, "--host"),
			DurationMS: time.Since(started).Milliseconds(),
			Decision:   hookMetricDecision,
		})
	}
	hookMetricDecision = ""
	return err
}

// hookMetricDecision is set by enforcement gates when they block. The hook
// CLI handles exactly one event per process, so a package-level marker is
// race-free and lets the single dispatcher metric line carry the decision.
var hookMetricDecision string

func markHookMetricBlocked() { hookMetricDecision = "block" }

func captureReplayableHookStdin() ([]byte, func(), error) {
	stat, err := os.Stdin.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice != 0 {
		return nil, nil, nil
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, nil, fmt.Errorf("read hook stdin: %w", err)
	}
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write(stdin)
		_ = w.Close()
	}()
	return stdin, func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}, nil
}

func runHookDispatch(args []string) error {
	if len(args) == 0 {
		hookUsage()
		return fmt.Errorf("missing hook subcommand")
	}
	switch args[0] {
	case "user-prompt":
		return runHookUserPrompt(args[1:])
	case "pre-tool-use":
		return runHookPreToolUse(args[1:])
	case "post-tool-use":
		return runHookPostToolUse(args[1:])
	case "pre-compact":
		return runHookPreCompact(args[1:])
	case "post-compact":
		return runHookPostCompact(args[1:])
	case "session-start":
		return runHookSessionStart(args[1:])
	case "stop":
		return runHookStop(args[1:])
	case "failures":
		return hookfailure.Run(args[1:])
	case "metrics":
		return hookfailure.RunMetrics(args[1:])
	default:
		hookUsage()
		return fmt.Errorf("unknown hook subcommand %q", args[0])
	}
}

func runHookPostCompact(args []string) error {
	return hookcatalog.RunPostCompact(args, hookCatalogConfig())
}

func runHookSessionStart(args []string) error {
	return hookcatalog.RunSessionStart(args, hookCatalogConfig())
}

func hookCatalogConfig() hookcatalog.Config {
	return hookcatalog.Config{ResolveTarget: ResolveTarget, PrintJSON: printJSON}
}

func hookUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness hook user-prompt [--prompt TEXT] [--host codex|claude] [--enable-agy-hints] [--json]
  agent-harness hook pre-tool-use [--repo PATH] [--host codex|claude] [--enforce-search-routing] [--enforce-worktree] [--enforce-staged-checks] [--enforce-gitops-kubectl] [--json]
  agent-harness hook post-tool-use [--repo PATH] [--json]
  agent-harness hook pre-compact [--repo PATH] [--json]
  agent-harness hook post-compact [--repo PATH] [--host codex|claude] [--json]
  agent-harness hook session-start [--repo PATH] [--host codex|claude] [--json]
  agent-harness hook stop [--repo PATH] [--host codex|claude] [--enforce-numbered-next-actions] [--relay-next-action-judgement] [--json]
  agent-harness hook failures [--limit N] [--json]
  agent-harness hook failures --prune DURATION [--json]
  agent-harness hook failures prune --max-age DURATION [--json]
  agent-harness hook failures stats [--json]
  agent-harness hook metrics [--json]
`)
}
