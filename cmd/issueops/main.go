package main

import (
	"os"

	"issueops/cmd/issueops/issueopsapp"
)

var osExit = os.Exit

func main() {
	osExit(run(os.Args[1:]))
}

func run(args []string) int {
	return issueopsapp.RunRootCommand(args)
}
