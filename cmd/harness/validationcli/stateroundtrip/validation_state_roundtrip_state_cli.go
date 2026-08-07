package stateroundtrip

import (
	statecontract "agent-harness/internal/contract/state"
	"encoding/json"
	"time"
)

type validateStateRoundtripStateInput struct {
	binary    string
	root      string
	tempState string
	key       string
	content   string
	env       []string
	started   time.Time
	deps      stateRoundtripValidationDeps
}

type validateStateRoundtripStateResult struct {
	step        StepResult
	stdoutParts []string
	commands    []string
}

type stateRoundtripStateSession struct {
	input       validateStateRoundtripStateInput
	stdoutParts []string
	commands    []string
}

func validateStateRoundtripStateCLI(input validateStateRoundtripStateInput) validateStateRoundtripStateResult {
	session := stateRoundtripStateSession{input: input}
	if step := session.validateWriteReadList(); !step.OK {
		return session.result(step)
	}
	if step := session.validatePrune(); !step.OK {
		return session.result(step)
	}
	return session.result(StepResult{OK: true})
}

func (s *stateRoundtripStateSession) run(label string, command ...string) StepResult {
	step := s.input.deps.run(s.input.root, label, 30*time.Second, "", s.input.env, command...)
	s.stdoutParts = append(s.stdoutParts, step.Stdout)
	s.commands = append(s.commands, step.Command)
	return step
}

func (s *stateRoundtripStateSession) fail(errs ...string) StepResult {
	return assertionStepWithOutput("state roundtrip", s.input.started, errs, s.stdoutParts, s.commands)
}

func (s *stateRoundtripStateSession) combineFailed(step StepResult) StepResult {
	return combineFailedStep("state roundtrip", s.input.started, step, s.stdoutParts, s.commands)
}

func (s *stateRoundtripStateSession) result(step StepResult) validateStateRoundtripStateResult {
	return validateStateRoundtripStateResult{step: step, stdoutParts: s.stdoutParts, commands: s.commands}
}

func (s *stateRoundtripStateSession) validateWriteReadList() StepResult {
	write := s.run("state write", s.input.binary, "state", "write", "--key", s.input.key, "--value", s.input.content, "--json")
	if !write.OK {
		return s.combineFailed(write)
	}
	var writeResult statecontract.StateResult
	if err := json.Unmarshal([]byte(write.Stdout), &writeResult); err != nil {
		return s.fail(err.Error())
	}
	if !writeResult.OK || writeResult.Record.Key != s.input.key || writeResult.Record.Content != s.input.content || writeResult.Record.Bytes != len([]byte(s.input.content)) {
		return s.fail("write result did not match expected record")
	}

	read := s.run("state read", s.input.binary, "state", "read", "--key", s.input.key, "--json")
	if !read.OK {
		return s.combineFailed(read)
	}
	var readResult statecontract.StateResult
	if err := json.Unmarshal([]byte(read.Stdout), &readResult); err != nil {
		return s.fail(err.Error())
	}
	if !readResult.OK || readResult.Record.Key != s.input.key || readResult.Record.Content != s.input.content || readResult.Record.Bytes != len([]byte(s.input.content)) {
		return s.fail("read result did not match expected record")
	}

	list := s.run("state list", s.input.binary, "state", "list", "--json")
	if !list.OK {
		return s.combineFailed(list)
	}
	var listResult statecontract.StateListResult
	if err := json.Unmarshal([]byte(list.Stdout), &listResult); err != nil {
		return s.fail(err.Error())
	}
	if !listResult.OK || !containsString(listResult.Keys, s.input.key) {
		return s.fail("state list did not include roundtrip key")
	}
	return StepResult{OK: true}
}
