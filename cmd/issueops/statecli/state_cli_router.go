package statecli

import (
	"fmt"
	"os"
	"strings"
)

func runState(deps Dependencies, args []string) error {
	if len(args) == 0 {
		stateUsage()
		return fmt.Errorf("missing state subcommand")
	}
	switch args[0] {
	case "write":
		return runStateWrite(deps, args[1:])
	case "read":
		return runStateRead(deps, args[1:])
	case "list":
		return runStateList(deps, args[1:])
	case "prune":
		return runStatePrune(deps, args[1:])
	case "doctor":
		return runStateDoctor(deps, args[1:])
	case "maintain":
		return runStateMaintain(deps, args[1:])
	default:
		stateUsage()
		return fmt.Errorf("unknown state subcommand %q", args[0])
	}
}

func stateUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  issueops state write --key KEY (--value TEXT|--input FILE|--stdin) [--json]
  issueops state read --key KEY [--json]
  issueops state list [--json]
  issueops state prune --max-age DURATION [--confirm] [--json]
  issueops state doctor [--json]
  issueops state maintain [--json]
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
