package statecli

import (
	"fmt"
	"os"
	"strings"
)

func runState(args []string) error {
	if len(args) == 0 {
		stateUsage()
		return fmt.Errorf("missing state subcommand")
	}
	switch args[0] {
	case "write":
		return runStateWrite(args[1:])
	case "read":
		return runStateRead(args[1:])
	case "list":
		return runStateList(args[1:])
	case "prune":
		return runStatePrune(args[1:])
	case "doctor":
		return runStateDoctor(args[1:])
	case "maintain":
		return runStateMaintain(args[1:])
	default:
		stateUsage()
		return fmt.Errorf("unknown state subcommand %q", args[0])
	}
}

func stateUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness state write --key KEY (--value TEXT|--input FILE|--stdin) [--json]
  agent-harness state read --key KEY [--json]
  agent-harness state list [--json]
  agent-harness state prune --max-age DURATION [--confirm] [--json]
  agent-harness state doctor [--json]
  agent-harness state maintain [--json]
`)
}

func flagProvided(args []string, name string) bool {
	long := "--" + name
	for i, arg := range args {
		if arg == long || strings.HasPrefix(arg, long+"=") {
			return true
		}
		if arg == "-"+name && i < len(args) {
			return true
		}
	}
	return false
}
