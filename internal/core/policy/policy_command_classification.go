package policy

import (
	"path/filepath"
	"strings"
)

func commandBase(command string) string {
	return strings.ToLower(filepath.Base(command))
}

func (catalog policyCatalog) isShellCommand(command string) bool {
	return catalog.shellInterpreters[commandBase(command)]
}

func (catalog policyCatalog) commandUsesNetwork(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	base := commandBase(argv[0])
	if catalog.networkCommands[base] {
		return true
	}
	return subcommandAllowed(catalog.networkSubcommands, base, argv)
}

func (catalog policyCatalog) commandWrites(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	base := commandBase(argv[0])
	if catalog.writeCommands[base] {
		return true
	}
	return subcommandAllowed(catalog.writeSubcommands, base, argv)
}

func (catalog policyCatalog) readOnlyAllowed(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	base := commandBase(argv[0])
	if catalog.readOnlyCommands[base] {
		return true
	}
	return subcommandAllowed(catalog.readOnlySubcommands, base, argv)
}

func subcommandAllowed(catalog map[string]map[string]bool, base string, argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	allowed, ok := catalog[base]
	return ok && allowed[strings.ToLower(argv[1])]
}
