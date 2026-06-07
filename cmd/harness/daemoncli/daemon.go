package daemoncli

import (
	"flag"
	"fmt"
	"os"
	"time"

	"agent-harness/cmd/harness/daemoncli/daemonpaths"
)

type daemonPaths = daemonpaths.Paths

type daemonStatus struct {
	OK      bool        `json:"ok"`
	Running bool        `json:"running"`
	PID     int         `json:"pid,omitempty"`
	Paths   daemonPaths `json:"paths"`
	Message string      `json:"message,omitempty"`
}

const daemonReadyTimeout = 15 * time.Second

func currentDaemonPaths() (daemonPaths, error) {
	return daemonpaths.Current()
}

func readDaemonPID(path string) int {
	return daemonpaths.ReadPID(path)
}

func processAlive(pid int) bool {
	return daemonpaths.ProcessAlive(pid)
}

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
