package main

import "time"

type validateStateRoundtripSelfVerifyInput struct {
	binary      string
	root        string
	seed        int64
	tempState   string
	key         string
	env         []string
	started     time.Time
	stdoutParts []string
	commands    []string
	deps        stateRoundtripValidationDeps
}

type stateRoundtripSelfVerifySession struct {
	input       validateStateRoundtripSelfVerifyInput
	stdoutParts []string
	commands    []string
}

func newStateRoundtripSelfVerifySession(input validateStateRoundtripSelfVerifyInput) *stateRoundtripSelfVerifySession {
	return &stateRoundtripSelfVerifySession{
		input:       input,
		stdoutParts: input.stdoutParts,
		commands:    input.commands,
	}
}

func (s *stateRoundtripSelfVerifySession) run(label string, command ...string) StepResult {
	step := s.input.deps.run(s.input.root, label, 30*time.Second, "", s.input.env, command...)
	s.stdoutParts = append(s.stdoutParts, step.Stdout)
	s.commands = append(s.commands, step.Command)
	return step
}

func (s *stateRoundtripSelfVerifySession) fail(errs ...string) StepResult {
	return assertionStepWithOutput("state roundtrip", s.input.started, errs, s.stdoutParts, s.commands)
}

func (s *stateRoundtripSelfVerifySession) combineFailed(step StepResult) StepResult {
	return combineFailedStep("state roundtrip", s.input.started, step, s.stdoutParts, s.commands)
}
