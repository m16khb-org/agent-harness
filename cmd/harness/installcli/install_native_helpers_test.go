package installcli

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
		PrintNativeResult(result)
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
		PrintNativeResult(result)
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
	if got := PreferredShellRC(home); got != filepath.Join(home, ".bashrc") {
		t.Fatalf("bash shell rc = %q", got)
	}

	t.Setenv("SHELL", "/bin/unknown")
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("# existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := PreferredShellRC(home); got != zshrc {
		t.Fatalf("existing zsh rc = %q", got)
	}

	dryRunFile, err := AppendShellPathLinePlan(filepath.Join(home, ".profile"), true)
	if err != nil {
		t.Fatalf("dry-run append path failed: %v", err)
	}
	if !dryRunFile.WouldWrite || dryRunFile.Written || dryRunFile.Kind != "shell_path_rc" {
		t.Fatalf("unexpected dry-run shell path file: %#v", dryRunFile)
	}

	rcPath := filepath.Join(home, "nested", ".profile")
	writtenFile, err := AppendShellPathLinePlan(rcPath, false)
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
	if !strings.Contains(string(body), ShellPathRCMarker) || !ShellRCAlreadyAddsLocalBin(rcPath, home) {
		t.Fatalf("shell rc did not contain local bin marker:\n%s", string(body))
	}
}

func TestNativeActivationStepAcceptsOnlyInternalTransitionModes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		dryRun bool
		raw    string
		want   string
		valid  bool
	}{
		{name: "normal", valid: true},
		{name: "begin", raw: "begin", want: "begin", valid: true},
		{name: "seal", raw: "seal", want: "seal", valid: true},
		{name: "unknown", raw: "other"},
		{name: "dry_run_transition", dryRun: true, raw: "begin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nativeActivationStep(tc.dryRun, tc.raw)
			if tc.valid && (err != nil || got != tc.want) {
				t.Fatalf("step=%q err=%v", got, err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("invalid step accepted: %q", got)
			}
		})
	}
}
