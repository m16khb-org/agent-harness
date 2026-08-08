package workercli

import (
	"flag"
	"fmt"
)

func runWorkerEnqueue(args []string) error {
	fs := flag.NewFlagSet("worker enqueue", flag.ContinueOnError)
	kind := fs.String("kind", "", "job kind")
	payload := fs.String("payload", "", "redacted job payload")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	job, err := EnqueueWorkerJob(*kind, *payload)
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
	job, err := ReadWorkerJob(*id)
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
	result, err := ListWorkerJobs()
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

func runWorkerCleanupStuck(args []string) error {
	fs := flag.NewFlagSet("worker cleanup-stuck", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := DetectStuckWorkerJobs()
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
	job, err := CancelWorkerJob(*id)
	if *jsonOut {
		_ = printJSON(job)
	}
	if err == nil && !*jsonOut {
		fmt.Printf("cancelled %s\n", job.ID)
	}
	return err
}
