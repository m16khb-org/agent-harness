package policy

import "context"

type Command struct {
	Argv    []string
	CWD     string
	Timeout string
	Env     []string
}

type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
}

type Runner interface {
	Run(context.Context, Command) (Result, error)
}
