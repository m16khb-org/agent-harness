package policy_test

import (
	"context"
	"testing"

	policyport "issueops/internal/port/policy"
)

type fakeRunner struct{}

func (fakeRunner) Run(context.Context, policyport.Command) (policyport.Result, error) {
	return policyport.Result{ExitCode: 0}, nil
}

func TestRunnerOwnsOnlyProcessExecution(t *testing.T) {
	var runner policyport.Runner = fakeRunner{}
	if result, err := runner.Run(context.Background(), policyport.Command{Argv: []string{"true"}}); err != nil || result.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v %v", result, err)
	}
}
