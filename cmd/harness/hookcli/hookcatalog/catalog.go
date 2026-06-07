package hookcatalog

import (
	"flag"
	"io"
	"os"
	"strings"

	"agent-harness/cmd/harness/hookcli/hookinput"
	"agent-harness/internal/core"
)

type Config struct {
	ResolveTarget func(string) string
	PrintJSON     func(any) error
}

func RunPostCompact(args []string, config Config) error {
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
		parsedRepo = hookinput.RepoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = config.ResolveTarget("")
	}
	result := core.BuildLifecyclePostCompactReminder(parsedRepo)
	if *jsonOut {
		return config.PrintJSON(result)
	}
	cat := core.BuildProjectDocCatalogContext(parsedRepo)
	if hostOf(hostFlag) == "codex" {
		context := strings.TrimSpace(result.AdditionalContext)
		if context == "" && cat.ShouldInject {
			context = strings.TrimSpace(cat.UserView)
		}
		if context == "" {
			return config.PrintJSON(map[string]any{})
		}
		return config.PrintJSON(map[string]any{"systemMessage": context})
	}
	if cat.ShouldInject {
		return emitCatalogPayload("PostCompact", hostOf(hostFlag), cat, result.AdditionalContext, config.PrintJSON)
	}
	return config.PrintJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostCompact",
			"additionalContext": result.AdditionalContext,
		},
	})
}

func RunSessionStart(args []string, config Config) error {
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
		parsedRepo = hookinput.RepoFromHookInput(stdin)
	}
	if parsedRepo == "" {
		parsedRepo = config.ResolveTarget("")
	}
	cat := core.BuildProjectDocCatalogContext(parsedRepo)
	if *jsonOut {
		return config.PrintJSON(cat)
	}
	if !cat.ShouldInject || hookinput.SourceFromHookInput(stdin) == "compact" {
		return config.PrintJSON(map[string]any{
			"hookSpecificOutput": map[string]any{"hookEventName": "SessionStart"},
		})
	}
	return emitCatalogPayload("SessionStart", hostOf(hostFlag), cat, "", config.PrintJSON)
}

func emitCatalogPayload(eventName, host string, cat core.ProjectDocCatalogContext, prefix string, printJSON func(any) error) error {
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
