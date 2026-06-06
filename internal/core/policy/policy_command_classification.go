package policy

import (
	"path/filepath"
	"strings"
)

func commandBase(command string) string {
	return strings.ToLower(filepath.Base(command))
}

func isShellCommand(command string) bool {
	return policyShellInterpreters[commandBase(command)]
}

func commandUsesNetwork(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	base := commandBase(argv[0])
	if policyNetworkCommands[base] {
		return true
	}
	return subcommandAllowed(policyNetworkSubcommands, base, argv)
}

func commandWrites(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	base := commandBase(argv[0])
	if policyWriteCommands[base] {
		return true
	}
	return subcommandAllowed(policyWriteSubcommands, base, argv)
}

func readOnlyAllowed(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	base := commandBase(argv[0])
	if policyReadOnlyCommands[base] {
		return true
	}
	return subcommandAllowed(policyReadOnlySubcommands, base, argv)
}

func subcommandAllowed(catalog map[string]map[string]bool, base string, argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	allowed, ok := catalog[base]
	return ok && allowed[strings.ToLower(argv[1])]
}
