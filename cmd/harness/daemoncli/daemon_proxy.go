package daemoncli

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

func runMCPProxy() error {
	return runMCPProxyWithDeps(daemonProxyDeps{
		ensureDaemonRunning: ensureDaemonRunning,
		dial: func(network, address string) (io.ReadWriteCloser, error) {
			return net.Dial(network, address)
		},
		stdin:  os.Stdin,
		stdout: os.Stdout,
	})
}

type daemonProxyDeps struct {
	ensureDaemonRunning func() (daemonStatus, error)
	dial                func(network, address string) (io.ReadWriteCloser, error)
	stdin               io.Reader
	stdout              io.Writer
}

func runMCPProxyWithDeps(deps daemonProxyDeps) error {
	status, err := deps.ensureDaemonRunning()
	if err != nil {
		return err
	}
	conn, err := deps.dial("unix", status.Paths.Socket)
	if err != nil {
		return fmt.Errorf("connect daemon: %w", err)
	}
	defer conn.Close()
	stdoutDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(deps.stdout, conn)
		stdoutDone <- err
	}()
	stdinDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(conn, deps.stdin)
		if unixConn, ok := conn.(interface{ CloseWrite() error }); ok {
			_ = unixConn.CloseWrite()
		}
		stdinDone <- err
	}()
	select {
	case stdinErr := <-stdinDone:
		stdoutErr := <-stdoutDone
		if stdinErr != nil && !errors.Is(stdinErr, net.ErrClosed) {
			return stdinErr
		}
		return daemonProxyOutputError(stdoutErr)
	case stdoutErr := <-stdoutDone:
		_ = conn.Close()
		return daemonProxyOutputError(stdoutErr)
	}
}

func daemonProxyOutputError(stdoutErr error) error {
	if stdoutErr != nil && !errors.Is(stdoutErr, net.ErrClosed) && !strings.Contains(stdoutErr.Error(), "use of closed network connection") {
		return stdoutErr
	}
	return nil
}
