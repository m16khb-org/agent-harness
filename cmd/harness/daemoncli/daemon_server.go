package daemoncli

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"agent-harness/cmd/harness/daemoncli/daemonpaths"
)

const maxConnections = 64

const (
	daemonAdmissionErrorCode    = -32001
	daemonStatusConnectionLimit = "daemon_connection_limit_reached"
)

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
		remove:         os.Remove,
		listen:         net.Listen,
		chmod:          os.Chmod,
		writeInstance:  daemonpaths.WriteInstance,
		getpid:         os.Getpid,
		inspectProcess: daemonpaths.InspectProcess,
		buildSHA:       daemonExecutableSHA,
		newToken:       newDaemonIdentityToken,
		now: func() time.Time {
			return time.Now().UTC()
		},
		serveMCPStream: func(ctx context.Context, conn net.Conn, logFile daemonServerLogFile) error {
			return ServeMCPStreamContext(ctx, conn, conn, logFile)
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
	writeInstance  func(string, daemonInstance) error
	getpid         func() int
	inspectProcess func(int) (daemonProcessIdentity, error)
	buildSHA       func(string) (string, error)
	newToken       func() (string, error)
	now            func() time.Time
	serveMCPStream func(context.Context, net.Conn, daemonServerLogFile) error
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
	defer func() { _ = deps.remove(paths.Socket) }()
	_ = deps.chmod(paths.Socket, 0o600)
	pid := deps.getpid()
	processIdentity, err := deps.inspectProcess(pid)
	if err != nil {
		return fmt.Errorf("inspect daemon process identity: %w", err)
	}
	nonce, err := deps.newToken()
	if err != nil {
		return fmt.Errorf("create daemon instance nonce: %w", err)
	}
	generation, err := deps.newToken()
	if err != nil {
		return fmt.Errorf("create daemon generation: %w", err)
	}
	buildSHA, err := deps.buildSHA(processIdentity.Executable)
	if err != nil {
		return fmt.Errorf("hash daemon executable: %w", err)
	}
	instance := daemonInstance{
		PID:              pid,
		ProcessStartTime: processIdentity.StartTime,
		Executable:       processIdentity.Executable,
		InstanceNonce:    nonce,
		BuildSHA:         buildSHA,
		ProtocolVersion:  daemonProtocolVersion,
		Generation:       generation,
	}
	if err := deps.writeInstance(paths.PID, instance); err != nil {
		return err
	}
	_ = deps.remove(paths.Lock)
	defer func() {
		_ = deps.remove(paths.PID)
	}()
	admission := newDaemonAdmission(maxConnections)
	var activeWG sync.WaitGroup
	fmt.Fprintf(logFile, "%s daemon started pid=%d socket=%s max_connections=%d\n", deps.now().Format(time.RFC3339), pid, paths.Socket, maxConnections)
	acceptErr := runDaemonAcceptLoop(listener, logFile, daemonServerLoopDeps{
		now: deps.now,
		serveConnection: func(conn net.Conn, logFile daemonServerLogFile) error {
			return serveDaemonConnectionWithAdmission(conn, logFile, instance, admission, func(ctx context.Context, conn net.Conn, logFile daemonServerLogFile) error {
				return deps.serveMCPStream(ctx, conn, logFile)
			})
		},
		wrapConn: func(c net.Conn) net.Conn {
			return &idleConn{Conn: c, timeout: mcpIdleTimeout}
		},
		activeWG: &activeWG,
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
	now             func() time.Time
	serveConnection func(net.Conn, daemonServerLogFile) error
	wrapConn        func(net.Conn) net.Conn
	activeWG        *sync.WaitGroup
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
		deps.activeWG.Add(1)
		go func(conn net.Conn) {
			defer deps.activeWG.Done()
			defer conn.Close()
			if deps.wrapConn != nil {
				conn = deps.wrapConn(conn)
			}
			if err := deps.serveConnection(conn, logFile); err != nil {
				fmt.Fprintf(logFile, "%s connection error: %v\n", deps.now().Format(time.RFC3339), err)
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
