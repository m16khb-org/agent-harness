package installcli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestInstallCommandDryRunJSONDispatches(t *testing.T) {
	home := t.TempDir()
	root := configureInstallCommandTest(t, home)

	out, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstallCommand("install", []string{"--dry-run", "--json"})
	})
	if err != nil {
		t.Fatalf("install --dry-run --json failed: %v\n%s", err, out)
	}
	var result port.NativeInstallResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("install --dry-run --json returned invalid JSON: %v\n%s", err, out)
	}
	if !result.OK || !result.DryRun {
		t.Fatalf("install dry-run result mismatch: ok=%v dry_run=%v result=%+v", result.OK, result.DryRun, result)
	}
	if result.Root != root {
		t.Fatalf("install used unexpected harness root: got %q want %q", result.Root, root)
	}
}

func TestInstallCommandDryRunAutoPathModePlansShimAndShellRC(t *testing.T) {
	home := t.TempDir()
	root := configureInstallCommandTest(t, home)
	result := runInstallDryRunJSON(t, home, "install", "auto")
	if !hasInstallLink(result.Links, filepath.Join(home, ".local", "bin", "agent-harness"), filepath.Join(root, "bin", "agent-harness"), true) {
		t.Fatalf("auto path mode did not plan command shim link: %+v", result.Links)
	}
	if !hasInstallFile(result.Files, filepath.Join(home, ".zshrc"), "shell_path_rc", true) {
		t.Fatalf("auto path mode did not plan shell rc PATH write: %+v", result.Files)
	}
}

func TestInstallCommandDryRunManualPathModePlansShimWithoutShellRC(t *testing.T) {
	home := t.TempDir()
	root := configureInstallCommandTest(t, home)
	result := runInstallDryRunJSON(t, home, "install", "manual")
	if !hasInstallLink(result.Links, filepath.Join(home, ".local", "bin", "agent-harness"), filepath.Join(root, "bin", "agent-harness"), true) {
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
	home := t.TempDir()
	configureInstallCommandTest(t, home)
	t.Setenv("AGENT_HARNESS_INSTALL_HELPER", "1")

	stdout, stderr, err := captureInstallCommandOutput(t, strings.NewReader("y\n2\n"), func() error {
		return RunInstallCommand("install", []string{"--interactive", "--dry-run", "--json"})
	})
	if err != nil {
		t.Fatalf("interactive install dry-run failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "Select PATH setup") {
		t.Fatalf("interactive install did not print PATH setup prompt to stderr:\n%s", stderr)
	}
	var result port.NativeInstallResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("interactive install returned invalid JSON: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
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

func TestValidateInteractiveInstallInputRejectsPipe(t *testing.T) {
	t.Setenv("AGENT_HARNESS_INSTALL_HELPER", "")
	stdin, stdout, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	defer stdout.Close()
	if err := ValidateInteractiveInput(stdin); err == nil || !strings.Contains(err.Error(), "requires a terminal") {
		t.Fatalf("validateInteractiveInstallInput error = %v, want terminal requirement", err)
	}
}

func TestInstallNativeAliasAcceptsPathMode(t *testing.T) {
	home := t.TempDir()
	configureInstallCommandTest(t, home)
	result := runInstallDryRunJSON(t, home, "install-native", "manual")
	if !messagesContain(result.Messages, `export PATH="$HOME/.local/bin:$PATH"`) {
		t.Fatalf("install-native alias did not honor --path-mode=manual: %+v", result.Messages)
	}
}

func TestInstallCommandDryRunSkipPathModeDoesNotPlanShellRC(t *testing.T) {
	home := t.TempDir()
	configureInstallCommandTest(t, home)
	result := runInstallDryRunJSON(t, home, "install", "skip")
	if hasInstallFileKind(result.Files, "shell_path_rc") {
		t.Fatalf("skip path mode unexpectedly planned shell rc write: %+v", result.Files)
	}
}

func TestInstallNativeAliasKeepsUsageName(t *testing.T) {
	configureInstallCommandTest(t, t.TempDir())
	_, stderr, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstallCommand("install-native", []string{"--badflag"})
	})
	if err == nil {
		t.Fatalf("install-native --badflag unexpectedly succeeded:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Usage of install-native:") {
		t.Fatalf("install-native alias did not keep usage name:\n%s", stderr)
	}
	if strings.Contains(stderr, "Usage of install:") {
		t.Fatalf("install-native alias leaked install usage name:\n%s", stderr)
	}
}

func configureInstallCommandTest(t *testing.T, home string) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve harness root: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("HARNESS_ROOT", root)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin:/bin")
	oldHarnessRoot := HarnessRoot
	HarnessRoot = func() string { return root }
	t.Cleanup(func() { HarnessRoot = oldHarnessRoot })
	return root
}

func runInstallDryRunJSON(t *testing.T, home, commandName, pathMode string) port.NativeInstallResult {
	t.Helper()
	configureInstallCommandTest(t, home)
	out, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstallCommand(commandName, []string{"--dry-run", "--json", "--path-mode=" + pathMode})
	})
	if err != nil {
		t.Fatalf("%s --dry-run --json --path-mode=%s failed: %v\n%s", commandName, pathMode, err, out)
	}
	var result port.NativeInstallResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("install dry-run returned invalid JSON: %v\n%s", err, out)
	}
	return result
}

func captureInstallCommandOutput(t *testing.T, input io.Reader, fn func() error) (string, string, error) {
	t.Helper()
	oldStdin, oldStdout, oldStderr := os.Stdin, os.Stdout, os.Stderr
	defer func() {
		os.Stdin, os.Stdout, os.Stderr = oldStdin, oldStdout, oldStderr
	}()

	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	var inWrite *os.File
	if input != nil {
		inRead, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("create stdin pipe: %v", err)
		}
		os.Stdin = inRead
		inWrite = w
	}
	os.Stdout = outWrite
	os.Stderr = errWrite

	if input != nil {
		if _, err := io.Copy(inWrite, input); err != nil {
			t.Fatalf("write stdin pipe: %v", err)
		}
		if err := inWrite.Close(); err != nil {
			t.Fatalf("close stdin writer: %v", err)
		}
	}
	callErr := fn()
	if err := outWrite.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	if err := errWrite.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if _, err := io.Copy(&stdout, outRead); err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	if _, err := io.Copy(&stderr, errRead); err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	return stdout.String(), stderr.String(), callErr
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
