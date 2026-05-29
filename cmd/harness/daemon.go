package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type daemonPaths struct {
	Dir    string `json:"dir"`
	Socket string `json:"socket"`
	PID    string `json:"pid_file"`
	Lock   string `json:"lock_file"`
	Log    string `json:"log_file"`
}

type daemonStatus struct {
	OK      bool        `json:"ok"`
	Running bool        `json:"running"`
	PID     int         `json:"pid,omitempty"`
	Paths   daemonPaths `json:"paths"`
	Message string      `json:"message,omitempty"`
}

const daemonReadyTimeout = 15 * time.Second

func runDaemon(args []string) error {
	if len(args) > 0 && args[0] == "--internal" {
		return runDaemonServer()
	}
	if len(args) == 0 {
		daemonUsage()
		return fmt.Errorf("missing daemon subcommand")
	}
	sub := args[0]
	fs := flag.NewFlagSet("daemon "+sub, flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch sub {
	case "start":
		status, err := ensureDaemonRunning()
		if *jsonOut {
			if printErr := printJSON(status); printErr != nil {
				return printErr
			}
			return err
		}
		if err != nil {
			return err
		}
		fmt.Printf("agent-harness daemon running pid=%d socket=%s\n", status.PID, status.Paths.Socket)
		return nil
	case "status":
		status := checkDaemonStatus()
		if *jsonOut {
			return printJSON(status)
		}
		if status.Running {
			fmt.Printf("running pid=%d socket=%s\n", status.PID, status.Paths.Socket)
		} else {
			fmt.Printf("stopped socket=%s\n", status.Paths.Socket)
		}
		return nil
	case "stop":
		status, err := stopDaemon()
		if *jsonOut {
			if printErr := printJSON(status); printErr != nil {
				return printErr
			}
			return err
		}
		if err != nil {
			return err
		}
		fmt.Println(status.Message)
		return nil
	default:
		daemonUsage()
		return fmt.Errorf("unknown daemon subcommand %q", sub)
	}
}

func daemonUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness daemon start [--json]
  agent-harness daemon status [--json]
  agent-harness daemon stop [--json]
`)
}

func runMCPProxy() error {
	status, err := ensureDaemonRunning()
	if err != nil {
		return err
	}
	conn, err := net.Dial("unix", status.Paths.Socket)
	if err != nil {
		return fmt.Errorf("connect daemon: %w", err)
	}
	defer conn.Close()
	stdoutDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(os.Stdout, conn)
		stdoutDone <- err
	}()
	_, stdinErr := io.Copy(conn, os.Stdin)
	if unixConn, ok := conn.(*net.UnixConn); ok {
		_ = unixConn.CloseWrite()
	}
	stdoutErr := <-stdoutDone
	if stdinErr != nil && !errors.Is(stdinErr, net.ErrClosed) {
		return stdinErr
	}
	if stdoutErr != nil && !errors.Is(stdoutErr, net.ErrClosed) && !strings.Contains(stdoutErr.Error(), "use of closed network connection") {
		return stdoutErr
	}
	return nil
}

func runDaemonServer() error {
	paths, err := currentDaemonPaths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		return err
	}
	logFile, err := os.OpenFile(paths.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	_ = os.Remove(paths.Socket)
	listener, err := net.Listen("unix", paths.Socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	_ = os.Chmod(paths.Socket, 0o600)
	pid := os.Getpid()
	if err := os.WriteFile(paths.PID, []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		return err
	}
	_ = os.Remove(paths.Lock)
	defer func() {
		_ = os.Remove(paths.Socket)
		_ = os.Remove(paths.PID)
	}()
	fmt.Fprintf(logFile, "%s daemon started pid=%d socket=%s\n", time.Now().UTC().Format(time.RFC3339), pid, paths.Socket)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			fmt.Fprintf(logFile, "%s accept error: %v\n", time.Now().UTC().Format(time.RFC3339), err)
			continue
		}
		go func(conn net.Conn) {
			defer conn.Close()
			if err := serveMCPStream(conn, conn, logFile); err != nil {
				fmt.Fprintf(logFile, "%s mcp stream error: %v\n", time.Now().UTC().Format(time.RFC3339), err)
			}
		}(conn)
	}
}

func ensureDaemonRunning() (daemonStatus, error) {
	if status := checkDaemonStatus(); status.Running {
		return status, nil
	}
	paths, err := currentDaemonPaths()
	if err != nil {
		return daemonStatus{}, err
	}
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		return daemonStatus{}, err
	}
	lock, err := acquireDaemonLock(paths)
	if err != nil {
		// Another launcher may be starting it. Wait briefly.
		if status, waitErr := waitForDaemon(paths, daemonReadyTimeout); waitErr == nil && status.Running {
			return status, nil
		}
		return daemonStatus{OK: false, Running: false, Paths: paths, Message: err.Error()}, err
	}
	defer func() {
		_ = lock.Close()
		_ = os.Remove(paths.Lock)
	}()
	if status := checkDaemonStatus(); status.Running {
		return status, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return daemonStatus{}, err
	}
	cmd := exec.Command(exe, "daemon", "--internal")
	cmd.Env = append(os.Environ(), "HARNESS_DAEMON_DIR="+paths.Dir, "HARNESS_ROOT="+harnessRoot())
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return daemonStatus{OK: false, Paths: paths, Message: err.Error()}, err
	}
	_ = cmd.Process.Release()
	return waitForDaemon(paths, daemonReadyTimeout)
}

func waitForDaemon(paths daemonPaths, timeout time.Duration) (daemonStatus, error) {
	deadline := time.Now().Add(timeout)
	var last daemonStatus
	for time.Now().Before(deadline) {
		last = checkDaemonStatus()
		if last.Running {
			return last, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if last.Paths.Dir == "" {
		last.Paths = paths
	}
	last.Message = "daemon did not become ready before timeout"
	return last, errors.New(last.Message)
}

func acquireDaemonLock(paths daemonPaths) (*os.File, error) {
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(paths.Lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(strconv.Itoa(os.Getpid()) + "\n")
			return f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if isStaleDaemonLock(paths.Lock, 30*time.Second) {
			_ = os.Remove(paths.Lock)
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("cannot acquire daemon lock %s", paths.Lock)
}

func isStaleDaemonLock(path string, maxAge time.Duration) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	if time.Since(info.ModTime()) > maxAge {
		return true
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return false
	}
	return !processAlive(pid)
}

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

func currentDaemonPaths() (daemonPaths, error) {
	dir := strings.TrimSpace(os.Getenv("HARNESS_DAEMON_DIR"))
	if dir == "" {
		if state := strings.TrimSpace(os.Getenv("HARNESS_STATE_DIR")); state != "" {
			dir = filepath.Join(state, "daemon")
		} else if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = filepath.Join(home, ".local", "state", "agent-harness", "daemon")
		} else {
			return daemonPaths{}, fmt.Errorf("cannot resolve daemon directory")
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return daemonPaths{}, err
	}
	abs = filepath.Clean(abs)
	return daemonPaths{
		Dir:    abs,
		Socket: filepath.Join(abs, "agent-harness.sock"),
		PID:    filepath.Join(abs, "agent-harness.pid"),
		Lock:   filepath.Join(abs, "agent-harness.lock"),
		Log:    filepath.Join(abs, "agent-harness.log"),
	}, nil
}

func readDaemonPID(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return pid
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func daemonStatusForMCP() daemonStatus {
	return checkDaemonStatus()
}
