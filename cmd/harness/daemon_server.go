package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

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
