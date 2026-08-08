package installcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/port"
)

func TestInstallCommandDryRunJSONDispatches(t *testing.T) {
	home := t.TempDir()
	root := configureInstallCommandTest(t, home)

	out, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--dry-run", "--json"})
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
	result := runInstallDryRunJSON(t, home, "auto")
	if !hasInstallLink(result.Links, filepath.Join(home, ".local", "bin", "agent-harness"), filepath.Join(root, "bin", "agent-harness"), true) {
		t.Fatalf("auto path mode did not plan command shim link: %+v", result.Links)
	}
	if !hasInstallLink(result.Links, filepath.Join(home, ".local", "bin", "ah"), filepath.Join(home, ".local", "bin", "agent-harness"), true) {
		t.Fatalf("auto path mode did not plan ah command shim: %+v", result.Links)
	}
	if !hasInstallFile(result.Files, filepath.Join(home, ".zshrc"), "shell_path_rc", true) {
		t.Fatalf("auto path mode did not plan shell rc PATH write: %+v", result.Files)
	}
}

func TestInstallCommandDryRunManualPathModePlansShimWithoutShellRC(t *testing.T) {
	home := t.TempDir()
	root := configureInstallCommandTest(t, home)
	result := runInstallDryRunJSON(t, home, "manual")
	if !hasInstallLink(result.Links, filepath.Join(home, ".local", "bin", "agent-harness"), filepath.Join(root, "bin", "agent-harness"), true) {
		t.Fatalf("manual path mode did not plan command shim link: %+v", result.Links)
	}
	if !hasInstallLink(result.Links, filepath.Join(home, ".local", "bin", "ah"), filepath.Join(home, ".local", "bin", "agent-harness"), true) {
		t.Fatalf("manual path mode did not plan ah command shim: %+v", result.Links)
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
		return RunInstall([]string{"--interactive", "--dry-run", "--json"})
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

func TestInstallCommandInteractiveDryRunClosedStdinFailsBeforeDeadline(t *testing.T) {
	home := t.TempDir()
	configureInstallCommandTest(t, home)
	t.Setenv("AGENT_HARNESS_INSTALL_HELPER", "1")

	type result struct {
		stdout string
		stderr string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		stdout, stderr, err := captureInstallCommandOutput(t, strings.NewReader(""), func() error {
			return RunInstall([]string{"--interactive", "--dry-run", "--json"})
		})
		done <- result{stdout: stdout, stderr: stderr, err: err}
	}()

	select {
	case got := <-done:
		if got.err == nil || !strings.Contains(got.err.Error(), "interactive input ended before Enable project-local files? [y/N]:") {
			t.Fatalf("interactive closed stdin error = %v\nstdout=%s\nstderr=%s", got.err, got.stdout, got.stderr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("interactive dry-run hung after stdin closed")
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

func TestInstallCommandDryRunSkipPathModeDoesNotPlanShellRC(t *testing.T) {
	home := t.TempDir()
	root := configureInstallCommandTest(t, home)
	result := runInstallDryRunJSON(t, home, "skip")
	if !hasInstallLink(result.Links, filepath.Join(home, ".local", "bin", "agent-harness"), filepath.Join(root, "bin", "agent-harness"), true) {
		t.Fatalf("skip path mode did not plan command shim link: %+v", result.Links)
	}
	if !hasInstallLink(result.Links, filepath.Join(home, ".local", "bin", "ah"), filepath.Join(home, ".local", "bin", "agent-harness"), true) {
		t.Fatalf("skip path mode did not plan ah command shim: %+v", result.Links)
	}
	if hasInstallFileKind(result.Files, "shell_path_rc") {
		t.Fatalf("skip path mode unexpectedly planned shell rc write: %+v", result.Files)
	}
}

func TestInstallCommandShortShimKeepsMatchingLink(t *testing.T) {
	home := t.TempDir()
	configureInstallCommandTest(t, home)
	canonical := filepath.Join(home, ".local", "bin", "agent-harness")
	short := filepath.Join(home, ".local", "bin", "ah")
	if err := os.MkdirAll(filepath.Dir(short), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(canonical, short); err != nil {
		t.Fatal(err)
	}

	result := runInstallDryRunJSON(t, home, "skip")
	if !hasInstallLink(result.Links, short, canonical, false) {
		t.Fatalf("matching ah shim was not preserved: %+v", result.Links)
	}
}

func TestInstallCommandShortShimRefusesExistingFile(t *testing.T) {
	home := t.TempDir()
	configureInstallCommandTest(t, home)
	short := filepath.Join(home, ".local", "bin", "ah")
	if err := os.MkdirAll(filepath.Dir(short), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(short, []byte("user command"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--dry-run", "--json", "--path-mode=skip"})
	})
	if err == nil || !strings.Contains(err.Error(), "refusing to replace existing ah command") {
		t.Fatalf("existing ah file error = %v", err)
	}
	body, readErr := os.ReadFile(short)
	if readErr != nil || string(body) != "user command" {
		t.Fatalf("existing ah file changed: body=%q err=%v", body, readErr)
	}
}

func TestInstallCommandShortShimRefusesUnrelatedSymlink(t *testing.T) {
	home := t.TempDir()
	configureInstallCommandTest(t, home)
	short := filepath.Join(home, ".local", "bin", "ah")
	if err := os.MkdirAll(filepath.Dir(short), 0o755); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(home, "bin", "another-ah")
	if err := os.MkdirAll(filepath.Dir(unrelated), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(unrelated, short); err != nil {
		t.Fatal(err)
	}

	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--dry-run", "--json", "--path-mode=skip"})
	})
	if err == nil || !strings.Contains(err.Error(), "refusing to replace existing ah command") {
		t.Fatalf("unrelated ah symlink error = %v", err)
	}
	target, readErr := os.Readlink(short)
	if readErr != nil || target != unrelated {
		t.Fatalf("unrelated ah symlink changed: target=%q err=%v", target, readErr)
	}
}
