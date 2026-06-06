package main

import "agent-harness/cmd/harness/statecli"

func runState(args []string) error {
	return statecli.Run(args)
}

func runStateWrite(args []string) error {
	return statecli.RunWrite(args)
}

func runStateRead(args []string) error {
	return statecli.RunRead(args)
}

func runStateList(args []string) error {
	return statecli.RunList(args)
}

func runStatePrune(args []string) error {
	return statecli.RunPrune(args)
}

func runStateDoctor(args []string) error {
	return statecli.RunDoctor(args)
}

func runStateMigrate(args []string) error {
	return statecli.RunMigrate(args)
}
