package rootcmd

import (
	"fmt"
	"io"
)

type Runner func([]string) error

type Command struct {
	Version       string
	Usage         func()
	Stdout        io.Writer
	Stderr        io.Writer
	Runners       map[string]Runner
	ErrorExitCode func(name string, err error) int
}

func (c Command) Run(args []string) int {
	if len(args) < 1 {
		c.printUsage()
		return 2
	}

	switch args[0] {
	case "help", "--help", "-h":
		c.printUsage()
		return 0
	case "version", "--version", "-v":
		fmt.Fprintln(c.stdout(), "agent-harness", c.Version)
		return 0
	}
	if ok, exitCode := c.runSubcommand(args[0], args[1:]); ok {
		return exitCode
	}
	c.printUsage()
	return 2
}

func (c Command) runSubcommand(name string, args []string) (bool, int) {
	runner, ok := c.Runners[name]
	if !ok {
		return false, 0
	}
	if err := runner(args); err != nil {
		fmt.Fprintln(c.stderr(), name+":", err)
		return true, c.errorExitCode(name, err)
	}
	return true, 0
}

func (c Command) printUsage() {
	if c.Usage == nil {
		return
	}
	c.Usage()
}

func (c Command) stdout() io.Writer {
	if c.Stdout != nil {
		return c.Stdout
	}
	return io.Discard
}

func (c Command) stderr() io.Writer {
	if c.Stderr != nil {
		return c.Stderr
	}
	return io.Discard
}

func (c Command) errorExitCode(name string, err error) int {
	if c.ErrorExitCode == nil {
		return 1
	}
	return c.ErrorExitCode(name, err)
}
