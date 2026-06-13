package main

import (
	"os"

	"agent-harness/cmd/harness/harnessapp"
)

var osExit = os.Exit

func main() {
	osExit(run(os.Args[1:]))
}

func run(args []string) int {
	return harnessapp.RunRootCommand(args)
}
