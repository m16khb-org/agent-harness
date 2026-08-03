package installcli

import (
	install "agent-harness/internal/adapter/install"
	activationport "agent-harness/internal/port/nativeactivation"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
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

func TestInstallCommandDryRunAllowsSelfVerifyBinaryOutsideHarnessRoot(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	skillDir := filepath.Join(root, "skills", "atomic-commit-push")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: atomic-commit-push\ndescription: fixture\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("HARNESS_ROOT", root)
	Configure(Deps{
		HarnessRoot:          func() string { return root },
		ExecutablePath:       func() (string, error) { return executable, nil },
		NativeInstallRequest: install.DefaultNativeInstallRequest,
		InstallNative: func(req port.NativeInstallRequest) (port.NativeInstallResult, error) {
			return install.InstallNative(req, installerFixture{})
		},
		ActivationReadback: func(port.NativeInstallRequest) activationport.ReadbackVerifier { return nil },
	})
	t.Cleanup(Reset)

	out, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--dry-run", "--project-local", "--path-mode=skip", "--json"})
	})
	if err != nil {
		t.Fatalf("self-verify install dry-run failed with executable outside HARNESS_ROOT: %v\n%s", err, out)
	}
	var result port.NativeInstallResult
	if err := json.Unmarshal([]byte(out), &result); err != nil || !result.OK || !result.DryRun {
		t.Fatalf("self-verify install dry-run result=%+v decodeErr=%v", result, err)
	}
}

func TestNativeInstallCandidatePathRejectsExternalBinaryForApply(t *testing.T) {
	target := filepath.Join(t.TempDir(), "bin", "agent-harness")
	external := filepath.Join(t.TempDir(), "agent-harness")
	_, err := nativeInstallCandidatePath(target, false, func() (string, error) { return external, nil })
	if err == nil || !strings.Contains(err.Error(), "canonical target or a same-directory staged binary") {
		t.Fatalf("external apply candidate error = %v", err)
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

func TestInstallCommandRefusesManagedRegularCommandWithoutApproval(t *testing.T) {
	home := t.TempDir()
	target := buildManagedTestCommand(t)
	command := filepath.Join(home, ".local", "bin", "agent-harness")
	copyTestCommand(t, target, command)
	want, err := os.ReadFile(command)
	if err != nil {
		t.Fatal(err)
	}
	result := port.NativeInstallResult{}
	_, err = prepareInstallPathPlan(&result, port.NativeInstallRequest{Home: home, BinPath: target, DryRun: true}, "skip")
	if err == nil || !strings.Contains(err.Error(), "--adopt-command-file") {
		t.Fatalf("regular command refusal error = %v", err)
	}
	got, readErr := os.ReadFile(command)
	if readErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("regular command changed without approval: readErr=%v", readErr)
	}
}

func TestInstallCommandApprovedDryRunReportsManagedAdoptionWithoutWriting(t *testing.T) {
	home := t.TempDir()
	target := buildManagedTestCommand(t)
	command := filepath.Join(home, ".local", "bin", "agent-harness")
	copyTestCommand(t, target, command)
	wantInfo, err := os.Lstat(command)
	if err != nil {
		t.Fatal(err)
	}
	result := port.NativeInstallResult{}
	_, err = prepareInstallPathPlan(&result, port.NativeInstallRequest{Home: home, BinPath: target, DryRun: true, AdoptCommandFile: true}, "skip")
	if err != nil {
		t.Fatal(err)
	}
	if result.CommandPath == nil || !result.CommandPath.AdoptionApproved || !result.CommandPath.WouldAdopt || !result.CommandPath.RollbackAvailable || result.CommandPath.BackupPath == "" {
		t.Fatalf("managed command dry-run receipt=%+v", result.CommandPath)
	}
	gotInfo, err := os.Lstat(command)
	if err != nil || gotInfo.Mode() != wantInfo.Mode() || gotInfo.Size() != wantInfo.Size() || gotInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("dry-run mutated command: got=%v want=%v err=%v", gotInfo, wantInfo, err)
	}
	if _, err := os.Lstat(result.CommandPath.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run created backup: %v", err)
	}
}

func TestInstallCommandApprovedDryRunValidatesStagedCandidate(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	target := filepath.Join(root, "bin", "agent-harness")
	buildManagedCommandAt(t, target)
	candidate := filepath.Join(filepath.Dir(target), ".agent-harness.activate-test")
	copyTestCommand(t, target, candidate)
	command := filepath.Join(home, ".local", "bin", "agent-harness")
	copyTestCommand(t, candidate, command)
	if err := os.WriteFile(target, []byte("not a managed Go binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := port.NativeInstallResult{}
	_, err := prepareInstallPathPlanForCandidate(&result, port.NativeInstallRequest{
		Home: home, BinPath: target, DryRun: true, AdoptCommandFile: true,
	}, candidate, "skip")
	if err != nil {
		t.Fatalf("staged candidate preflight failed: %v", err)
	}
	if result.CommandPath == nil || !result.CommandPath.WouldAdopt || result.CommandPath.Target != target {
		t.Fatalf("staged candidate plan = %+v", result.CommandPath)
	}
}

func buildManagedTestCommand(t *testing.T) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "agent-harness")
	buildManagedCommandAt(t, target)
	return target
}

func buildManagedCommandAt(t *testing.T, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "-o", target, "../../../cmd/harness")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build managed test command: %v\n%s", err, output)
	}
}

func copyTestCommand(t *testing.T, source, destination string) {
	t.Helper()
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, body, 0o755); err != nil {
		t.Fatal(err)
	}
}
