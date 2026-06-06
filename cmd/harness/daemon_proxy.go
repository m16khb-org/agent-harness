package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

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
