package hookcli

import (
	"flag"
	"io"
	"os"
	"strings"

	"agent-harness/internal/core"
)

func runHookPostCompact(args []string) error {
	fs := flag.NewFlagSet("hook post-compact", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = repoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = ResolveTarget("")
	}
	result := core.BuildLifecyclePostCompactReminder(parsedRepo)
	if *jsonOut {
		return printJSON(result)
	}
	// Re-establish the project-doc catalog after compaction, alongside the
	// lifecycle reminder. Compaction drops the SessionStart catalog injection.
	cat := core.BuildProjectDocCatalogContext(parsedRepo)
	if hostOf(hostFlag) == "codex" {
		context := strings.TrimSpace(result.AdditionalContext)
		if context == "" && cat.ShouldInject {
			context = strings.TrimSpace(cat.UserView)
		}
		if context == "" {
			return printJSON(map[string]any{})
		}
		// Codex PostCompact accepts only the compact-control schema
		// (continue/stopReason/suppressOutput/systemMessage). Unlike
		// SessionStart, it rejects hookSpecificOutput/additionalContext.
		return printJSON(map[string]any{"systemMessage": context})
	}
	if cat.ShouldInject {
		return emitCatalogPayload("PostCompact", hostOf(hostFlag), cat, result.AdditionalContext)
	}
	return printJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostCompact",
			"additionalContext": result.AdditionalContext,
		},
	})
}

func runHookSessionStart(args []string) error {
	fs := flag.NewFlagSet("hook session-start", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	parsedRepo := strings.TrimSpace(*repo)
	if parsedRepo == "" {
		parsedRepo = repoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = ResolveTarget("")
	}
	cat := core.BuildProjectDocCatalogContext(parsedRepo)
	if *jsonOut {
		return printJSON(cat)
	}
	// On compaction Claude Code fires SessionStart with source=compact AND the
	// PostCompact hook; let PostCompact own that case to avoid double injection.
	if !cat.ShouldInject || sourceFromHookInput(stdin) == "compact" {
		return printJSON(map[string]any{
			"hookSpecificOutput": map[string]any{"hookEventName": "SessionStart"},
		})
	}
	return emitCatalogPayload("SessionStart", hostOf(hostFlag), cat, "")
}

// emitCatalogPayload writes the host-aware project-doc catalog injection. The
// model-facing additionalContext stays hidden on Claude Code (paired with a
// pretty systemMessage) while Codex renders additionalContext in its TUI, so the
// readable view is placed there and systemMessage is omitted. prefix, when set
// (PostCompact lifecycle reminder), is prepended to additionalContext.
func emitCatalogPayload(eventName, host string, cat core.ProjectDocCatalogContext, prefix string) error {
	additionalContext := cat.Compact
	if host == "codex" {
		additionalContext = cat.UserView
	}
	if strings.TrimSpace(prefix) != "" {
		additionalContext = prefix + "\n" + additionalContext
	}
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     eventName,
			"additionalContext": additionalContext,
		},
	}
	if host != "codex" && cat.UserView != "" {
		payload["systemMessage"] = cat.UserView
	}
	return printJSON(payload)
}

func hostOf(hostFlag *string) string {
	return strings.ToLower(strings.TrimSpace(*hostFlag))
}
