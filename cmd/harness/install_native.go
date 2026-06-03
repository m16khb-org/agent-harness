package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	claudeadapter "agent-harness/internal/adapter/claude"
	codexadapter "agent-harness/internal/adapter/codex"
	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/core"
	"agent-harness/internal/port"
)

const shellPathRCMarker = "# agent-harness: add user-local bin to PATH"

func runInstall(args []string) error {
	return runInstallCommand("install", args)
}

func runInstallNative(args []string) error {
	return runInstallCommand("install-native", args)
}

func runInstallCommand(commandName string, args []string) error {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	projectLocal := fs.Bool("project-local", false, "also write project-local .mcp.json/.claude settings and project skill links; default is user/global only")
	dryRun := fs.Bool("dry-run", false, "plan files and links without writing them")
	pathMode := fs.String("path-mode", "auto", "manage ~/.local/bin PATH setup: auto, manual, or skip")
	interactive := fs.Bool("interactive", false, "ask for install choices before applying the plan")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *interactive {
		choices, err := promptInstallChoices(*projectLocal, *dryRun, *pathMode, os.Stdin, os.Stderr)
		if err != nil {
			return err
		}
		*projectLocal = choices.ProjectLocal
		*dryRun = choices.DryRun
		*pathMode = choices.PathMode
	}
	if !validInstallPathMode(*pathMode) {
		return fmt.Errorf("invalid --path-mode %q: expected auto, manual, or skip", *pathMode)
	}
	home, _ := os.UserHomeDir()
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" && home != "" {
		codexHome = filepath.Join(home, ".codex")
	}
	req := core.DefaultNativeInstallRequest(harnessRoot(), home, codexHome, filepath.Join(harnessRoot(), "bin", "agent-harness"))
	req.ProjectLocal = *projectLocal
	req.DryRun = *dryRun
	result, err := core.InstallNative(req, codexadapter.NewInstaller(), claudeadapter.NewInstaller())
	if pathErr := applyInstallPathPlan(&result, req, *pathMode); pathErr != nil {
		result.OK = false
		err = errors.Join(err, pathErr)
	}
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

func validInstallPathMode(mode string) bool {
	switch mode {
	case "auto", "manual", "skip":
		return true
	default:
		return false
	}
}

type installInteractiveChoices struct {
	ProjectLocal bool
	DryRun       bool
	PathMode     string
}

func promptInstallChoices(projectLocal, dryRun bool, pathMode string, in io.Reader, out io.Writer) (installInteractiveChoices, error) {
	choices := installInteractiveChoices{ProjectLocal: projectLocal, DryRun: dryRun, PathMode: pathMode}
	reader := bufio.NewReader(in)
	fprintf(out, "agent-harness install\n")
	fprintf(out, "Installs user-scope Codex/Claude skills, MCP config, hooks, and the agent-harness command shim.\n")
	fprintf(out, "\n")
	fprintf(out, "Project-local files write .mcp.json and .claude/ links into this harness repo. Most installs should keep this disabled.\n")
	projectAnswer, err := promptLine(reader, out, "Enable project-local files? [y/N]: ")
	if err != nil {
		return choices, err
	}
	if strings.TrimSpace(projectAnswer) != "" {
		choices.ProjectLocal = yesAnswer(projectAnswer)
	}
	fprintf(out, "\nPATH setup:\n")
	fprintf(out, "  1) auto   Create ~/.local/bin/agent-harness and add ~/.local/bin to your shell rc. Recommended.\n")
	fprintf(out, "  2) manual Create the command shim and print the export command; you edit your shell rc.\n")
	fprintf(out, "  3) skip   Create the command shim but skip shell rc edits.\n")
	pathAnswer, err := promptLine(reader, out, "Select PATH setup [1]: ")
	if err != nil {
		return choices, err
	}
	switch strings.TrimSpace(strings.ToLower(pathAnswer)) {
	case "", "1", "auto", "a":
		choices.PathMode = "auto"
	case "2", "manual", "m":
		choices.PathMode = "manual"
	case "3", "skip", "s":
		choices.PathMode = "skip"
	default:
		return choices, fmt.Errorf("invalid PATH setup choice %q", strings.TrimSpace(pathAnswer))
	}
	if !dryRun {
		applyAnswer, err := promptLine(reader, out, "Apply changes now? [Y/n]: ")
		if err != nil {
			return choices, err
		}
		if strings.TrimSpace(applyAnswer) != "" && !yesAnswer(applyAnswer) {
			choices.DryRun = true
		}
	}
	return choices, nil
}

func promptLine(reader *bufio.Reader, out io.Writer, prompt string) (string, error) {
	_, _ = io.WriteString(out, prompt)
	line, err := reader.ReadString('\n')
	if errors.Is(err, io.EOF) {
		if strings.TrimSpace(line) == "" {
			return "", fmt.Errorf("interactive input ended before %s", strings.TrimSpace(prompt))
		}
		return strings.TrimSpace(line), nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func yesAnswer(answer string) bool {
	switch strings.TrimSpace(strings.ToLower(answer)) {
	case "y", "yes", "1", "true", "on":
		return true
	default:
		return false
	}
}

func fprintf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func applyInstallPathPlan(result *port.NativeInstallResult, req port.NativeInstallRequest, mode string) error {
	userBin := filepath.Join(req.Home, ".local", "bin")
	commandPath := filepath.Join(userBin, "agent-harness")
	link, err := installutil.EnsureSymlinkPlan(req.BinPath, commandPath, req.DryRun)
	result.Links = append(result.Links, link)
	if err != nil {
		return err
	}
	if mode == "manual" {
		result.Messages = append(result.Messages, `path-mode=manual: command shim is planned; run export PATH="$HOME/.local/bin:$PATH" for this shell or add it to your shell rc`)
		return nil
	}
	if mode == "skip" {
		result.Messages = append(result.Messages, "path-mode=skip: shell rc PATH update skipped; command shim still uses "+commandPath)
		return nil
	}
	if localBinInPath(req.Home) {
		return nil
	}
	rcPath := preferredShellRC(req.Home)
	if shellRCAlreadyAddsLocalBin(rcPath, req.Home) {
		return nil
	}
	file, err := appendShellPathLinePlan(rcPath, req.DryRun)
	if file.Path != "" && (file.WouldWrite || file.Written) {
		result.Files = append(result.Files, file)
	}
	if err != nil {
		return err
	}
	if file.WouldWrite {
		result.Messages = append(result.Messages, "dry-run: would add ~/.local/bin to PATH in "+rcPath)
	} else if file.Written {
		result.Messages = append(result.Messages, `added ~/.local/bin to PATH in `+rcPath+`; restart shell or run: export PATH="$HOME/.local/bin:$PATH"`)
	}
	return nil
}

func preferredShellRC(home string) string {
	switch filepath.Base(os.Getenv("SHELL")) {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		return filepath.Join(home, ".bashrc")
	}
	for _, name := range []string{".zshrc", ".bashrc"} {
		path := filepath.Join(home, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(home, ".profile")
}

func localBinInPath(home string) bool {
	localBin := filepath.Clean(filepath.Join(home, ".local", "bin"))
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(entry) == localBin {
			return true
		}
	}
	return false
}

func shellRCAlreadyAddsLocalBin(path, home string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(b)
	return strings.Contains(text, shellPathRCMarker) ||
		strings.Contains(text, `export PATH="$HOME/.local/bin:$PATH"`) ||
		strings.Contains(text, `export PATH="`+filepath.Join(home, ".local", "bin")+`:$PATH"`)
}

func appendShellPathLinePlan(path string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "shell_path_rc"}
	if dryRun {
		file.WouldWrite = true
		return file, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return file, err
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "\n%s\n%s\n", shellPathRCMarker, `export PATH="$HOME/.local/bin:$PATH"`); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}
