package hookcatalog

import (
	"flag"
	"io"
	"os"
	"strings"

	"issueops/cmd/issueops/hookcli/hookinput"
	hookadapter "issueops/internal/domain/hook"
)

type Config struct {
	ResolveTarget func(string) string
	PrintJSON     func(any) error
}

// RunSessionStart renders the static project-doc catalog for every SessionStart
// source, including "compact". Claude Code 2.1.247 and Codex 0.150.1 both re-run
// SessionStart with source "compact" after compaction, and on both hosts only
// SessionStart output can carry model-facing additionalContext (verified against
// the installed binaries on 2026-08-27), so the catalog is re-established here
// rather than on PostCompact.
func RunSessionStart(args []string, config Config) error {
	fs := flag.NewFlagSet("hook session-start", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	cat := BuildProjectDocCatalogContext(resolveRepo(*repo, stdin, config))
	if *jsonOut {
		return config.PrintJSON(cat)
	}
	ho := resolveCatalogHost(hostFlag)
	if !cat.ShouldInject {
		return config.PrintJSON(ho.FormatNoop())
	}
	return config.PrintJSON(ho.FormatContext("SessionStart", cat.Compact, cat.UserView))
}

// RunPostCompact keeps an explicit post-compaction catalog surface for hosts whose
// compaction event has no SessionStart re-run (Omo session_compact reads --json)
// and for diagnosis. Claude and Codex default installs do not register it: both
// hosts accept only user-facing output on PostCompact (Claude renders the raw
// stdout as a display message, Codex's post-compact.command.output schema has no
// hookSpecificOutput), so the host shape carries the readable catalog through
// systemMessage only.
func RunPostCompact(args []string, config Config) error {
	fs := flag.NewFlagSet("hook post-compact", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	cat := BuildProjectDocCatalogContext(resolveRepo(*repo, stdin, config))
	if *jsonOut {
		return config.PrintJSON(cat)
	}
	if !cat.ShouldInject {
		return config.PrintJSON(resolveCatalogHost(hostFlag).FormatNoop())
	}
	return config.PrintJSON(map[string]any{"systemMessage": cat.UserView})
}

func resolveRepo(flagValue string, stdin []byte, config Config) string {
	repo := strings.TrimSpace(flagValue)
	if repo == "" {
		repo = hookinput.RepoFromHookInput(stdin)
	}
	if repo == "" {
		repo = config.ResolveTarget("")
	}
	return repo
}

// resolveCatalogHost returns the hook output adapter for catalog hooks.
// Catalog hooks default to Claude, not Codex.
func resolveCatalogHost(hostFlag *string) hookadapter.HostHookOutput {
	switch hostOf(hostFlag) {
	case "codex":
		return hookadapter.CodexHookOutput{}
	default:
		return hookadapter.ClaudeHookOutput{}
	}
}

func hostOf(hostFlag *string) string {
	return strings.ToLower(strings.TrimSpace(*hostFlag))
}
