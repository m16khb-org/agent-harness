package main

import (
	"os"
	"testing"
)

func TestRunHelp(t *testing.T) {
	if code := run([]string{"--help"}); code != 0 {
		t.Fatalf("run help exit code = %d", code)
	}
}

func TestMainUsesRunExitCode(t *testing.T) {
	oldArgs := os.Args
	oldExit := osExit
	defer func() {
		os.Args = oldArgs
		osExit = oldExit
	}()
	os.Args = []string{"agent-harness", "--help"}
	var got int
	osExit = func(code int) { got = code }

	main()

	if got != 0 {
		t.Fatalf("main exit code = %d", got)
	}
}
