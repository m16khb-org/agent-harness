package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	assets "agent-harness"
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
		return stableExecutablePath(exe)
	}
	return filepath.Join(harnessRoot(), "bin", "agent-harness")
}

// stableExecutablePath maps a version-pinned Homebrew Cellar path back to the
// stable `<prefix>/bin/<name>` symlink, which brew keeps pointing at the current
// version. Without this, a registered command captured from a Cellar path would
// break after `brew upgrade` (the old versioned directory is removed). Paths that
// are not under a Cellar directory, or whose bin symlink is absent, are returned
// unchanged.
func stableExecutablePath(exe string) string {
	idx := strings.Index(exe, "/Cellar/")
	if idx < 0 {
		return exe
	}
	stable := filepath.Join(exe[:idx], "bin", filepath.Base(exe))
	if info, err := os.Lstat(stable); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return stable
	}
	return exe
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
	root := harnessRoot()
	req := core.DefaultNativeInstallRequest(root, home, codexHome, installBinPath())
	req.ProjectLocal = *projectLocal
	req.DryRun = *dryRun
	// No repository skills (packaged binary, e.g. brew): install from embedded
	// assets. The signal is "no checkout skills found", which includes both a
	// read error and an existing-but-empty skills directory.
	embedMode := false
	if names, err := core.ListSkillNames(root); err != nil || len(names) == 0 {
		if embeddedNames, nerr := assets.SkillNames(); nerr == nil && len(embeddedNames) > 0 {
			if skillsFS, ferr := assets.SkillsFS(); ferr == nil {
				req.EmbeddedSkills = skillsFS
				req.SkillNames = embeddedNames
				embedMode = true
			}
		}
	}
	result, err := core.InstallNative(req, codexadapter.NewInstaller(), claudeadapter.NewInstaller())
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printInstallNativeResult(result, embedMode)
	return err
}

func printInstallNativeResult(result port.NativeInstallResult, embedMode bool) {
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
	if embedMode {
		fmt.Printf("- source: embedded assets (no repository checkout)\n")
		fmt.Printf("- Codex user skills: copied to %s\n", filepath.Join(result.CodexHome, "skills", "*"))
		fmt.Printf("- Claude user skills: copied to %s\n", filepath.Join(result.Home, ".claude", "skills", "*"))
	} else {
		fmt.Printf("- Codex user skills: %s/skills/* -> %s/skills/*\n", result.CodexHome, result.Root)
		fmt.Printf("- Claude user skills: %s -> %s/skills/*\n", filepath.Join(result.Home, ".claude", "skills", "*"), result.Root)
	}
	fmt.Printf("- Codex MCP config: %s\n", filepath.Join(result.CodexHome, "config.toml"))
	fmt.Printf("- Codex UserPromptSubmit hook: %s\n", filepath.Join(result.CodexHome, "hooks.json"))
	if !embedMode {
		fmt.Printf("- Claude project MCP template: %s\n", filepath.Join(result.Root, "configs", "claude", "mcp.project.json"))
		fmt.Printf("- Codex MCP template: %s\n", filepath.Join(result.Root, "configs", "codex", "mcp.config.toml"))
		fmt.Printf("- Codex hook template: %s\n", filepath.Join(result.Root, "configs", "codex", "hooks.json"))
	}
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
