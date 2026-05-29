package main

import (
	"flag"
	"fmt"
	"os"

	"agent-harness/internal/core"
)

func runWorker(args []string) error {
	if len(args) == 0 {
		workerUsage()
		return fmt.Errorf("missing worker subcommand")
	}
	switch args[0] {
	case "enqueue":
		return runWorkerEnqueue(args[1:])
	case "status":
		return runWorkerStatus(args[1:])
	case "list":
		return runWorkerList(args[1:])
	case "cancel":
		return runWorkerCancel(args[1:])
	default:
		workerUsage()
		return fmt.Errorf("unknown worker subcommand %q", args[0])
	}
}

func workerUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness worker enqueue --kind KIND [--payload TEXT] [--json]
  agent-harness worker status --id ID [--json]
  agent-harness worker list [--json]
  agent-harness worker cancel --id ID [--json]
`)
}

func runWorkerEnqueue(args []string) error {
	fs := flag.NewFlagSet("worker enqueue", flag.ContinueOnError)
	kind := fs.String("kind", "", "job kind")
	payload := fs.String("payload", "", "redacted job payload")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	job, err := core.EnqueueWorkerJob(*kind, *payload)
	if *jsonOut {
		_ = printJSON(job)
	}
	if err == nil && !*jsonOut {
		fmt.Printf("queued %s %s\n", job.ID, job.Kind)
	}
	return err
}

func runWorkerStatus(args []string) error {
	fs := flag.NewFlagSet("worker status", flag.ContinueOnError)
	id := fs.String("id", "", "job id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	job, err := core.ReadWorkerJob(*id)
	if *jsonOut {
		_ = printJSON(job)
	}
	if err == nil && !*jsonOut {
		fmt.Printf("%s %s\n", job.ID, job.Status)
	}
	return err
}

func runWorkerList(args []string) error {
	fs := flag.NewFlagSet("worker list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.ListWorkerJobs()
	if *jsonOut {
		_ = printJSON(result)
	}
	if err == nil && !*jsonOut {
		for _, job := range result.Jobs {
			fmt.Printf("%s %s %s\n", job.ID, job.Status, job.Kind)
		}
	}
	return err
}

func runWorkerCancel(args []string) error {
	fs := flag.NewFlagSet("worker cancel", flag.ContinueOnError)
	id := fs.String("id", "", "job id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	job, err := core.CancelWorkerJob(*id)
	if *jsonOut {
		_ = printJSON(job)
	}
	if err == nil && !*jsonOut {
		fmt.Printf("cancelled %s\n", job.ID)
	}
	return err
}
