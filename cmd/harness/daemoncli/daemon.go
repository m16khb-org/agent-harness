package daemoncli

import (
	"flag"
	"fmt"
	"os"
	"time"

	"agent-harness/cmd/harness/daemoncli/daemonpaths"
)

type daemonPaths = daemonpaths.Paths
type daemonInstance = daemonpaths.InstanceRecord
type daemonProcessIdentity = daemonpaths.ProcessIdentity

type daemonStatus struct {
	OK                bool            `json:"ok"`
	Running           bool            `json:"running"`
	Reachable         bool            `json:"reachable"`
	IdentityVerified  bool            `json:"identity_verified"`
	ActiveConnections int             `json:"active_connections"`
	MaxConnections    int             `json:"max_connections"`
	Accepting         bool            `json:"accepting"`
	Draining          bool            `json:"draining"`
	LegacyPID         bool            `json:"legacy_pid,omitempty"`
	PID               int             `json:"pid,omitempty"`
	Code              string          `json:"code"`
	Paths             daemonPaths     `json:"paths"`
	Instance          *daemonInstance `json:"instance,omitempty"`
	Message           string          `json:"message,omitempty"`
}

const daemonReadyTimeout = 15 * time.Second

func currentDaemonPaths() (daemonPaths, error) {
	return daemonpaths.Current()
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
		if daemonStatusIsReady(status) {
			fmt.Printf("running pid=%d socket=%s\n", status.PID, status.Paths.Socket)
		} else if status.Running || status.Reachable || status.PID > 0 {
			fmt.Printf("unverified code=%s pid=%d socket=%s\n", status.Code, status.PID, status.Paths.Socket)
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
