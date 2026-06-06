package main

import (
	"fmt"
)

func runRootCommand(args []string) int {
	if len(args) < 1 {
		usage()
		return 2
	}

	switch args[0] {
	case "help", "--help", "-h":
		usage()
		return 0
	case "version", "--version", "-v":
		fmt.Println("agent-harness", version)
		return 0
	}
	if ok, exitCode := runRootSubcommand(args[0], args[1:]); ok {
		return exitCode
	}
	usage()
	return 2
}
