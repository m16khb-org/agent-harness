package verifyloop

import (
	"testing"
	"time"

	"issueops/cmd/issueops/commandstep"
)

func TestSelfVerifyRunsExactlyOneEvidencePass(t *testing.T) {
	goTestCalls := 0
	stepDeps := fakeVerifyLoopStepDeps("", "")
	runCommandStep := stepDeps.RunCommandStep
	stepDeps.RunCommandStep = func(
		dir string,
		label string,
		timeout time.Duration,
		stdin string,
		name string,
		args ...string,
	) commandstep.StepResult {
		if label == "go test" {
			goTestCalls++
		}
		return runCommandStep(dir, label, timeout, stdin, name, args...)
	}

	result, err := SelfVerify(Request{
		BaseSeed:    100,
		TargetScore: 0,
	}, Deps{
		IssueOpsRoot: func() string { return t.TempDir() },
		StepDeps:     stepDeps,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Iterations != 1 || len(result.Runs) != 1 {
		t.Fatalf("self-verify repeated evidence pass: %+v", result)
	}
	if goTestCalls != 1 {
		t.Fatalf("go test calls=%d want=1", goTestCalls)
	}
}
