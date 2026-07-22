package hookcli

import (
	"flag"
	"io"
	"os"
	"strings"

	"agent-harness/cmd/harness/hookcli/hookinput"
	hookadapter "agent-harness/internal/adapter/hook"
	"agent-harness/internal/core"
	"agent-harness/internal/core/lifecycle/doctarget"
	"agent-harness/internal/core/lintgate"
)

func runHookPostToolUse(args []string) error {
	fs := flag.NewFlagSet("hook post-tool-use", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	host := fs.String("host", "", "hook host (claude or codex); controls whether lint feedback is injected")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = hookinput.RepoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = ResolveTarget("")
	}
	tool := hookinput.ToolNameFromHookInput(stdin)
	paths := hookinput.PathsFromHookInput(stdin)
	command := hookinput.CommandFromHookInput(stdin)
	req := core.HookToolUseLifecycleRequest{
		Repo:    parsedRepo,
		Tool:    tool,
		Paths:   paths,
		Command: command,
		Source:  "post-tool-use",
	}
	result, err := core.RecordLifecycleToolUse(req)
	if err != nil {
		result = core.HookToolUseLifecycleResult{OK: false, Warnings: []string{"lifecycle_record_error:" + err.Error()}}
	}
	misdirectWarning := core.SourceCheckoutMisdirectWarning(req)
	if *jsonOut {
		return printJSON(map[string]any{
			"ok":                result.OK,
			"lifecycle":         result,
			"misdirect_warning": misdirectWarning,
		})
	}
	// B3 linter-as-gate: after an edit/write that touches .go files, surface a
	// deterministic gofmt failure as feedback — but ONLY on hosts that accept
	// PostToolUse additionalContext (Claude). Codex (which never gets
	// --host here) and the clean case keep the no-op schema so lifecycle
	// bookkeeping can never surface as a hook failure. LintEditedGoFiles is
	// fail-open and self-gates on .go paths, so no process is spawned for
	// non-Go edits, reads, or command tools.
	h := strings.TrimSpace(*host)
	if h == "claude" && doctarget.ToolUseMayMutateLifecycleFiles(tool, command) {
		feedbackParts := []string{}
		if misdirectWarning != "" {
			feedbackParts = append(feedbackParts, misdirectWarning)
		}
		if failed, feedback := lintgate.LintEditedGoFiles(parsedRepo, paths); failed {
			feedbackParts = append(feedbackParts, feedback)
		}
		if len(feedbackParts) > 0 {
			return printJSON(hookadapter.Resolve(h).FormatContext("PostToolUse", strings.Join(feedbackParts, "\n"), ""))
		}
	}
	return printJSON(map[string]any{})
}

func runHookPreCompact(args []string) error {
	fs := flag.NewFlagSet("hook pre-compact", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = hookinput.RepoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = ResolveTarget("")
	}
	result := core.BuildLifecyclePreCompactCapsule(parsedRepo)
	if *jsonOut {
		return printJSON(result)
	}
	// Codex PreCompact only accepts the stop-control shape; hookSpecificOutput
	// makes Codex report "invalid PreCompact hook JSON output". The capsule was
	// already persisted above, so host stdout can stay a no-op object.
	return printJSON(map[string]any{})
}
