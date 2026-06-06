package daemoncli

import (
	"net"
	"os"
	"syscall"
	"time"
)

func checkDaemonStatus() daemonStatus {
	paths, err := currentDaemonPaths()
	if err != nil {
		return daemonStatus{OK: false, Message: err.Error()}
	}
	status := daemonStatus{OK: true, Paths: paths}
	pid := readDaemonPID(paths.PID)
	if pid > 0 {
		status.PID = pid
	}
	conn, err := net.DialTimeout("unix", paths.Socket, 150*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		status.Running = true
		if status.PID == 0 {
			status.PID = pid
		}
		status.Message = "daemon is reachable"
		return status
	}
	if pid > 0 && processAlive(pid) {
		status.Message = "daemon pid exists but socket is not reachable"
	} else {
		status.Message = "daemon is not running"
	}
	return status
}

func stopDaemon() (daemonStatus, error) {
	paths, err := currentDaemonPaths()
	if err != nil {
		return daemonStatus{}, err
	}
	pid := readDaemonPID(paths.PID)
	if pid <= 0 || !processAlive(pid) {
		_ = os.Remove(paths.Socket)
		_ = os.Remove(paths.PID)
		return daemonStatus{OK: true, Running: false, Paths: paths, Message: "agent-harness daemon already stopped"}, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return daemonStatus{OK: false, Running: true, PID: pid, Paths: paths, Message: err.Error()}, err
	}
	_ = proc.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			_ = os.Remove(paths.Socket)
			_ = os.Remove(paths.PID)
			return daemonStatus{OK: true, Running: false, PID: pid, Paths: paths, Message: "agent-harness daemon stopped"}, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = proc.Kill()
	_ = os.Remove(paths.Socket)
	_ = os.Remove(paths.PID)
	return daemonStatus{OK: true, Running: false, PID: pid, Paths: paths, Message: "agent-harness daemon killed after timeout"}, nil
}

func daemonStatusForMCP() daemonStatus {
	return checkDaemonStatus()
}
