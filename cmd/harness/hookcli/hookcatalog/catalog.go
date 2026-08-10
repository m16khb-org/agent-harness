package hookcatalog

import (
	"flag"
	"io"
	"os"
	"strings"

	"agent-harness/cmd/harness/hookcli/hookinput"
	hookadapter "agent-harness/internal/domain/hook"
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
	cat := BuildProjectDocCatalogContext(parsedRepo)
	if *jsonOut {
		return config.PrintJSON(cat)
	}

	// Codex post-compact uses systemMessage only.
	if hostOf(hostFlag) == "codex" {
		if !cat.ShouldInject {
			return config.PrintJSON(map[string]any{})
		}
		return config.PrintJSON(map[string]any{"systemMessage": cat.UserView})
	}

	// Non-Codex post-compact: default host is Claude for catalog hooks.
	ho := resolveCatalogHost(hostFlag)
	if cat.ShouldInject {
		return config.PrintJSON(ho.FormatContext("PostCompact", cat.Compact, cat.UserView))
	}

	return config.PrintJSON(map[string]any{})
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
	cat := BuildProjectDocCatalogContext(parsedRepo)
	if *jsonOut {
		return config.PrintJSON(cat)
	}
	// Default host for catalog hooks is Claude (not Codex).
	ho := resolveCatalogHost(hostFlag)
	if hookinput.SourceFromHookInput(stdin) == "compact" {
		return config.PrintJSON(ho.FormatContext("SessionStart", "", ""))
	}
	if !cat.ShouldInject {
		return config.PrintJSON(ho.FormatContext("SessionStart", "", ""))
	}
	return config.PrintJSON(ho.FormatContext("SessionStart", cat.Compact, cat.UserView))
}

// resolveCatalogHost returns the hook output adapter for catalog hooks.
// Catalog hooks (SessionStart, PostCompact) default to Claude, not Codex.
func resolveCatalogHost(hostFlag *string) hookadapter.HostHookOutput {
	switch hostOf(hostFlag) {
	case "codex":
		return hookadapter.CodexHookOutput{}
	default:
		return hookadapter.ClaudeHookOutput{}
	}
}

// joinContext concatenates a prefix context with a catalog context.
func hostOf(hostFlag *string) string {
	return strings.ToLower(strings.TrimSpace(*hostFlag))
}
