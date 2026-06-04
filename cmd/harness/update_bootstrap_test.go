package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func stubInstallScriptCommandRunner(t *testing.T, fn func(string, ...string) error) func() {
	t.Helper()
	previous := installScriptCommandRunner
	installScriptCommandRunner = fn
	return func() { installScriptCommandRunner = previous }
}

func stubPostInstallDaemonRefresh(t *testing.T, fn func() (bool, error)) func() {
	t.Helper()
	previous := postInstallDaemonRefresh
	postInstallDaemonRefresh = fn
	return func() { postInstallDaemonRefresh = previous }
}

func TestRunInstallScriptCommandRestartsRunningDaemonAfterUpdate(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var commands []string
	restore := stubInstallScriptCommandRunner(t, func(name string, args ...string) error {
		commands = append(commands, append([]string{name}, args...)...)
		return nil
	})
	defer restore()

	daemonWasRunning := true
	restoreDaemon := stubPostInstallDaemonRefresh(t, func() (bool, error) {
		commands = append(commands, "daemon-refresh")
		return daemonWasRunning, nil
	})
	defer restoreDaemon()

	if err := runInstallScriptCommand("update", []string{"--skip-upstream-tools"}); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "scripts", "install-native.sh"), "--skip-upstream-tools", "daemon-refresh"}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected command sequence:\n got: %#v\nwant: %#v", commands, want)
	}
}

func TestRunInstallScriptCommandSkipsDaemonRefreshOnDryRun(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var refreshed bool
	restore := stubInstallScriptCommandRunner(t, func(name string, args ...string) error { return nil })
	defer restore()
	restoreDaemon := stubPostInstallDaemonRefresh(t, func() (bool, error) {
		refreshed = true
		return true, nil
	})
	defer restoreDaemon()

	if err := runInstallScriptCommand("update", []string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if refreshed {
		t.Fatal("dry-run update must not restart daemon")
	}
}
