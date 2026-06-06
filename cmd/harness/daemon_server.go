package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func runDaemonServer() error {
	return runDaemonServerWithDeps(daemonServerDeps{
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
			return serveMCPStream(conn, conn, logFile)
		},
	})
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
	fmt.Fprintf(logFile, "%s daemon started pid=%d socket=%s\n", deps.now().Format(time.RFC3339), pid, paths.Socket)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			fmt.Fprintf(logFile, "%s accept error: %v\n", deps.now().Format(time.RFC3339), err)
			continue
		}
		go func(conn net.Conn) {
			defer conn.Close()
			if err := deps.serveMCPStream(conn, logFile); err != nil {
				fmt.Fprintf(logFile, "%s mcp stream error: %v\n", deps.now().Format(time.RFC3339), err)
			}
		}(conn)
	}
}
