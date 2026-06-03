package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestInstallCommandDryRunJSONDispatches(t *testing.T) {
	if os.Getenv("AGENT_HARNESS_INSTALL_HELPER") == "1" {
		return
	}
	home := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=TestInstallCommandDryRunJSONDispatchHelper", "--", "install", "--dry-run", "--json")
	cmd.Env = mergeEnvOverrides(os.Environ(), []string{
		"AGENT_HARNESS_INSTALL_HELPER=1",
		"HOME=" + home,
		"CODEX_HOME=" + filepath.Join(home, ".codex"),
		"HARNESS_ROOT=" + harnessRoot(),
	})
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install --dry-run --json failed: %v\n%s", err, string(out))
	}
	var result port.NativeInstallResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("install --dry-run --json returned invalid JSON: %v\n%s", err, string(out))
	}
	if !result.OK || !result.DryRun {
		t.Fatalf("install dry-run result mismatch: ok=%v dry_run=%v result=%+v", result.OK, result.DryRun, result)
	}
	if result.Root != harnessRoot() {
		t.Fatalf("install used unexpected harness root: got %q want %q", result.Root, harnessRoot())
	}
}

func TestInstallCommandDryRunAutoPathModePlansShimAndShellRC(t *testing.T) {
	if os.Getenv("AGENT_HARNESS_INSTALL_HELPER") == "1" {
		return
	}
	home := t.TempDir()
	result := runInstallDryRunJSON(t, home, "install", "auto")
	if !hasInstallLink(result.Links, filepath.Join(home, ".local", "bin", "agent-harness"), filepath.Join(harnessRoot(), "bin", "agent-harness"), true) {
		t.Fatalf("auto path mode did not plan command shim link: %+v", result.Links)
	}
	if !hasInstallFile(result.Files, filepath.Join(home, ".zshrc"), "shell_path_rc", true) {
		t.Fatalf("auto path mode did not plan shell rc PATH write: %+v", result.Files)
	}
}

func TestInstallCommandDryRunManualPathModePlansShimWithoutShellRC(t *testing.T) {
	if os.Getenv("AGENT_HARNESS_INSTALL_HELPER") == "1" {
		return
	}
	home := t.TempDir()
	result := runInstallDryRunJSON(t, home, "install", "manual")
	if !hasInstallLink(result.Links, filepath.Join(home, ".local", "bin", "agent-harness"), filepath.Join(harnessRoot(), "bin", "agent-harness"), true) {
		t.Fatalf("manual path mode did not plan command shim link: %+v", result.Links)
	}
	if hasInstallFileKind(result.Files, "shell_path_rc") {
		t.Fatalf("manual path mode unexpectedly planned shell rc write: %+v", result.Files)
	}
	if !messagesContain(result.Messages, `export PATH="$HOME/.local/bin:$PATH"`) {
		t.Fatalf("manual path mode did not include PATH export guidance: %+v", result.Messages)
	}
}

func TestInstallCommandInteractiveDryRunSelectsProjectLocalAndManualPathMode(t *testing.T) {
	if os.Getenv("AGENT_HARNESS_INSTALL_HELPER") == "1" {
		return
	}
	home := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=TestInstallCommandDryRunJSONDispatchHelper", "--", "install", "--interactive", "--dry-run", "--json")
	cmd.Env = mergeEnvOverrides(os.Environ(), []string{
		"AGENT_HARNESS_INSTALL_HELPER=1",
		"HOME=" + home,
		"CODEX_HOME=" + filepath.Join(home, ".codex"),
		"HARNESS_ROOT=" + harnessRoot(),
		"SHELL=/bin/zsh",
		"PATH=/usr/bin:/bin",
	})
	cmd.Stdin = strings.NewReader("y\n2\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("interactive install dry-run failed: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Select PATH setup") {
		t.Fatalf("interactive install did not print PATH setup prompt to stderr:\n%s", stderr.String())
	}
	var result port.NativeInstallResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("interactive install returned invalid JSON: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if !result.ProjectLocal {
		t.Fatalf("interactive install did not apply project-local choice: %+v", result)
	}
	if hasInstallFileKind(result.Files, "shell_path_rc") {
		t.Fatalf("manual path mode unexpectedly planned shell rc write: %+v", result.Files)
	}
	if !messagesContain(result.Messages, `export PATH="$HOME/.local/bin:$PATH"`) {
		t.Fatalf("manual path mode did not include PATH export guidance: %+v", result.Messages)
	}
}

func TestInstallNativeAliasAcceptsPathMode(t *testing.T) {
	if os.Getenv("AGENT_HARNESS_INSTALL_HELPER") == "1" {
		return
	}
	result := runInstallDryRunJSON(t, t.TempDir(), "install-native", "manual")
	if !messagesContain(result.Messages, `export PATH="$HOME/.local/bin:$PATH"`) {
		t.Fatalf("install-native alias did not honor --path-mode=manual: %+v", result.Messages)
	}
}

func TestInstallCommandDryRunSkipPathModeDoesNotPlanShellRC(t *testing.T) {
	if os.Getenv("AGENT_HARNESS_INSTALL_HELPER") == "1" {
		return
	}
	home := t.TempDir()
	result := runInstallDryRunJSON(t, home, "install", "skip")
	if hasInstallFileKind(result.Files, "shell_path_rc") {
		t.Fatalf("skip path mode unexpectedly planned shell rc write: %+v", result.Files)
	}
}

func TestInstallCommandDryRunJSONDispatchHelper(t *testing.T) {
	if os.Getenv("AGENT_HARNESS_INSTALL_HELPER") != "1" {
		return
	}
	os.Args = append([]string{"agent-harness"}, os.Args[3:]...)
	main()
	os.Exit(0)
}

func TestInstallNativeAliasKeepsUsageName(t *testing.T) {
	if os.Getenv("AGENT_HARNESS_INSTALL_HELPER") == "1" {
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestInstallCommandDryRunJSONDispatchHelper", "--", "install-native", "--badflag")
	cmd.Env = mergeEnvOverrides(os.Environ(), []string{
		"AGENT_HARNESS_INSTALL_HELPER=1",
		"HARNESS_ROOT=" + harnessRoot(),
	})
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("install-native --badflag unexpectedly succeeded:\n%s", string(out))
	}
	text := string(out)
	if !strings.Contains(text, "Usage of install-native:") {
		t.Fatalf("install-native alias did not keep usage name:\n%s", text)
	}
	if strings.Contains(text, "Usage of install:") {
		t.Fatalf("install-native alias leaked install usage name:\n%s", text)
	}
}

func runInstallDryRunJSON(t *testing.T, home, commandName, pathMode string) port.NativeInstallResult {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestInstallCommandDryRunJSONDispatchHelper", "--", commandName, "--dry-run", "--json", "--path-mode="+pathMode)
	cmd.Env = mergeEnvOverrides(os.Environ(), []string{
		"AGENT_HARNESS_INSTALL_HELPER=1",
		"HOME=" + home,
		"CODEX_HOME=" + filepath.Join(home, ".codex"),
		"HARNESS_ROOT=" + harnessRoot(),
		"SHELL=/bin/zsh",
		"PATH=/usr/bin:/bin",
	})
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s --dry-run --json --path-mode=%s failed: %v\n%s", commandName, pathMode, err, string(out))
	}
	var result port.NativeInstallResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("install dry-run returned invalid JSON: %v\n%s", err, string(out))
	}
	return result
}

func hasInstallLink(links []port.InstallLink, path, target string, wouldCreate bool) bool {
	for _, link := range links {
		if link.Path == path && link.Target == target && link.WouldCreate == wouldCreate {
			return true
		}
	}
	return false
}

func hasInstallFile(files []port.InstallFile, path, kind string, wouldWrite bool) bool {
	for _, file := range files {
		if file.Path == path && file.Kind == kind && file.WouldWrite == wouldWrite {
			return true
		}
	}
	return false
}

func hasInstallFileKind(files []port.InstallFile, kind string) bool {
	for _, file := range files {
		if file.Kind == kind {
			return true
		}
	}
	return false
}

func messagesContain(messages []string, needle string) bool {
	for _, message := range messages {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}
