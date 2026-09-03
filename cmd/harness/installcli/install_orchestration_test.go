package installcli

import (
	install "agent-harness/internal/adapter/install"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"agent-harness/internal/port"
	activationport "agent-harness/internal/port/nativeactivation"
)

const validTransitionID = "00112233445566778899aabbccddeeff"

type activationBackendFixture struct {
	calls   []string
	sealErr error
}

func (fixture *activationBackendFixture) Begin(_ context.Context, request activationport.BeginRequest) (activationport.Result, error) {
	fixture.calls = append(fixture.calls, "begin")
	return activationport.Result{
		StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary,
		BinarySHA256: hashFixture('a'), TransitionID: validTransitionID, Pending: true, UpdatedAt: "2000-01-01T00:00:00Z",
	}, nil
}

func (fixture *activationBackendFixture) Seal(_ context.Context, request activationport.SealRequest) (activationport.Result, error) {
	fixture.calls = append(fixture.calls, "seal")
	if fixture.sealErr != nil {
		return activationport.Result{}, fixture.sealErr
	}
	return activationport.Result{
		StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary,
		BinarySHA256: hashFixture('a'), TransitionID: request.TransitionID, Sealed: true, UpdatedAt: "2000-01-01T00:00:01Z",
	}, nil
}

func (fixture *activationBackendFixture) Abort(_ context.Context, request activationport.AbortRequest) (activationport.Result, error) {
	fixture.calls = append(fixture.calls, "abort")
	return activationport.Result{
		StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary,
		BinarySHA256: hashFixture('a'), TransitionID: request.TransitionID, Aborted: true, UpdatedAt: "2000-01-01T00:00:01Z",
	}, nil
}

type activationReadbackFixture struct{}

func (activationReadbackFixture) Verify(context.Context, string, string) (activationport.Readback, error) {
	evidence := make([]activationport.Evidence, 0, 7)
	for _, item := range [][2]string{
		{"codex", "mcp"}, {"codex", "hooks"},
		{"claude", "mcp"}, {"claude", "hooks"},
		{"omo", "mcp"}, {"omo", "hooks"},
		{"agy", "mcp"},
	} {
		evidence = append(evidence, activationport.Evidence{
			Host: item[0], Surface: item[1], Path: "/" + item[0] + "/" + item[1], SemanticSHA256: hashFixture('b'), SHA256: hashFixture('c'),
		})
	}
	return activationport.Readback{CatalogSHA256: hashFixture('d'), Evidence: evidence}, nil
}

type installerFixture struct {
	err   error
	calls *int
}

func (installerFixture) Name() string { return "fixture" }
func (fixture installerFixture) Install(request port.NativeInstallRequest) (port.HostInstallResult, error) {
	if fixture.calls != nil {
		*fixture.calls++
	}
	if request.DryRun {
		return port.HostInstallResult{Host: "fixture", OK: true, DryRun: true}, nil
	}
	return port.HostInstallResult{Host: "fixture", OK: fixture.err == nil, DryRun: request.DryRun}, fixture.err
}

type mutatingInstallerFixture struct {
	existingFile string
	existingLink string
	newFile      string
	newLink      string
	oldTarget    string
	newTarget    string
}

func (mutatingInstallerFixture) Name() string { return "fixture" }

func (fixture mutatingInstallerFixture) Install(request port.NativeInstallRequest) (port.HostInstallResult, error) {
	files := []port.InstallFile{
		{Path: fixture.existingFile, Kind: "existing", WouldWrite: request.DryRun},
		{Path: fixture.newFile, Kind: "new", WouldWrite: request.DryRun},
	}
	links := []port.InstallLink{
		{Path: fixture.existingLink, Target: fixture.newTarget, WouldCreate: request.DryRun},
		{Path: fixture.newLink, Target: fixture.newTarget, WouldCreate: request.DryRun},
	}
	if request.DryRun {
		return port.HostInstallResult{Host: "fixture", OK: true, DryRun: true, Files: files, Links: links}, nil
	}
	for _, path := range []string{fixture.existingFile, fixture.newFile} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return port.HostInstallResult{Host: "fixture"}, err
		}
		if err := os.WriteFile(path, []byte("after\n"), 0o644); err != nil {
			return port.HostInstallResult{Host: "fixture"}, err
		}
	}
	if err := os.Remove(fixture.existingLink); err != nil {
		return port.HostInstallResult{Host: "fixture"}, err
	}
	for _, path := range []string{fixture.existingLink, fixture.newLink} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return port.HostInstallResult{Host: "fixture"}, err
		}
		if err := os.Symlink(fixture.newTarget, path); err != nil {
			return port.HostInstallResult{Host: "fixture"}, err
		}
	}
	for index := range files {
		files[index].Written = true
		files[index].WouldWrite = false
	}
	for index := range links {
		links[index].Created = true
		links[index].WouldCreate = false
	}
	return port.HostInstallResult{Host: "fixture", OK: false, Files: files, Links: links}, errors.New("injected host failure after writes")
}

type preflightFailureInstallerFixture struct{}

func (preflightFailureInstallerFixture) Name() string { return "fixture" }
func (preflightFailureInstallerFixture) Install(request port.NativeInstallRequest) (port.HostInstallResult, error) {
	return port.HostInstallResult{Host: "fixture", OK: false, DryRun: request.DryRun}, errors.New("injected host preflight failure")
}

func TestInstallOrchestrationRollsBackAndAbortsAfterHostFailure(t *testing.T) {
	root, home, target, command := installOrchestrationFixture(t)
	backend := &activationBackendFixture{}
	configureInstallOrchestrationFixture(t, root, home, backend, installerFixture{err: errors.New("injected host failure")})

	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--adopt-command-file", "--path-mode=skip", "--json"})
	})
	if err == nil {
		t.Fatal("host failure was accepted")
	}
	assertRegularCommandBytes(t, command, target)
	if !reflect.DeepEqual(backend.calls, []string{"begin", "abort"}) {
		t.Fatalf("activation calls=%v", backend.calls)
	}
}

func TestInstallOrchestrationRemovesNewCommandShimsAfterHostFailure(t *testing.T) {
	root, home, _, command := installOrchestrationFixture(t)
	if err := os.RemoveAll(filepath.Join(home, ".local")); err != nil {
		t.Fatal(err)
	}
	backend := &activationBackendFixture{}
	configureInstallOrchestrationFixture(t, root, home, backend, installerFixture{err: errors.New("injected host failure")})

	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--path-mode=skip", "--json"})
	})
	if err == nil {
		t.Fatal("host failure was accepted")
	}
	for _, path := range []string{command, filepath.Join(filepath.Dir(command), "ah"), filepath.Join(home, ".local")} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("new command path survived rollback: %s err=%v", path, statErr)
		}
	}
	if !reflect.DeepEqual(backend.calls, []string{"begin", "abort"}) {
		t.Fatalf("activation calls=%v", backend.calls)
	}
}

func TestInstallOrchestrationPreservesExistingEmptyCommandDirectories(t *testing.T) {
	root, home, _, _ := installOrchestrationFixture(t)
	localDir := filepath.Join(home, ".local")
	binDir := filepath.Join(localDir, "bin")
	if err := os.RemoveAll(localDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	backend := &activationBackendFixture{}
	configureInstallOrchestrationFixture(t, root, home, backend, installerFixture{err: errors.New("injected host failure")})

	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--path-mode=skip", "--json"})
	})
	if err == nil {
		t.Fatal("host failure was accepted")
	}
	for _, path := range []string{localDir, binDir} {
		info, statErr := os.Stat(path)
		if statErr != nil || !info.IsDir() {
			t.Fatalf("existing command directory was removed: %s info=%v err=%v", path, info, statErr)
		}
	}
}

func TestInstallOrchestrationRestoresReplacedCommandSymlink(t *testing.T) {
	root, home, _, command := installOrchestrationFixture(t)
	oldTarget := filepath.Join(home, "old-agent-harness")
	if err := os.Remove(command); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldTarget, command); err != nil {
		t.Fatal(err)
	}
	backend := &activationBackendFixture{}
	configureInstallOrchestrationFixture(t, root, home, backend, installerFixture{err: errors.New("injected host failure")})

	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--path-mode=skip", "--json"})
	})
	if err == nil {
		t.Fatal("host failure was accepted")
	}
	assertInstallSymlinkTarget(t, command, oldTarget)
	assertInstallSymlinkTarget(t, filepath.Join(filepath.Dir(command), "ah"), command)
	if !reflect.DeepEqual(backend.calls, []string{"begin", "abort"}) {
		t.Fatalf("activation calls=%v", backend.calls)
	}
}

func TestInstallOrchestrationRestoresHostPathsAfterWriteFailure(t *testing.T) {
	root, home, target, command := installOrchestrationFixture(t)
	oldTarget := filepath.Join(root, "skills", "old")
	newTarget := filepath.Join(root, "skills", "fixture")
	if err := os.MkdirAll(oldTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	existingFile := filepath.Join(home, ".omo", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(existingFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingFile, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	existingLink := filepath.Join(home, ".omo", "skills", "fixture")
	if err := os.MkdirAll(filepath.Dir(existingLink), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldTarget, existingLink); err != nil {
		t.Fatal(err)
	}
	newFile := filepath.Join(root, "configs", "omo", "mcp.json")
	newLink := filepath.Join(root, ".omo", "skills", "fixture")
	backend := &activationBackendFixture{}
	configureInstallOrchestrationFixture(t, root, home, backend, mutatingInstallerFixture{
		existingFile: existingFile,
		existingLink: existingLink,
		newFile:      newFile,
		newLink:      newLink,
		oldTarget:    oldTarget,
		newTarget:    newTarget,
	})

	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--adopt-command-file", "--path-mode=skip", "--json"})
	})
	if err == nil {
		t.Fatal("host write failure was accepted")
	}
	assertRegularCommandBytes(t, command, target)
	body, readErr := os.ReadFile(existingFile)
	info, statErr := os.Stat(existingFile)
	if readErr != nil || statErr != nil || string(body) != "before\n" || info.Mode().Perm() != 0o600 {
		t.Fatalf("existing Omo config was not restored: body=%q mode=%v readErr=%v statErr=%v", body, info.Mode(), readErr, statErr)
	}
	assertInstallSymlinkTarget(t, existingLink, oldTarget)
	for _, path := range []string{newFile, newLink, filepath.Join(root, "configs"), filepath.Join(root, ".omo")} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("new host mutation path survived rollback: %s err=%v", path, statErr)
		}
	}
	if !reflect.DeepEqual(backend.calls, []string{"begin", "abort"}) {
		t.Fatalf("activation calls=%v", backend.calls)
	}
}

func TestInstallOrchestrationRestoresShellPathAfterSealFailure(t *testing.T) {
	root, home, target, command := installOrchestrationFixture(t)
	rcPath := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(rcPath, []byte("# before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin:/bin")
	backend := &activationBackendFixture{sealErr: errors.New("injected seal failure")}
	configureInstallOrchestrationFixture(t, root, home, backend, installerFixture{})

	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--adopt-command-file", "--path-mode=auto", "--json"})
	})
	if err == nil {
		t.Fatal("seal failure was accepted")
	}
	assertRegularCommandBytes(t, command, target)
	body, readErr := os.ReadFile(rcPath)
	info, statErr := os.Stat(rcPath)
	if readErr != nil || statErr != nil || string(body) != "# before\n" || info.Mode().Perm() != 0o600 {
		t.Fatalf("shell rc was not restored: body=%q mode=%v readErr=%v statErr=%v", body, info.Mode(), readErr, statErr)
	}
	if !reflect.DeepEqual(backend.calls, []string{"begin", "seal", "abort"}) {
		t.Fatalf("activation calls=%v", backend.calls)
	}
}

func TestInstallOrchestrationPreflightRefusalDoesNotBeginActivation(t *testing.T) {
	root, home, target, command := installOrchestrationFixture(t)
	backend := &activationBackendFixture{}
	installCalls := 0
	configureInstallOrchestrationFixture(t, root, home, backend, installerFixture{calls: &installCalls})

	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--dry-run", "--path-mode=skip", "--json"})
	})
	if err == nil || len(backend.calls) != 0 || installCalls != 0 {
		t.Fatalf("preflight err=%v activation calls=%v installer calls=%d", err, backend.calls, installCalls)
	}
	assertRegularCommandBytes(t, command, target)
}

func TestInstallOrchestrationHostPreflightFailureDoesNotBeginActivation(t *testing.T) {
	root, home, target, command := installOrchestrationFixture(t)
	backend := &activationBackendFixture{}
	configureInstallOrchestrationFixture(t, root, home, backend, preflightFailureInstallerFixture{})

	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--adopt-command-file", "--path-mode=skip", "--json"})
	})
	if err == nil || len(backend.calls) != 0 {
		t.Fatalf("host preflight err=%v activation calls=%v", err, backend.calls)
	}
	assertRegularCommandBytes(t, command, target)
}

func TestInstallOrchestrationExplicitSealPreflightFailureRequiresAbort(t *testing.T) {
	root, home, target, command := installOrchestrationFixture(t)
	backend := &activationBackendFixture{}
	configureInstallOrchestrationFixture(t, root, home, backend, preflightFailureInstallerFixture{})
	t.Setenv("HARNESS_NATIVE_ACTIVATION_STEP", "seal")
	t.Setenv("HARNESS_NATIVE_ACTIVATION_TRANSITION_ID", validTransitionID)

	out, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--adopt-command-file", "--path-mode=skip", "--json"})
	})
	if err == nil {
		t.Fatal("explicit seal preflight failure was accepted")
	}
	var result port.NativeInstallResult
	decodeErr := json.Unmarshal([]byte(out), &result)
	if decodeErr != nil || !result.AbortRequired || result.TransitionID != validTransitionID ||
		result.CommandPath == nil || !result.CommandPath.AbortRequired {
		t.Fatalf("explicit seal preflight result=%+v commandPath=%+v decodeErr=%v", result, result.CommandPath, decodeErr)
	}
	if len(backend.calls) != 0 {
		t.Fatalf("explicit seal preflight failure must leave abort to caller: calls=%v", backend.calls)
	}
	assertRegularCommandBytes(t, command, target)
}

func TestInstallOrchestrationExplicitSealRollsBackAndLeavesAbortToCaller(t *testing.T) {
	root, home, target, command := installOrchestrationFixture(t)
	backend := &activationBackendFixture{}
	configureInstallOrchestrationFixture(t, root, home, backend, installerFixture{err: errors.New("injected host failure")})
	t.Setenv("HARNESS_NATIVE_ACTIVATION_STEP", "seal")
	t.Setenv("HARNESS_NATIVE_ACTIVATION_TRANSITION_ID", validTransitionID)

	out, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--adopt-command-file", "--path-mode=skip", "--json"})
	})
	if err == nil {
		t.Fatal("host failure was accepted")
	}
	var result port.NativeInstallResult
	decodeErr := json.Unmarshal([]byte(out), &result)
	if decodeErr != nil || !result.AbortRequired || result.CommandPath == nil || !result.CommandPath.RolledBack {
		t.Fatalf("explicit seal result=%+v commandPath=%+v decodeErr=%v", result, result.CommandPath, decodeErr)
	}
	if len(backend.calls) != 0 {
		t.Fatalf("explicit seal failure must not abort pending itself: calls=%v", backend.calls)
	}
	assertRegularCommandBytes(t, command, target)
}

func TestInstallOrchestrationFinalizesOnlyAfterSeal(t *testing.T) {
	root, home, target, command := installOrchestrationFixture(t)
	backend := &activationBackendFixture{}
	configureInstallOrchestrationFixture(t, root, home, backend, installerFixture{})

	out, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstall([]string{"--adopt-command-file", "--path-mode=skip", "--json"})
	})
	if err != nil {
		t.Fatalf("install failed: %v\n%s", err, out)
	}
	assertInstallSymlinkTarget(t, command, target)
	if !reflect.DeepEqual(backend.calls, []string{"begin", "seal"}) {
		t.Fatalf("activation calls=%v", backend.calls)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(command), ".agent-harness.command-backup-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("committed backup matches=%v err=%v", matches, err)
	}
}

func installOrchestrationFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	target := filepath.Join(root, "bin", "agent-harness")
	if err := os.MkdirAll(filepath.Join(root, "skills", "fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	buildManagedCommandAt(t, target)
	command := filepath.Join(home, ".local", "bin", "agent-harness")
	copyTestCommand(t, target, command)
	if err := os.Symlink(command, filepath.Join(filepath.Dir(command), "ah")); err != nil {
		t.Fatal(err)
	}
	return root, home, target, command
}

func configureInstallOrchestrationFixture(t *testing.T, root, home string, backend *activationBackendFixture, installer port.HostInstaller) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("HARNESS_ROOT", root)
	t.Setenv("HARNESS_NATIVE_ACTIVATION_STEP", "")
	Configure(Deps{
		HarnessRoot: func() string { return root }, ExecutablePath: func() (string, error) { return filepath.Join(root, "bin", "agent-harness"), nil },
		ActivationBackend: backend,
		ActivationReadback: func(port.NativeInstallRequest) activationport.ReadbackVerifier {
			return activationReadbackFixture{}
		},
		NativeInstallRequest: install.DefaultNativeInstallRequest,
		InstallNative: func(req port.NativeInstallRequest) (port.NativeInstallResult, error) {
			return install.InstallNative(req, installer)
		},
	})
	t.Cleanup(Reset)
}

func assertRegularCommandBytes(t *testing.T, path, expected string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(expected)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || !reflect.DeepEqual(got, want) {
		t.Fatalf("restored command info=%v err=%v", info, err)
	}
}

func assertInstallSymlinkTarget(t *testing.T, path, want string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 || got != want {
		t.Fatalf("command symlink target=%q mode=%v err=%v", got, info.Mode(), err)
	}
}

func hashFixture(character byte) string {
	value := make([]byte, 64)
	for index := range value {
		value[index] = character
	}
	return string(value)
}
