package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunInstallScriptExecUsesHarnessRootAndPropagatesExit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HARNESS_ROOT", root)
	script := filepath.Join(root, "record.sh")
	outPath := filepath.Join(root, "pwd.txt")
	body := "#!/bin/sh\npwd > \"$1\"\nexit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runInstallScriptExec(script, outPath); err != nil {
		t.Fatalf("runInstallScriptExec success failed: %v", err)
	}
	pwd, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(pwd)) != root {
		t.Fatalf("script dir = %q, want %q", strings.TrimSpace(string(pwd)), root)
	}

	failScript := filepath.Join(root, "fail.sh")
	if err := os.WriteFile(failScript, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runInstallScriptExec(failScript); err == nil {
		t.Fatal("runInstallScriptExec should propagate non-zero exit")
	}
}

func TestBootstrapProcessListingAndInvalidTerminateErrors(t *testing.T) {
	t.Setenv("HARNESS_ROOT", t.TempDir())

	if _, err := listDaemonProcesses(); err != nil {
		t.Fatalf("listDaemonProcesses failed: %v", err)
	}
	if _, err := listMCPProxyProcesses(); err != nil {
		t.Fatalf("listMCPProxyProcesses failed: %v", err)
	}
	if err := terminateDaemonProcess(-1); err == nil {
		t.Fatal("terminateDaemonProcess should reject invalid pid")
	}
	if err := terminateMCPProxyProcess(-1); err == nil {
		t.Fatal("terminateMCPProxyProcess should reject invalid pid")
	}
}

func TestRefreshRunningDaemonAfterInstallCleansStaleProcessesWhenStopped(t *testing.T) {
	t.Setenv("HARNESS_DAEMON_DIR", t.TempDir())
	restoreList := stubDaemonProcessLister(t, func() ([]daemonProcess, error) {
		return []daemonProcess{
			{PID: os.Getpid(), Command: "agent-harness daemon --internal"},
			{PID: 30101, Command: "agent-harness daemon --internal"},
			{PID: 30102, Command: "agent-harness daemon --internal"},
		}, nil
	})
	defer restoreList()
	var terminated []int
	restoreTerminate := stubDaemonProcessTerminator(t, func(pid int) error {
		terminated = append(terminated, pid)
		return nil
	})
	defer restoreTerminate()

	changed, err := refreshRunningDaemonAfterInstall()
	if err != nil {
		t.Fatalf("refreshRunningDaemonAfterInstall failed: %v", err)
	}
	if !changed || !reflect.DeepEqual(terminated, []int{30101, 30102}) {
		t.Fatalf("unexpected stopped daemon refresh changed=%v terminated=%v", changed, terminated)
	}
}

func TestRefreshRunningDaemonAfterInstallPropagatesStaleProcessListError(t *testing.T) {
	t.Setenv("HARNESS_DAEMON_DIR", t.TempDir())
	want := errors.New("daemon process list failed")
	restoreList := stubDaemonProcessLister(t, func() ([]daemonProcess, error) {
		return nil, want
	})
	defer restoreList()

	changed, err := refreshRunningDaemonAfterInstall()
	if changed || !errors.Is(err, want) {
		t.Fatalf("refreshRunningDaemonAfterInstall changed=%v err=%v, want %v", changed, err, want)
	}
}
