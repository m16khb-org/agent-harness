package updatecli

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

	if err := runInstallScriptCommand("update", nil); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "scripts", "install-native.sh"), "daemon-refresh", "mcp-proxy-refresh"}
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

	if err := runUpdate([]string{"--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	if err := runBootstrap([]string{"--dry-run", "--path-mode=skip", "--skip-build"}); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{filepath.Join(root, "scripts", "install-native.sh"), "--dry-run", "--json"},
		{filepath.Join(root, "scripts", "install-native.sh"), "--dry-run", "--path-mode=skip", "--skip-build"},
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

func TestExportedUpdateFacadesForwardToConfiguredDependencies(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	restoreRunner := stubInstallScriptCommandRunner(t, installScriptCommandRunner)
	defer restoreRunner()
	restoreDaemon := stubPostInstallDaemonRefresh(t, postInstallDaemonRefresh)
	defer restoreDaemon()
	restoreMCPProxy := stubPostInstallMCPProxyRefresh(t, postInstallMCPProxyRefresh)
	defer restoreMCPProxy()
	restoreDaemonList := stubDaemonProcessLister(t, daemonProcessLister)
	defer restoreDaemonList()
	restoreDaemonTerm := stubDaemonProcessTerminator(t, daemonProcessTerminator)
	defer restoreDaemonTerm()
	restoreMCPProxyList := stubMCPProxyProcessLister(t, mcpProxyProcessLister)
	defer restoreMCPProxyList()
	restoreMCPProxyTerm := stubMCPProxyTerminator(t, mcpProxyTerminator)
	defer restoreMCPProxyTerm()

	var commands [][]string
	SetInstallScriptCommandRunner(func(name string, args ...string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	})
	SetPostInstallDaemonRefresh(func() (bool, error) { return true, nil })
	SetPostInstallMCPProxyRefresh(func() (int, error) { return 2, nil })

	if err := RunUpdate([]string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if err := RunBootstrap([]string{"--dry-run", "--path-mode=skip"}); err != nil {
		t.Fatal(err)
	}
	if err := RunInstallScriptCommand("update", []string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if got := len(commands); got != 3 {
		t.Fatalf("expected three exported install wrapper calls, got %d: %#v", got, commands)
	}

	SetDaemonProcessLister(func() ([]DaemonProcess, error) {
		return []DaemonProcess{{PID: 11, Command: "agent-harness daemon serve"}}, nil
	})
	SetDaemonProcessTerminator(func(pid int) error {
		if pid != 11 {
			t.Fatalf("daemon pid = %d", pid)
		}
		return nil
	})
	terminated, err := TerminateStaleDaemonProcesses()
	if err != nil || terminated != 1 {
		t.Fatalf("TerminateStaleDaemonProcesses = %d err=%v", terminated, err)
	}
	if _, err := ListDaemonProcesses(); err != nil {
		t.Fatalf("ListDaemonProcesses err=%v", err)
	}
	binary := filepath.Join(root, "bin", "agent-harness")
	if parsed, ok := ParseDaemonProcess("11 "+binary+" daemon --internal", binary); !ok || parsed.PID != 11 {
		t.Fatalf("ParseDaemonProcess = %#v ok=%v", parsed, ok)
	}

	SetMCPProxyProcessLister(func() ([]MCPProxyProcess, error) {
		return []MCPProxyProcess{{PID: 22, Command: "agent-harness mcp serve"}}, nil
	})
	SetMCPProxyTerminator(func(pid int) error {
		if pid != 22 {
			t.Fatalf("mcp proxy pid = %d", pid)
		}
		return nil
	})
	refreshed, err := RefreshRunningMCPProxiesAfterInstall()
	if err != nil || refreshed != 1 {
		t.Fatalf("RefreshRunningMCPProxiesAfterInstall = %d err=%v", refreshed, err)
	}
	if _, err := ListMCPProxyProcesses(); err != nil {
		t.Fatalf("ListMCPProxyProcesses err=%v", err)
	}
	if parsed, ok := ParseMCPProxyProcess("22 "+binary+" mcp", binary); !ok || parsed.PID != 22 {
		t.Fatalf("ParseMCPProxyProcess = %#v ok=%v", parsed, ok)
	}
}

func TestCleanupMCPProxiesDryRunAndApply(t *testing.T) {
	restoreList := stubMCPProxyProcessLister(t, func() ([]mcpProxyProcess, error) {
		return []mcpProxyProcess{
			{PID: os.Getpid(), Command: "agent-harness mcp"},
			{PID: 22, Command: "agent-harness mcp"},
			{PID: 33, Command: "agent-harness mcp"},
		}, nil
	})
	defer restoreList()

	var terminated []int
	restoreTerm := stubMCPProxyTerminator(t, func(pid int) error {
		terminated = append(terminated, pid)
		return nil
	})
	defer restoreTerm()

	dryRun, err := CleanupMCPProxies(true)
	if err != nil {
		t.Fatal(err)
	}
	if !dryRun.OK || !dryRun.DryRun || dryRun.Matched != 3 || dryRun.Terminated != 0 || len(terminated) != 0 {
		t.Fatalf("dry-run cleanup = %#v terminated=%#v", dryRun, terminated)
	}
	if len(dryRun.Processes) != 3 || dryRun.Processes[0].Action != "skip-current" || dryRun.Processes[1].Action != "would-terminate" {
		t.Fatalf("dry-run cleanup actions = %#v", dryRun.Processes)
	}

	applied, err := CleanupMCPProxies(false)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.OK || applied.DryRun || applied.Matched != 3 || applied.Terminated != 2 {
		t.Fatalf("apply cleanup = %#v", applied)
	}
	if !reflect.DeepEqual(terminated, []int{22, 33}) {
		t.Fatalf("terminated = %#v", terminated)
	}
	if applied.Processes[0].Action != "skip-current" || applied.Processes[1].Action != "terminated" {
		t.Fatalf("apply cleanup actions = %#v", applied.Processes)
	}
}

func TestCleanupMCPProxiesReturnsEmptyProcessListWhenNoMatches(t *testing.T) {
	restoreList := stubMCPProxyProcessLister(t, func() ([]mcpProxyProcess, error) {
		return nil, nil
	})
	defer restoreList()

	result, err := CleanupMCPProxies(true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Matched != 0 || result.Processes == nil || len(result.Processes) != 0 {
		t.Fatalf("cleanup no matches = %#v", result)
	}
}
