package steps

import (
	"strings"
	"testing"
	"time"
)

func TestPlannedSelfVerifyStepsPreservesExecutionOrder(t *testing.T) {
	var goTestStep StepResult

	steps := PlannedSelfVerifySteps("/repo", "/tmp/agent-harness", 100, &goTestStep, fakeSelfVerifyStepDeps(t))

	got := make([]string, 0, len(steps))
	for _, step := range steps {
		got = append(got, step.Label)
	}
	want := []string{
		"harness invariants",
		"risk QA tier",
		"go test",
		"contract golden tests",
		"go build",
		"binary drift",
		"inspect smoke",
		"docs index smoke",
		"candidate export",
		"step budget baseline",
		"install dry-run smoke",
		"command policy smoke",
		"command audit smoke",
		"contract check",
		"tool contract conformance",
		"worker lifecycle smoke",
		"MCP smoke",
		"state roundtrip",
		"parallel isolation",
		"daemon resilience",
		"preflight fuzz",
		"web fetch battery",
		"native integration",
		"redaction audit",
		"QA gate",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d steps, got %d: %v", len(want), len(got), got)
	}
	for i, label := range want {
		if got[i] != label {
			t.Fatalf("step %d label = %q, want %q; all labels: %v", i, got[i], label, got)
		}
	}
}

func TestPlannedSelfVerifyStepsUsesCachedContractGoldenAfterGoTest(t *testing.T) {
	goTestStep := StepResult{Label: "go test", OK: true}

	steps := PlannedSelfVerifySteps("/repo", "/tmp/agent-harness", 100, &goTestStep, fakeSelfVerifyStepDeps(t))
	got := steps[3].Run()

	if !got.OK || got.Label != "contract golden tests" || got.Command != "covered by go test ./... -count=1" {
		t.Fatalf("expected cached contract golden result, got %#v", got)
	}
}

func TestPlannedSelfVerifyStepsUsesSuccessfulRaceAsFullTestEvidence(t *testing.T) {
	var goTestStep StepResult
	goTestCalls := 0
	deps := fakeSelfVerifyStepDeps(t)
	deps.ValidateRiskQATier = func(string) RiskQAEvidence {
		return RiskQAEvidence{
			Step: StepResult{
				Label:   "risk QA tier",
				Command: "go test -race ./... -count=1",
				OK:      true,
			},
			CoversFullGoTest: true,
		}
	}
	deps.RunCommandStep = func(_ string, label string, _ time.Duration, _ string, _ string, _ ...string) StepResult {
		if label == "go test" {
			goTestCalls++
		}
		return StepResult{Label: label, OK: true}
	}

	planned := PlannedSelfVerifySteps("/repo", "/tmp/harness", 100, &goTestStep, deps)
	riskStep := planned[1].Run()
	testStep := planned[2].Run()

	if !riskStep.OK || !testStep.OK {
		t.Fatalf("risk/test steps failed: risk=%+v test=%+v", riskStep, testStep)
	}
	if goTestCalls != 0 {
		t.Fatalf("regular full test calls=%d want=0", goTestCalls)
	}
	if !strings.Contains(testStep.Stdout, "race") {
		t.Fatalf("cached test evidence did not identify race coverage: %+v", testStep)
	}
}

func TestPlannedSelfVerifyStepsRunsEveryPlannedClosure(t *testing.T) {
	var goTestStep StepResult
	steps := PlannedSelfVerifySteps("/repo", "/tmp/agent-harness", 100, &goTestStep, fakeSelfVerifyStepDeps(t))

	for _, step := range steps {
		got := step.Run()
		if !got.OK {
			t.Fatalf("%s returned non-ok result: %#v", step.Label, got)
		}
	}

	if goTestStep.Label != "go test" || !goTestStep.OK {
		t.Fatalf("go test step was not captured: %#v", goTestStep)
	}
}

func TestPlannedSelfVerifyStepsGivesGoTestFullGateTimeout(t *testing.T) {
	var goTestStep StepResult
	var gotTimeout time.Duration
	deps := fakeSelfVerifyStepDeps(t)
	deps.RunCommandStep = func(_ string, label string, timeout time.Duration, _ string, _ string, _ ...string) StepResult {
		if label == "go test" {
			gotTimeout = timeout
		}
		return StepResult{Label: label, OK: true}
	}

	steps := PlannedSelfVerifySteps("/repo", "/tmp/agent-harness", 100, &goTestStep, deps)
	_ = steps[1].Run()
	got := steps[2].Run()

	if !got.OK || got.Label != "go test" {
		t.Fatalf("go test step returned unexpected result: %#v", got)
	}
	if gotTimeout != 10*time.Minute {
		t.Fatalf("go test timeout = %s, want 10m0s", gotTimeout)
	}
}

func TestPlannedSelfVerifyStepsUsesStaticDoctorForBinaryDrift(t *testing.T) {
	var goTestStep StepResult
	var gotTimeout time.Duration
	var gotExecutable string
	var gotArgs []string
	deps := fakeSelfVerifyStepDeps(t)
	deps.RunCommandStep = func(_ string, label string, timeout time.Duration, _ string, executable string, args ...string) StepResult {
		if label == "binary drift" {
			gotTimeout = timeout
			gotExecutable = executable
			gotArgs = append([]string(nil), args...)
		}
		return StepResult{Label: label, OK: true}
	}

	steps := PlannedSelfVerifySteps("/repo", "/tmp/agent-harness", 100, &goTestStep, deps)
	got := steps[5].Run()
	if !got.OK || got.Label != "binary drift" {
		t.Fatalf("binary drift step returned unexpected result: %#v", got)
	}
	if gotTimeout != 10*time.Second {
		t.Fatalf("binary drift timeout = %s, want 10s", gotTimeout)
	}
	if gotExecutable != "/tmp/agent-harness" {
		t.Fatalf("binary drift executable = %q, want temp binary", gotExecutable)
	}
	wantArgs := []string{"doctor", "--static-only", "--json", "--repo", "/repo"}
	if strings.Join(gotArgs, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("binary drift args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestCachedContractGoldenStepFallsBackWhenGoTestDidNotPass(t *testing.T) {
	step := CachedContractGoldenStep(StepResult{Label: "go test", OK: false}, fakeSelfVerifyStepDeps(t))
	if !step.OK || step.Label != "contract golden tests" {
		t.Fatalf("expected fallback contract golden step, got %#v", step)
	}
}

func TestCachedContractGoldenStepUsesFullGoTestEvidence(t *testing.T) {
	step := CachedContractGoldenStep(StepResult{Label: "go test", Command: "go test ./... -count=1", OK: true}, fakeSelfVerifyStepDeps(t))
	if !step.OK || step.Label != "contract golden tests" {
		t.Fatalf("unexpected cached step: %+v", step)
	}
	if step.DurationMS != 0 {
		t.Fatalf("cached step should not report subprocess duration: %+v", step)
	}
	if !strings.Contains(step.Command, "covered by go test") || !strings.Contains(step.Stdout, "full go test suite") {
		t.Fatalf("cached step did not explain evidence source: %+v", step)
	}
}

func fakeSelfVerifyStepDeps(t *testing.T) SelfVerifyStepDeps {
	t.Helper()
	ok := func(label string) StepResult {
		return StepResult{Label: label, OK: true}
	}
	return SelfVerifyStepDeps{
		HarnessRoot: func() string {
			return "/repo"
		},
		RunCommandStep: func(_ string, label string, _ time.Duration, _ string, _ string, _ ...string) StepResult {
			return ok(label)
		},
		ValidateHarnessInvariants: func(string) StepResult {
			return ok("harness invariants")
		},
		ValidateRiskQATier: func(string) RiskQAEvidence {
			return RiskQAEvidence{Step: ok("risk QA tier")}
		},
		ValidateInspect: func(string, string) StepResult {
			return ok("inspect smoke")
		},
		ValidateDocsIndex: func(string, string) StepResult {
			return ok("docs index smoke")
		},
		ValidateSelfVerifyCandidate: func(string, string, int64) StepResult {
			return ok("candidate export")
		},
		ValidateStepBudgetBaseline: func(string, string, int64) StepResult {
			return ok("step budget baseline")
		},
		ValidateInstallDryRunSmoke: func(string, string, int64) StepResult {
			return ok("install dry-run smoke")
		},
		ValidateCommandPolicy: func(string, string) StepResult {
			return ok("command policy smoke")
		},
		ValidateCommandAudit: func(string, string, int64) StepResult {
			return ok("command audit smoke")
		},
		ValidateContractCheck: func(string, string) StepResult {
			return ok("contract check")
		},
		ValidateToolConformance: func(string, string) StepResult {
			return ok("tool contract conformance")
		},
		ValidateWorkerLifecycle: func(string, string, int64) StepResult {
			return ok("worker lifecycle smoke")
		},
		ValidateMCP: func(string, string) StepResult {
			return ok("MCP smoke")
		},
		ValidateStateRoundtrip: func(string, string, int64) StepResult {
			return ok("state roundtrip")
		},
		ValidateParallelTempIsolation: func(string, string, int64) StepResult {
			return ok("parallel isolation")
		},
		ValidateDaemonRestartResilience: func(string, string, int64) StepResult {
			return ok("daemon resilience")
		},
		ValidatePreflightFuzz: func(string, string, int64) StepResult {
			return ok("preflight fuzz")
		},
		ValidateWebFetchBattery: func(string, string, int64) StepResult {
			return ok("web fetch battery")
		},
		ValidateNativeIntegration: func(string) StepResult {
			return ok("native integration")
		},
		ValidateRedactionAudit: func(string) StepResult {
			return ok("redaction audit")
		},
		ValidateQAGate: func(string) StepResult {
			return ok("QA gate")
		},
	}
}
