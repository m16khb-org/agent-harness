package main

import (
	"os"

	"agent-harness/cmd/harness/harnessapp"
)

func main() {
	os.Exit(harnessapp.RunRootCommand(os.Args[1:]))
}
