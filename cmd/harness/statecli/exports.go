package statecli

func Run(deps Dependencies, args []string) error {
	return runState(deps, args)
}

func RunWrite(deps Dependencies, args []string) error {
	return runStateWrite(deps, args)
}

func RunRead(deps Dependencies, args []string) error {
	return runStateRead(deps, args)
}

func RunList(deps Dependencies, args []string) error {
	return runStateList(deps, args)
}

func RunPrune(deps Dependencies, args []string) error {
	return runStatePrune(deps, args)
}

func RunDoctor(deps Dependencies, args []string) error {
	return runStateDoctor(deps, args)
}
