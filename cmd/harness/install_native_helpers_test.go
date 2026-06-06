package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestPrintInstallNativeResultCoversDryRunAndProjectLocalModes(t *testing.T) {
	result := port.NativeInstallResult{
		OK:           true,
		Root:         "/repo/harness",
		Home:         "/home/user",
		CodexHome:    "/home/user/.codex",
		BinPath:      "/repo/harness/bin/agent-harness",
		ProjectLocal: false,
		DryRun:       true,
		Messages:     []string{"planned install"},
	}

	dryRunOut := captureStatusVerifyStdout(t, func() error {
		printInstallNativeResult(result)
		return nil
	})
	for _, want := range []string{
		"Dry-run plan for agent-harness native integrations:",
		"- mode: user/global only",
		"- Project-local repo files: unchanged by default",
		"- planned install",
	} {
		if !strings.Contains(dryRunOut, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, dryRunOut)
		}
	}

	result.ProjectLocal = true
	result.DryRun = false
	installedOut := captureStatusVerifyStdout(t, func() error {
		printInstallNativeResult(result)
		return nil
	})
	for _, want := range []string{
		"Installed agent-harness native integrations:",
		"- mode: user/global + explicit project-local",
		"- Project-local Claude MCP config: /repo/harness/.mcp.json",
		"- Project-local Claude skills: /repo/harness/.claude/skills/*",
	} {
		if !strings.Contains(installedOut, want) {
			t.Fatalf("installed output missing %q:\n%s", want, installedOut)
		}
	}
}

func TestPreferredShellRCAndAppendShellPathLinePlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SHELL", "/bin/bash")
	if got := preferredShellRC(home); got != filepath.Join(home, ".bashrc") {
		t.Fatalf("bash shell rc = %q", got)
	}

	t.Setenv("SHELL", "/bin/unknown")
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("# existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := preferredShellRC(home); got != zshrc {
		t.Fatalf("existing zsh rc = %q", got)
	}

	dryRunFile, err := appendShellPathLinePlan(filepath.Join(home, ".profile"), true)
	if err != nil {
		t.Fatalf("dry-run append path failed: %v", err)
	}
	if !dryRunFile.WouldWrite || dryRunFile.Written || dryRunFile.Kind != "shell_path_rc" {
		t.Fatalf("unexpected dry-run shell path file: %#v", dryRunFile)
	}

	rcPath := filepath.Join(home, "nested", ".profile")
	writtenFile, err := appendShellPathLinePlan(rcPath, false)
	if err != nil {
		t.Fatalf("append path failed: %v", err)
	}
	if !writtenFile.Written || writtenFile.WouldWrite {
		t.Fatalf("unexpected written shell path file: %#v", writtenFile)
	}
	body, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), shellPathRCMarker) || !shellRCAlreadyAddsLocalBin(rcPath, home) {
		t.Fatalf("shell rc did not contain local bin marker:\n%s", string(body))
	}
}
