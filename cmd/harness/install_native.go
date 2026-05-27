package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	claudeadapter "agent-harness/internal/adapter/claude"
	codexadapter "agent-harness/internal/adapter/codex"
	"agent-harness/internal/core"
	"agent-harness/internal/port"
)

func runInstallNative(args []string) error {
	fs := flag.NewFlagSet("install-native", flag.ContinueOnError)
	llmWikiRoot := fs.String("llm-wiki-root", "", "llm-wiki root; defaults to ~/workspace/knowledge-base/llm-wiki")
	projectLocal := fs.Bool("project-local", false, "also write project-local .mcp.json/.claude settings and project skill links; default is user/global only")
	noClaudeUserHook := fs.Bool("no-claude-user-hook", false, "skip merging the Claude user SessionStart hook")
	dryRun := fs.Bool("dry-run", false, "plan files and links without writing them")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" && home != "" {
		codexHome = filepath.Join(home, ".codex")
	}
	req := core.DefaultNativeInstallRequest(harnessRoot(), home, codexHome, filepath.Join(harnessRoot(), "bin", "harness"), *llmWikiRoot)
	req.ProjectLocal = *projectLocal
	req.ClaudeUserHook = !*noClaudeUserHook
	req.DryRun = *dryRun
	result, err := core.InstallNative(req, codexadapter.NewInstaller(), claudeadapter.NewInstaller())
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printInstallNativeResult(result)
	return err
}

func printInstallNativeResult(result port.NativeInstallResult) {
	mode := "user/global only"
	if result.ProjectLocal {
		mode = "user/global + explicit project-local"
	}
	if result.DryRun {
		fmt.Println("Dry-run plan for agent harness native integrations:")
	} else {
		fmt.Println("Installed agent harness native integrations:")
	}
	fmt.Printf("- mode: %s\n", mode)
	fmt.Printf("- binary: %s\n", result.BinPath)
	fmt.Printf("- LLM Wiki root: %s\n", result.LLMWikiRoot)
	fmt.Printf("- Codex user skills: %s/skills/* -> %s/skills/*\n", result.CodexHome, result.Root)
	fmt.Printf("- Claude user skills: %s -> %s/skills/*\n", filepath.Join(result.Home, ".claude", "skills", "*"), result.Root)
	fmt.Printf("- Codex MCP config: %s\n", filepath.Join(result.CodexHome, "config.toml"))
	fmt.Printf("- Claude user SessionStart hook: %s\n", filepath.Join(result.Home, ".claude", "settings.json"))
	fmt.Printf("- Claude project MCP template: %s\n", filepath.Join(result.Root, "configs", "claude", "mcp.project.json"))
	fmt.Printf("- Codex MCP template: %s\n", filepath.Join(result.Root, "configs", "codex", "mcp.config.toml"))
	fmt.Printf("- LLM Wiki hook helper: %s\n", filepath.Join(result.Root, "scripts", "session-start-llm-wiki.sh"))
	if result.ProjectLocal {
		fmt.Printf("- Project-local Claude MCP config: %s\n", filepath.Join(result.Root, ".mcp.json"))
		fmt.Printf("- Project-local Claude skills: %s\n", filepath.Join(result.Root, ".claude", "skills", "*"))
	} else {
		fmt.Println("- Project-local repo files: not written by default; use --project-local only when you intentionally want repo-scoped files")
	}
}
