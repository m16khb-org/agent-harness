package installcli

import (
	"fmt"
	"path/filepath"

	"issueops/internal/port"
)

func printInstallNativeResult(result port.NativeInstallResult) {
	mode := "user/global only"
	if result.ProjectLocal {
		mode = "user/global + explicit project-local"
	}
	if result.DryRun {
		fmt.Println("Dry-run plan for issueops native integrations:")
	} else {
		fmt.Println("Installed issueops native integrations:")
	}
	fmt.Printf("- mode: %s\n", mode)
	fmt.Printf("- binary: %s\n", result.BinPath)
	if command := result.CommandPath; command != nil {
		state := "approved"
		if command.WouldAdopt {
			state = "would adopt"
		} else if command.RolledBack {
			state = "rolled back"
		} else if command.Committed {
			state = "committed"
		} else if command.Adopted {
			state = "adopted pending activation seal"
		}
		fmt.Printf("- managed command adoption: %s -> %s (%s)\n", command.Path, command.Target, state)
		if command.BackupPath != "" {
			fmt.Printf("- managed command backup/recovery path: %s (rollback_available=%t, backup_retained=%t)\n", command.BackupPath, command.RollbackAvailable, command.BackupRetained)
		}
	}
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
