package updatecli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunInstallScriptCommandRejectsUnexpectedArgsAndMissingScript(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)

	if err := runInstallScriptCommand("update", []string{"extra"}); err == nil {
		t.Fatalf("expected unexpected argument error")
	}
	if err := runInstallScriptCommand("update", nil); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing install script error, got %v", err)
	}
}

func TestRefreshRunningMCPProxiesAfterInstallPropagatesListError(t *testing.T) {
	want := errors.New("list failed")
	restoreList := stubMCPProxyProcessLister(t, func() ([]mcpProxyProcess, error) {
		return nil, want
	})
	defer restoreList()

	count, err := refreshRunningMCPProxiesAfterInstall()
	if count != 0 || !errors.Is(err, want) {
		t.Fatalf("refreshRunningMCPProxiesAfterInstall count=%d err=%v, want error %v", count, err, want)
	}
}

func TestRunInstallScriptCommandForwardsProjectLocalAndInteractiveFlags(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var got []string
	restore := stubInstallScriptCommandRunner(t, func(name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	})
	defer restore()

	if err := runInstallScriptCommand("bootstrap", []string{"--dry-run", "--project-local", "--interactive"}); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "scripts", "install-native.sh"), "--project-local", "--dry-run", "--interactive"}
	if !equalStringSlices(got, want) {
		t.Fatalf("unexpected install args: got %#v want %#v", got, want)
	}
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
