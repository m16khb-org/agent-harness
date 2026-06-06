package hookcli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func runHook(args []string) error {
	stdin, restoreStdin, stdinErr := captureReplayableHookStdin()
	if restoreStdin != nil {
		defer restoreStdin()
	}
	if stdinErr != nil {
		return stdinErr
	}
	err := runHookDispatch(args)
	if err != nil {
		recordHookFailure(args, stdin, err)
	}
	return err
}

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
		return runHookFailures(args[1:])
	default:
		hookUsage()
		return fmt.Errorf("unknown hook subcommand %q", args[0])
	}
}

func hookArgValue(args []string, flagName string) string {
	prefix := flagName + "="
	for i, arg := range args {
		if arg == flagName && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
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
`)
}
