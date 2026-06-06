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

func stubPostInstallMCPProxyRefresh(t *testing.T, fn func() (int, error)) func() {
	t.Helper()
	previous := postInstallMCPProxyRefresh
	postInstallMCPProxyRefresh = fn
	return func() { postInstallMCPProxyRefresh = previous }
}

func stubDaemonProcessLister(t *testing.T, fn func() ([]daemonProcess, error)) func() {
	t.Helper()
	previous := daemonProcessLister
	daemonProcessLister = fn
	return func() { daemonProcessLister = previous }
}

func stubDaemonProcessTerminator(t *testing.T, fn func(int) error) func() {
	t.Helper()
	previous := daemonProcessTerminator
	daemonProcessTerminator = fn
	return func() { daemonProcessTerminator = previous }
}

func stubMCPProxyProcessLister(t *testing.T, fn func() ([]mcpProxyProcess, error)) func() {
	t.Helper()
	previous := mcpProxyProcessLister
	mcpProxyProcessLister = fn
	return func() { mcpProxyProcessLister = previous }
}

func stubMCPProxyTerminator(t *testing.T, fn func(int) error) func() {
	t.Helper()
	previous := mcpProxyTerminator
	mcpProxyTerminator = fn
	return func() { mcpProxyTerminator = previous }
}

func TestRunInstallScriptCommandRefreshesRuntimeProcessesAfterUpdate(t *testing.T) {
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
	restoreMCPProxy := stubPostInstallMCPProxyRefresh(t, func() (int, error) {
		commands = append(commands, "mcp-proxy-refresh")
		return 2, nil
	})
	defer restoreMCPProxy()

	if err := runInstallScriptCommand("update", []string{"--skip-upstream-tools"}); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "scripts", "install-native.sh"), "--skip-upstream-tools", "daemon-refresh", "mcp-proxy-refresh"}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected command sequence:\n got: %#v\nwant: %#v", commands, want)
	}
}

func TestRunUpdateAndBootstrapForwardToInstallScript(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var commands [][]string
	restore := stubInstallScriptCommandRunner(t, func(name string, args ...string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	})
	defer restore()
	restoreDaemon := stubPostInstallDaemonRefresh(t, func() (bool, error) {
		t.Fatal("dry-run wrapper must not refresh daemon")
		return false, nil
	})
	defer restoreDaemon()
	restoreMCPProxy := stubPostInstallMCPProxyRefresh(t, func() (int, error) {
		t.Fatal("dry-run wrapper must not refresh MCP proxies")
		return 0, nil
	})
	defer restoreMCPProxy()

	if err := runUpdate([]string{"--dry-run", "--without-upstream-tools", "--json"}); err != nil {
		t.Fatal(err)
	}
	if err := runBootstrap([]string{"--dry-run", "--path-mode=skip", "--skip-build"}); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{filepath.Join(root, "scripts", "install-native.sh"), "--skip-upstream-tools", "--dry-run", "--json"},
		{filepath.Join(root, "scripts", "install-native.sh"), "--skip-upstream-tools", "--dry-run", "--path-mode=skip", "--skip-build"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected wrapper command sequence:\n got: %#v\nwant: %#v", commands, want)
	}
}

func TestRunInstallScriptCommandSkipsRuntimeProcessRefreshOnDryRun(t *testing.T) {
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
	restoreMCPProxy := stubPostInstallMCPProxyRefresh(t, func() (int, error) {
		refreshed = true
		return 1, nil
	})
	defer restoreMCPProxy()

	if err := runInstallScriptCommand("update", []string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if refreshed {
		t.Fatal("dry-run update must not refresh runtime processes")
	}
}

func TestParseMCPProxyProcessOnlyMatchesCurrentHarnessMCP(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bin", "agent-harness")
	match, ok := parseMCPProxyProcess("  123 "+binary+" mcp", binary)
	if !ok || match.PID != 123 || match.Command != binary+" mcp" {
		t.Fatalf("expected MCP proxy match, got match=%+v ok=%v", match, ok)
	}
	for _, line := range []string{
		"124 " + binary + " daemon --internal",
		"125 " + binary + " update",
		"126 /other/bin/agent-harness mcp",
		"not-a-pid " + binary + " mcp",
	} {
		if got, ok := parseMCPProxyProcess(line, binary); ok {
			t.Fatalf("unexpected match for %q: %+v", line, got)
		}
	}
}

func TestParseDaemonProcessOnlyMatchesCurrentHarnessDaemon(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bin", "agent-harness")
	match, ok := parseDaemonProcess("  123 "+binary+" daemon --internal", binary)
	if !ok || match.PID != 123 || match.Command != binary+" daemon --internal" {
		t.Fatalf("expected daemon match, got match=%+v ok=%v", match, ok)
	}
	for _, line := range []string{
		"124 " + binary + " mcp",
		"125 " + binary + " daemon start",
		"126 /other/bin/agent-harness daemon --internal",
		"not-a-pid " + binary + " daemon --internal",
	} {
		if got, ok := parseDaemonProcess(line, binary); ok {
			t.Fatalf("unexpected match for %q: %+v", line, got)
		}
	}
}

func TestTerminateStaleDaemonProcessesSkipsCurrentProcess(t *testing.T) {
	currentPID := os.Getpid()
	restoreList := stubDaemonProcessLister(t, func() ([]daemonProcess, error) {
		return []daemonProcess{
			{PID: currentPID, Command: "agent-harness daemon --internal"},
			{PID: 12345, Command: "agent-harness daemon --internal"},
			{PID: 12346, Command: "agent-harness daemon --internal"},
		}, nil
	})
	defer restoreList()
	var terminated []int
	restoreTerminate := stubDaemonProcessTerminator(t, func(pid int) error {
		terminated = append(terminated, pid)
		return nil
	})
	defer restoreTerminate()

	count, err := terminateStaleDaemonProcesses()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || !reflect.DeepEqual(terminated, []int{12345, 12346}) {
		t.Fatalf("unexpected daemon cleanup result count=%d terminated=%v", count, terminated)
	}
}

func TestRefreshRunningMCPProxiesAfterInstallSkipsCurrentProcess(t *testing.T) {
	currentPID := os.Getpid()
	restoreList := stubMCPProxyProcessLister(t, func() ([]mcpProxyProcess, error) {
		return []mcpProxyProcess{
			{PID: currentPID, Command: "agent-harness mcp"},
			{PID: 22345, Command: "agent-harness mcp"},
			{PID: 22346, Command: "agent-harness mcp"},
		}, nil
	})
	defer restoreList()
	var terminated []int
	restoreTerminate := stubMCPProxyTerminator(t, func(pid int) error {
		terminated = append(terminated, pid)
		return nil
	})
	defer restoreTerminate()

	count, err := refreshRunningMCPProxiesAfterInstall()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || !reflect.DeepEqual(terminated, []int{22345, 22346}) {
		t.Fatalf("unexpected MCP proxy cleanup result count=%d terminated=%v", count, terminated)
	}
}
