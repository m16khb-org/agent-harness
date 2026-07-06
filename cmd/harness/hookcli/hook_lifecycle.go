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
	result, err := core.RecordLifecycleToolUse(core.HookToolUseLifecycleRequest{
		Repo:    parsedRepo,
		Tool:    tool,
		Paths:   paths,
		Command: command,
		Source:  "post-tool-use",
	})
	if err != nil {
		result = core.HookToolUseLifecycleResult{OK: false, Warnings: []string{"lifecycle_record_error:" + err.Error()}}
	}
	// Live skill-routing capture: when a Skill tool fires during a session-bound
	// IssueOps cycle, record (current phase, skill) so skill_routing_fidelity can
	// be scored against observed activation. Best-effort and fail-open — it never
	// affects the hook decision and no-ops without an active cycle. Recorded here
	// (PostToolUse, off the critical PreToolUse path) since the tool has fired.
	if strings.EqualFold(tool, "Skill") {
		core.AutoRecordSkillRouting(parsedRepo, hookinput.SkillFromHookInput(stdin))
	}
	if *jsonOut {
		return printJSON(map[string]any{
			"ok":        result.OK,
			"lifecycle": result,
		})
	}
	// B3 linter-as-gate: after an edit/write that touches .go files, surface a
	// deterministic gofmt failure as feedback — but ONLY on hosts that accept
	// PostToolUse additionalContext (Claude/Reasonix). Codex (which never gets
	// --host here) and the clean case keep the no-op schema so lifecycle
	// bookkeeping can never surface as a hook failure. LintEditedGoFiles is
	// fail-open and self-gates on .go paths, so no process is spawned for
	// non-Go edits, reads, or command tools.
	h := strings.TrimSpace(*host)
	if (h == "claude" || h == "reasonix") && doctarget.ToolUseMayMutateLifecycleFiles(tool, command) {
		if failed, feedback := lintgate.LintEditedGoFiles(parsedRepo, paths); failed {
			return printJSON(hookadapter.Resolve(h).FormatContext("PostToolUse", feedback, ""))
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
