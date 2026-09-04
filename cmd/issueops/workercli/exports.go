package workercli

func Run(args []string) error {
	return runWorker(args)
}

func RunEnqueue(args []string) error {
	return runWorkerEnqueue(args)
}

func RunReadOnly(args []string) error {
	return runWorkerRun(args)
}

func RunStatus(args []string) error {
	return runWorkerStatus(args)
}

func RunList(args []string) error {
	return runWorkerList(args)
}

func RunCancel(args []string) error {
	return runWorkerCancel(args)
}
