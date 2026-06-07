package runmode

import "fmt"

type Mode struct {
	Full          bool
	Iterations    int
	ContractLabel string
}

func Resolve(full bool, iterationsFlagSet bool, iterations int) (Mode, error) {
	if !full {
		if iterationsFlagSet {
			return Mode{}, fmt.Errorf("--iterations requires --full; default self-verify runs quick one-iteration mode")
		}
		return Mode{Full: false, Iterations: 1, ContractLabel: "quick one-iteration gate"}, nil
	}
	if iterations < 10 {
		return Mode{}, fmt.Errorf("full self-verification requires at least 10 iterations; use --full --iterations=10 or higher")
	}
	return Mode{Full: true, Iterations: iterations, ContractLabel: "full ten-plus-iteration gate"}, nil
}
