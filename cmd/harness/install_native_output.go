package main

import (
	"fmt"
	"path/filepath"

	"agent-harness/internal/port"
)

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
