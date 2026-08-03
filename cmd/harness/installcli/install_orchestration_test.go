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

type activationBackendFixture struct{ calls []string }

func (fixture *activationBackendFixture) Begin(_ context.Context, request activationport.BeginRequest) (activationport.Result, error) {
	fixture.calls = append(fixture.calls, "begin")
	return activationport.Result{
		StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary,
		BinarySHA256: hashFixture('a'), TransitionID: validTransitionID, Pending: true, UpdatedAt: "2000-01-01T00:00:00Z",
	}, nil
}

func (fixture *activationBackendFixture) Seal(_ context.Context, request activationport.SealRequest) (activationport.Result, error) {
	fixture.calls = append(fixture.calls, "seal")
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
	evidence := make([]activationport.Evidence, 0, 4)
	for _, item := range [][2]string{{"codex", "mcp"}, {"codex", "hooks"}, {"claude", "mcp"}, {"claude", "hooks"}} {
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
	return port.HostInstallResult{Host: "fixture", OK: fixture.err == nil, DryRun: request.DryRun}, fixture.err
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
	if decodeErr := json.Unmarshal([]byte(out), &result); decodeErr != nil || !result.AbortRequired || result.CommandPath == nil || !result.CommandPath.RolledBack {
		t.Fatalf("explicit seal result=%+v decodeErr=%v", result, decodeErr)
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
