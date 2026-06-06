package updatecli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"agent-harness/cmd/harness/daemoncli"
)

var postInstallDaemonRefresh = refreshRunningDaemonAfterInstall
var daemonProcessLister = listDaemonProcesses
var daemonProcessTerminator = terminateDaemonProcess

type daemonProcess struct {
	PID     int
	Command string
}

func refreshRunningDaemonAfterInstall() (bool, error) {
	status := daemoncli.CheckDaemonStatus()
	if !status.Running {
		terminated, err := terminateStaleDaemonProcesses()
		if err != nil {
			return false, err
		}
		return terminated > 0, nil
	}
	if _, err := daemoncli.StopDaemon(); err != nil {
		return true, err
	}
	if _, err := terminateStaleDaemonProcesses(); err != nil {
		return true, err
	}
	if _, err := daemoncli.EnsureDaemonRunning(); err != nil {
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
	binary := filepath.Join(HarnessRoot(), "bin", "agent-harness")
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
