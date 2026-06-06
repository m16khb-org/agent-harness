package main

import "agent-harness/cmd/harness/workercli"

func init() {
	workercli.ResolveTarget = resolveTarget
}

func runWorker(args []string) error {
	return workercli.Run(args)
}

func runWorkerEnqueue(args []string) error {
	return workercli.RunEnqueue(args)
}

func runWorkerDraftWiki(args []string) error {
	return workercli.RunDraftWiki(args)
}

func runWorkerRun(args []string) error {
	return workercli.RunReadOnly(args)
}

func runWorkerStatus(args []string) error {
	return workercli.RunStatus(args)
}

func runWorkerList(args []string) error {
	return workercli.RunList(args)
}

func runWorkerCancel(args []string) error {
	return workercli.RunCancel(args)
}
