package updatecli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func stubInstalledDaemonCommandRunner(t *testing.T, fn func(string, ...string) error) func() {
	t.Helper()
	previous := installedDaemonCommandRunner
	installedDaemonCommandRunner = fn
	return func() { installedDaemonCommandRunner = previous }
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

func TestRunInstallScriptCommandRefreshesDaemonWithoutTouchingMCPProcesses(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
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
		t.Fatal("update must preserve active Codex, Claude, and external MCP processes")
		return 0, nil
	})
	defer restoreMCPProxy()

	if err := runInstallScriptCommand("update", nil); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "scripts", "install-native.sh"), "daemon-refresh"}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected command sequence:\n got: %#v\nwant: %#v", commands, want)
	}
}

func TestRefreshRunningDaemonAfterInstallUsesInstalledBinaryLifecycle(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
	t.Setenv("ISSUEOPS_DAEMON_DIR", t.TempDir())

	var commands [][]string
	restoreRunner := stubInstalledDaemonCommandRunner(t, func(name string, args ...string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	})
	defer restoreRunner()
	restoreList := stubDaemonProcessLister(t, func() ([]daemonProcess, error) {
		return nil, nil
	})
	defer restoreList()

	refreshed, err := refreshRunningDaemonAfterInstall()
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed {
		t.Fatal("expected daemon lifecycle refresh")
	}
	binary := filepath.Join(root, "bin", "issueops")
	want := [][]string{
		{binary, "daemon", "stop", "--json"},
		{binary, "daemon", "start", "--json"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected installed daemon command sequence:\n got: %#v\nwant: %#v", commands, want)
	}
}

func TestRefreshRunningDaemonAfterInstallStopsOnInstalledStopFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)

	var commands [][]string
	restoreRunner := stubInstalledDaemonCommandRunner(t, func(name string, args ...string) error {
		commands = append(commands, append([]string{name}, args...))
		return errors.New("stop rejected")
	})
	defer restoreRunner()
	restoreList := stubDaemonProcessLister(t, func() ([]daemonProcess, error) {
		t.Fatal("stale process cleanup must not run after a rejected verified stop")
		return nil, nil
	})
	defer restoreList()

	refreshed, err := refreshRunningDaemonAfterInstall()
	if !refreshed || err == nil || !strings.Contains(err.Error(), "stop rejected") {
		t.Fatalf("expected installed stop failure, refreshed=%v err=%v", refreshed, err)
	}
	binary := filepath.Join(root, "bin", "issueops")
	want := [][]string{{binary, "daemon", "stop", "--json"}}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected command sequence after stop failure:\n got: %#v\nwant: %#v", commands, want)
	}
}

func TestRunUpdateAndBootstrapForwardToInstallScript(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
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

func TestRunUpdateUsesResolvedIssueOpsRootOutsideCheckout(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "scripts", "install-native.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCWD) })
	Configure(Deps{IssueOpsRoot: func() string { return root }})
	t.Cleanup(Reset)

	var got string
	restore := stubInstallScriptCommandRunner(t, func(name string, args ...string) error {
		got = name
		return nil
	})
	defer restore()
	restoreDaemon := stubPostInstallDaemonRefresh(t, func() (bool, error) { return false, nil })
	defer restoreDaemon()
	restoreMCP := stubPostInstallMCPProxyRefresh(t, func() (int, error) { return 0, nil })
	defer restoreMCP()

	if err := runUpdate([]string{"--dry-run", "--path-mode=skip"}); err != nil {
		t.Fatal(err)
	}
	if got != script {
		t.Fatalf("update script = %q, want %q", got, script)
	}
}

func TestRunInstallScriptCommandSkipsRuntimeProcessRefreshOnDryRun(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
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
	t.Setenv("ISSUEOPS_ROOT", root)
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
		return []DaemonProcess{{PID: 11, Command: "issueops daemon serve"}}, nil
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
	binary := filepath.Join(root, "bin", "issueops")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if parsed, ok := ParseDaemonProcess("11 "+binary+" daemon --internal", binary); !ok || parsed.PID != 11 {
		t.Fatalf("ParseDaemonProcess = %#v ok=%v", parsed, ok)
	}

	SetMCPProxyProcessLister(func() ([]MCPProxyProcess, error) {
		return []MCPProxyProcess{{PID: 22, Command: "issueops mcp serve"}}, nil
	})
	SetMCPProxyTerminator(func(pid int) error {
		if pid != 22 {
			t.Fatalf("mcp proxy pid = %d", pid)
		}
		return nil
	})
	refreshed, err := RefreshRunningMCPProxiesAfterInstall()
	if err != nil || refreshed != 0 {
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
	previousSupport := mcpProxyOrphanTerminationSupported
	mcpProxyOrphanTerminationSupported = func() bool { return true }
	t.Cleanup(func() {
		mcpProxyOrphanTerminationSupported = previousSupport
	})
	binary := "/repo/bin/issueops"
	processes := []mcpProxyProcess{
		{
			PID: os.Getpid(), ParentPID: 1, Command: binary + " mcp",
			StartTime: "current-start", Executable: binary, IdentityVerified: true,
		},
		{
			PID: 22, ParentPID: 900, Command: binary + " mcp",
			StartTime: "live-start", Executable: binary, IdentityVerified: true,
		},
		{
			PID: 33, ParentPID: 1, Command: binary + " mcp",
			StartTime: "orphan-start", Executable: binary, IdentityVerified: true,
		},
		{
			PID: 44, ParentPID: 1, Command: binary + " mcp",
			StartTime: "", Executable: "", IdentityVerified: false,
		},
		{
			PID: 55, ParentPID: 1, Command: "npm exec @upstash/context7-mcp",
			StartTime: "external-start", Executable: "/usr/local/bin/node", IdentityVerified: true,
		},
	}
	restoreList := stubMCPProxyProcessLister(t, func() ([]mcpProxyProcess, error) {
		return append([]mcpProxyProcess(nil), processes...), nil
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
	if !dryRun.OK || !dryRun.DryRun || dryRun.Matched != 5 || dryRun.Terminated != 0 || len(terminated) != 0 {
		t.Fatalf("dry-run cleanup = %#v terminated=%#v", dryRun, terminated)
	}
	wantDryRunActions := []string{"skip-current", "skip-live-parent", "would-terminate", "skip-unverified", "skip-not-exact"}
	if got := mcpCleanupActions(dryRun.Processes); !reflect.DeepEqual(got, wantDryRunActions) {
		t.Fatalf("dry-run cleanup actions = %#v", dryRun.Processes)
	}

	applied, err := CleanupMCPProxies(false)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.OK || applied.DryRun || applied.Matched != 5 || applied.Terminated != 1 {
		t.Fatalf("apply cleanup = %#v", applied)
	}
	if !reflect.DeepEqual(terminated, []int{33}) {
		t.Fatalf("terminated = %#v", terminated)
	}
	wantApplyActions := []string{"skip-current", "skip-live-parent", "terminated", "skip-unverified", "skip-not-exact"}
	if got := mcpCleanupActions(applied.Processes); !reflect.DeepEqual(got, wantApplyActions) {
		t.Fatalf("apply cleanup actions = %#v", applied.Processes)
	}
}

func TestCleanupMCPProxiesSkipsIdentityChangedBeforeSignal(t *testing.T) {
	previousSupport := mcpProxyOrphanTerminationSupported
	mcpProxyOrphanTerminationSupported = func() bool { return true }
	t.Cleanup(func() {
		mcpProxyOrphanTerminationSupported = previousSupport
	})
	binary := "/repo/bin/issueops"
	first := mcpProxyProcess{
		PID: 33, ParentPID: 1, Command: binary + " mcp",
		StartTime: "orphan-start", Executable: binary, IdentityVerified: true,
	}
	second := first
	second.StartTime = "reused-pid-start"
	listCalls := 0
	restoreList := stubMCPProxyProcessLister(t, func() ([]mcpProxyProcess, error) {
		listCalls++
		if listCalls == 1 {
			return []mcpProxyProcess{first}, nil
		}
		return []mcpProxyProcess{second}, nil
	})
	defer restoreList()
	restoreTerm := stubMCPProxyTerminator(t, func(pid int) error {
		t.Fatalf("identity-changed pid %d must not be signaled", pid)
		return nil
	})
	defer restoreTerm()

	result, err := CleanupMCPProxies(false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Terminated != 0 || len(result.Processes) != 1 ||
		result.Processes[0].Action != "skip-identity-changed" {
		t.Fatalf("identity-changed cleanup = %#v", result)
	}
}

func TestCleanupMCPProxiesSkipsOrphansOnUnsupportedPlatforms(t *testing.T) {
	previousSupport := mcpProxyOrphanTerminationSupported
	mcpProxyOrphanTerminationSupported = func() bool { return false }
	t.Cleanup(func() {
		mcpProxyOrphanTerminationSupported = previousSupport
	})
	binary := "/repo/bin/issueops"
	restoreList := stubMCPProxyProcessLister(t, func() ([]mcpProxyProcess, error) {
		return []mcpProxyProcess{{
			PID:              33,
			ParentPID:        1,
			Command:          binary + " mcp",
			StartTime:        "orphan-start",
			Executable:       binary,
			IdentityVerified: true,
		}}, nil
	})
	defer restoreList()
	restoreTerm := stubMCPProxyTerminator(t, func(pid int) error {
		t.Fatalf("unsupported platform pid %d must not be signaled", pid)
		return nil
	})
	defer restoreTerm()

	result, err := CleanupMCPProxies(false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Terminated != 0 || len(result.Processes) != 1 ||
		result.Processes[0].Action != "skip-unsupported-platform" {
		t.Fatalf("unsupported-platform cleanup = %#v", result)
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

func mcpCleanupActions(processes []MCPCleanupProcess) []string {
	actions := make([]string, 0, len(processes))
	for _, process := range processes {
		actions = append(actions, process.Action)
	}
	return actions
}
