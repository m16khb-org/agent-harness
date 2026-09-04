package installcli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type installInteractiveChoices struct {
	ProjectLocal bool
	DryRun       bool
	PathMode     string
}

func validateInteractiveInstallInput(stdin *os.File) error {
	if os.Getenv("ISSUEOPS_INSTALL_HELPER") == "1" {
		return nil
	}
	if !term.IsTerminal(int(stdin.Fd())) {
		return errors.New("--interactive requires a terminal stdin")
	}
	return nil
}

func promptInstallChoices(projectLocal, dryRun bool, pathMode string, in io.Reader, out io.Writer) (installInteractiveChoices, error) {
	choices := installInteractiveChoices{ProjectLocal: projectLocal, DryRun: dryRun, PathMode: pathMode}
	reader := bufio.NewReader(in)
	fprintf(out, "issueops install\n")
	fprintf(out, "Installs user-scope Codex/Claude skills, MCP config, hooks, and the issueops command shim.\n")
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
	fprintf(out, "  1) auto   Create ~/.local/bin/issueops and add ~/.local/bin to your shell rc. Recommended.\n")
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
