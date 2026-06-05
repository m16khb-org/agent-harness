package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var installScriptCommandRunner = runInstallScriptExec
var postInstallDaemonRefresh = refreshRunningDaemonAfterInstall
var postInstallMCPProxyRefresh = refreshRunningMCPProxiesAfterInstall
var daemonProcessLister = listDaemonProcesses
var daemonProcessTerminator = terminateDaemonProcess
var mcpProxyProcessLister = listMCPProxyProcesses
var mcpProxyTerminator = terminateMCPProxyProcess

type daemonProcess struct {
	PID     int
	Command string
}

type mcpProxyProcess struct {
	PID     int
	Command string
}

func runUpdate(args []string) error {
	return runInstallScriptCommand("update", args)
}

func runBootstrap(args []string) error {
	return runInstallScriptCommand("bootstrap", args)
}

func runInstallScriptCommand(commandName string, args []string) error {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	sync := fs.Bool("sync", false, "sync optional upstream companion tool versions")
	explicitWithUpstream := hasInstallFlag(args, "with-upstream-tools")
	withUpstream := fs.Bool("with-upstream-tools", true, "install/update upstream llm-wiki, codegraph, and claude-mem integrations")
	skipUpstream := fs.Bool("skip-upstream-tools", false, "skip upstream companion tool setup")
	withoutUpstream := fs.Bool("without-upstream-tools", false, "skip upstream companion tool setup")
	projectLocal := fs.Bool("project-local", false, "also write explicit project-local files")
	dryRun := fs.Bool("dry-run", false, "show install plan without writing")
	pathMode := fs.String("path-mode", "", "manage ~/.local/bin PATH setup: auto, manual, or skip")
	interactive := fs.Bool("interactive", false, "ask for install choices before applying the plan")
	jsonOut := fs.Bool("json", false, "print JSON from install-native")
	skipBuild := fs.Bool("skip-build", false, "do not rebuild bin/agent-harness")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	script := filepath.Join(harnessRoot(), "scripts", "install-native.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("install script not found at %s: %w", script, err)
	}

	scriptArgs := make([]string, 0, 5)
	if *sync || explicitWithUpstream || (!*skipUpstream && !*withoutUpstream && *withUpstream && commandName == "update") {
		scriptArgs = append(scriptArgs, "--with-upstream-tools")
	} else {
		scriptArgs = append(scriptArgs, "--skip-upstream-tools")
	}
	if *projectLocal {
		scriptArgs = append(scriptArgs, "--project-local")
	}
	if *dryRun {
		scriptArgs = append(scriptArgs, "--dry-run")
	}
	if *pathMode != "" {
		scriptArgs = append(scriptArgs, "--path-mode="+*pathMode)
	}
	if *interactive {
		scriptArgs = append(scriptArgs, "--interactive")
	}
	if *jsonOut {
		scriptArgs = append(scriptArgs, "--json")
	}
	if *skipBuild {
		scriptArgs = append(scriptArgs, "--skip-build")
	}

	if err := installScriptCommandRunner(script, scriptArgs...); err != nil {
		return err
	}
	if *dryRun {
		return nil
	}
	if _, err := postInstallDaemonRefresh(); err != nil {
		return err
	}
	_, err := postInstallMCPProxyRefresh()
	return err
}

func hasInstallFlag(args []string, name string) bool {
	long := "--" + name
	for _, arg := range args {
		if arg == long || len(arg) > len(long) && arg[:len(long)+1] == long+"=" {
			return true
		}
	}
	return false
}

func runInstallScriptExec(script string, args ...string) error {
	cmd := exec.Command(script, args...)
	cmd.Dir = harnessRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func refreshRunningDaemonAfterInstall() (bool, error) {
	status := checkDaemonStatus()
	if !status.Running {
		terminated, err := terminateStaleDaemonProcesses()
		if err != nil {
			return false, err
		}
		return terminated > 0, nil
	}
	if _, err := stopDaemon(); err != nil {
		return true, err
	}
	if _, err := terminateStaleDaemonProcesses(); err != nil {
		return true, err
	}
	if _, err := ensureDaemonRunning(); err != nil {
		return true, err
	}
	return true, nil
}

func terminateStaleDaemonProcesses() (int, error) {
	processes, err := daemonProcessLister()
	if err != nil {
		return 0, err
	}
	currentPID := os.Getpid()
	terminated := 0
	for _, process := range processes {
		if process.PID == currentPID {
			continue
		}
		if err := daemonProcessTerminator(process.PID); err != nil {
			return terminated, err
		}
		terminated++
	}
	return terminated, nil
}

func listDaemonProcesses() ([]daemonProcess, error) {
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return nil, err
	}
	binary := filepath.Join(harnessRoot(), "bin", "agent-harness")
	var processes []daemonProcess
	for _, line := range strings.Split(string(out), "\n") {
		process, ok := parseDaemonProcess(line, binary)
		if ok {
			processes = append(processes, process)
		}
	}
	return processes, nil
}

func parseDaemonProcess(line, binary string) (daemonProcess, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return daemonProcess{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return daemonProcess{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return daemonProcess{}, false
	}
	command := strings.Join(fields[1:], " ")
	if command != binary+" daemon --internal" {
		return daemonProcess{}, false
	}
	return daemonProcess{PID: pid, Command: command}, true
}

func terminateDaemonProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}

func refreshRunningMCPProxiesAfterInstall() (int, error) {
	processes, err := mcpProxyProcessLister()
	if err != nil {
		return 0, err
	}
	currentPID := os.Getpid()
	terminated := 0
	for _, process := range processes {
		if process.PID == currentPID {
			continue
		}
		if err := mcpProxyTerminator(process.PID); err != nil {
			return terminated, err
		}
		terminated++
	}
	return terminated, nil
}

func listMCPProxyProcesses() ([]mcpProxyProcess, error) {
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return nil, err
	}
	binary := filepath.Join(harnessRoot(), "bin", "agent-harness")
	var processes []mcpProxyProcess
	for _, line := range strings.Split(string(out), "\n") {
		process, ok := parseMCPProxyProcess(line, binary)
		if ok {
			processes = append(processes, process)
		}
	}
	return processes, nil
}

func parseMCPProxyProcess(line, binary string) (mcpProxyProcess, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return mcpProxyProcess{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return mcpProxyProcess{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return mcpProxyProcess{}, false
	}
	command := strings.Join(fields[1:], " ")
	if command != binary+" mcp" {
		return mcpProxyProcess{}, false
	}
	return mcpProxyProcess{PID: pid, Command: command}, true
}

func terminateMCPProxyProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
