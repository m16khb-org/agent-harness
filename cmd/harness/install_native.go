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

// installBinPath returns the absolute path of the running binary so registered
// MCP/hook commands work regardless of the caller's cwd (for example a brew
// install). It falls back to the repo bin layout only when os.Executable fails.
func installBinPath() string {
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe
	}
	return filepath.Join(harnessRoot(), "bin", "agent-harness")
}

func runInstallNative(args []string) error {
	fs := flag.NewFlagSet("install-native", flag.ContinueOnError)
	projectLocal := fs.Bool("project-local", false, "also write project-local .mcp.json/.claude settings and project skill links; default is user/global only")
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
	req := core.DefaultNativeInstallRequest(harnessRoot(), home, codexHome, installBinPath())
	req.ProjectLocal = *projectLocal
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
		fmt.Println("Dry-run plan for agent-harness native integrations:")
	} else {
		fmt.Println("Installed agent-harness native integrations:")
	}
	fmt.Printf("- mode: %s\n", mode)
	fmt.Printf("- binary: %s\n", result.BinPath)
	fmt.Printf("- Codex user skills: %s/skills/* -> %s/skills/*\n", result.CodexHome, result.Root)
	fmt.Printf("- Claude user skills: %s -> %s/skills/*\n", filepath.Join(result.Home, ".claude", "skills", "*"), result.Root)
	fmt.Printf("- Codex MCP config: %s\n", filepath.Join(result.CodexHome, "config.toml"))
	fmt.Printf("- Codex UserPromptSubmit hook: %s\n", filepath.Join(result.CodexHome, "hooks.json"))
	fmt.Printf("- Claude project MCP template: %s\n", filepath.Join(result.Root, "configs", "claude", "mcp.project.json"))
	fmt.Printf("- Codex MCP template: %s\n", filepath.Join(result.Root, "configs", "codex", "mcp.config.toml"))
	fmt.Printf("- Codex hook template: %s\n", filepath.Join(result.Root, "configs", "codex", "hooks.json"))
	if result.ProjectLocal {
		fmt.Printf("- Project-local Claude MCP config: %s\n", filepath.Join(result.Root, ".mcp.json"))
		fmt.Printf("- Project-local Claude skills: %s\n", filepath.Join(result.Root, ".claude", "skills", "*"))
	} else {
		fmt.Println("- Project-local repo files: unchanged by default; use --project-local only when you intentionally want repo-scoped files")
	}
	for _, message := range result.Messages {
		fmt.Printf("- %s\n", message)
	}
}
