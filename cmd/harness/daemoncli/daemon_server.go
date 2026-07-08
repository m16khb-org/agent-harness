package daemoncli

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxConnections = 64

// mcpIdleTimeout bounds how long a daemon MCP connection may stay idle (no
// reads) before being closed. Without it, server.Run blocks forever on a
// read with no deadline, so abandoned client connections permanently occupy
// a connSlot and exhaust the pool. Refreshed on every Read by idleConn.
var mcpIdleTimeout = func() time.Duration {
	if v := os.Getenv("HARNESS_MCP_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 30 * time.Minute
}()

func runDaemonServer() error {
	return runDaemonServerWithDeps(daemonServerDefaultDeps())
}

var daemonServerDefaultDeps = func() daemonServerDeps {
	return daemonServerDeps{
		paths:    currentDaemonPaths,
		mkdirAll: os.MkdirAll,
		openLog: func(path string) (daemonServerLogFile, error) {
			return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		},
		remove:    os.Remove,
		listen:    net.Listen,
		chmod:     os.Chmod,
		writeFile: os.WriteFile,
		getpid:    os.Getpid,
		now: func() time.Time {
			return time.Now().UTC()
		},
		serveMCPStream: func(conn net.Conn, logFile daemonServerLogFile) error {
			return ServeMCPStream(conn, conn, logFile)
		},
	}
}

type daemonServerLogFile interface {
	io.Writer
	io.Closer
}

type daemonServerDeps struct {
	paths          func() (daemonPaths, error)
	mkdirAll       func(string, os.FileMode) error
	openLog        func(string) (daemonServerLogFile, error)
	remove         func(string) error
	listen         func(network, address string) (net.Listener, error)
	chmod          func(string, os.FileMode) error
	writeFile      func(string, []byte, os.FileMode) error
	getpid         func() int
	now            func() time.Time
	serveMCPStream func(net.Conn, daemonServerLogFile) error
}

func runDaemonServerWithDeps(deps daemonServerDeps) error {
	paths, err := deps.paths()
	if err != nil {
		return err
	}
	if err := deps.mkdirAll(paths.Dir, 0o700); err != nil {
		return err
	}
	logFile, err := deps.openLog(paths.Log)
	if err != nil {
		return err
	}
	defer logFile.Close()
	_ = deps.remove(paths.Socket)
	listener, err := deps.listen("unix", paths.Socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	_ = deps.chmod(paths.Socket, 0o600)
	pid := deps.getpid()
	if err := deps.writeFile(paths.PID, []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		return err
	}
	_ = deps.remove(paths.Lock)
	defer func() {
		_ = deps.remove(paths.Socket)
		_ = deps.remove(paths.PID)
	}()
	connSlots := make(chan struct{}, maxConnections)
	var activeWG sync.WaitGroup
	fmt.Fprintf(logFile, "%s daemon started pid=%d socket=%s max_connections=%d\n", deps.now().Format(time.RFC3339), pid, paths.Socket, maxConnections)
	acceptErr := runDaemonAcceptLoop(listener, logFile, daemonServerLoopDeps{
		now:            deps.now,
		serveMCPStream: deps.serveMCPStream,
		wrapConn: func(c net.Conn) net.Conn {
			return &idleConn{Conn: c, timeout: mcpIdleTimeout}
		},
		connSlots: connSlots,
		activeWG:  &activeWG,
	})
	fmt.Fprintf(logFile, "%s daemon stopping, waiting for active connections\n", deps.now().Format(time.RFC3339))
	shutdownDone := make(chan struct{})
	go func() {
		activeWG.Wait()
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
		fmt.Fprintf(logFile, "%s daemon stopped cleanly\n", deps.now().Format(time.RFC3339))
	case <-time.After(30 * time.Second):
		fmt.Fprintf(logFile, "%s daemon stopped with connections still active after 30s timeout\n", deps.now().Format(time.RFC3339))
	}
	return acceptErr
}

type daemonServerLoopDeps struct {
	now            func() time.Time
	serveMCPStream func(net.Conn, daemonServerLogFile) error
	wrapConn       func(net.Conn) net.Conn
	connSlots      chan struct{}
	activeWG       *sync.WaitGroup
}

func runDaemonAcceptLoop(listener net.Listener, logFile daemonServerLogFile, deps daemonServerLoopDeps) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			fmt.Fprintf(logFile, "%s accept error: %v\n", deps.now().Format(time.RFC3339), err)
			continue
		}
		select {
		case deps.connSlots <- struct{}{}:
		default:
			fmt.Fprintf(logFile, "%s connection limit reached (%d), rejecting connection\n", deps.now().Format(time.RFC3339), maxConnections)
			_, _ = conn.Write([]byte("daemon connection limit reached\n"))
			_ = conn.Close()
			continue
		}
		deps.activeWG.Add(1)
		go func(conn net.Conn) {
			defer func() {
				<-deps.connSlots
				deps.activeWG.Done()
			}()
			defer conn.Close()
			if deps.wrapConn != nil {
				conn = deps.wrapConn(conn)
			}
			if err := deps.serveMCPStream(conn, logFile); err != nil {
				fmt.Fprintf(logFile, "%s mcp stream error: %v\n", deps.now().Format(time.RFC3339), err)
			}
		}(conn)
	}
}

// idleConn wraps a net.Conn and refreshes the read deadline on every Read so
// that abandoned MCP connections (no traffic) hit the idle timeout, cause
// server.Run to return, and release their connSlot instead of blocking on a
// read forever. Active connections refresh the deadline on each Read and are
// unaffected.
type idleConn struct {
	net.Conn
	timeout time.Duration
}

func (c *idleConn) Read(p []byte) (int, error) {
	if err := c.Conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(p)
}
