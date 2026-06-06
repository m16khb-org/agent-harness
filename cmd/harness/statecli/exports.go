package statecli

func Run(args []string) error {
	return runState(args)
}

func RunWrite(args []string) error {
	return runStateWrite(args)
}

func RunRead(args []string) error {
	return runStateRead(args)
}

func RunList(args []string) error {
	return runStateList(args)
}

func RunPrune(args []string) error {
	return runStatePrune(args)
}

func RunDoctor(args []string) error {
	return runStateDoctor(args)
}

func RunMigrate(args []string) error {
	return runStateMigrate(args)
}
