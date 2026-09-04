package stateroundtrip

import (
	"fmt"
	"time"
)

func Validate(binary, root string, seed int64) StepResult {
	return validateStateRoundtripWithDeps(binary, root, seed, stateRoundtripValidationDeps{})
}

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	return Validate(binary, root, seed)
}

func validateStateRoundtripWithDeps(binary, root string, seed int64, deps stateRoundtripValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempState, err := deps.mkdirTemp("", "issueops-state-roundtrip-*")
	if err != nil {
		return failedStep("state roundtrip", err)
	}
	defer func() { _ = deps.removeAll(tempState) }()

	key := fmt.Sprintf("self-verify-%d", seed)
	content := fmt.Sprintf("seed=%d\nLore: state roundtrip\n", seed)
	env := []string{"ISSUEOPS_STATE_DIR=" + tempState}
	stateResult := validateStateRoundtripStateCLI(validateStateRoundtripStateInput{
		binary:    binary,
		root:      root,
		tempState: tempState,
		key:       key,
		content:   content,
		env:       env,
		started:   started,
		deps:      deps,
	})
	if !stateResult.step.OK {
		return stateResult.step
	}

	return validateStateRoundtripSelfVerifyDeps(validateStateRoundtripSelfVerifyInput{
		binary:      binary,
		root:        root,
		seed:        seed,
		tempState:   tempState,
		key:         key,
		env:         env,
		started:     started,
		stdoutParts: stateResult.stdoutParts,
		commands:    stateResult.commands,
		deps:        deps,
	})
}
