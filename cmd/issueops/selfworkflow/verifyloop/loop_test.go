package verifyloop

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"issueops/cmd/issueops/commandstep"
	"issueops/cmd/issueops/selfworkflow/progress"
	"issueops/cmd/issueops/selfworkflow/steps"
)

func TestSelfVerifyRunsAllStepsSuccessfully(t *testing.T) {
	var printed []string
	result, err := SelfVerify(Request{BaseSeed: 100, TargetScore: 0, Verbose: true}, Deps{
		IssueOpsRoot: func() string { return t.TempDir() },
		StepDeps:     fakeVerifyLoopStepDeps("", ""),
		PrintStep: func(step commandstep.StepResult) {
			printed = append(printed, step.Label)
		},
		Printf: func(format string, args ...any) (int, error) {
			return 0, nil
		},
	})
	if err != nil {
		t.Fatalf("SelfVerify returned error: %v", err)
	}
	if !result.OK || len(result.Runs) != 1 || len(result.Runs[0].Steps) == 0 {
		t.Fatalf("unexpected successful result: %#v", result)
	}
	if len(printed) != len(result.Runs[0].Steps) {
		t.Fatalf("verbose PrintStep calls=%d steps=%d", len(printed), len(result.Runs[0].Steps))
	}
}

func TestSelfVerifyStopsOnFailedStepAndEmitsProgress(t *testing.T) {
	var buf bytes.Buffer
	reporter, err := progress.NewSelfVerifyProgressReporter("jsonl", &buf)
	if err != nil {
		t.Fatalf("NewSelfVerifyProgressReporter: %v", err)
	}
	result, err := SelfVerify(Request{
		BaseSeed:    200,
		TargetScore: 95,
		Reporter:    reporter,
	}, Deps{
		IssueOpsRoot: func() string { return t.TempDir() },
		StepDeps:     fakeVerifyLoopStepDeps("docs index smoke", "docs failed"),
	})
	if err == nil || !errors.Is(err, ErrSelfVerificationGateFailed) {
		t.Fatalf("expected gate failure, got result=%#v err=%v", result, err)
	}
	if result.OK || len(result.Runs) != 1 {
		t.Fatalf("unexpected failed result: %#v", result)
	}
	events := decodeProgressEventsForLoopTest(t, buf.String())
	if len(events) == 0 || events[len(events)-1].Event != "loop_end" || events[len(events)-1].OK == nil || *events[len(events)-1].OK {
		t.Fatalf("expected failed loop_end event, got %#v", events)
	}
}

func decodeProgressEventsForLoopTest(t *testing.T, out string) []progress.SelfVerifyProgressEvent {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	events := make([]progress.SelfVerifyProgressEvent, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event progress.SelfVerifyProgressEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode progress event: %v\n%s", err, line)
		}
		events = append(events, event)
	}
	return events
}

func fakeVerifyLoopStepDeps(failLabel, failError string) steps.SelfVerifyStepDeps {
	return fakeVerifyLoopStepDepsOK(func(label string) commandstep.StepResult {
		if label == failLabel {
			return commandstep.StepResult{Label: label, OK: false, Error: failError}
		}
		return commandstep.StepResult{Label: label, OK: true}
	})
}

// fakeVerifyLoopStepDepsFailing fails every label in failLabels (the rest pass),
// for exercising collect-all-steps across multiple gates in one iteration.
func fakeVerifyLoopStepDepsFailing(failLabels ...string) steps.SelfVerifyStepDeps {
	fail := map[string]bool{}
	for _, label := range failLabels {
		fail[label] = true
	}
	return fakeVerifyLoopStepDepsOK(func(label string) commandstep.StepResult {
		if fail[label] {
			return commandstep.StepResult{Label: label, OK: false, Error: label + " failed"}
		}
		return commandstep.StepResult{Label: label, OK: true}
	})
}

func fakeVerifyLoopStepDepsOK(ok func(string) commandstep.StepResult) steps.SelfVerifyStepDeps {
	return steps.SelfVerifyStepDeps{
		IssueOpsRoot: func() string { return "." },
		RunCommandStep: func(_ string, label string, _ time.Duration, _ string, _ string, _ ...string) commandstep.StepResult {
			return ok(label)
		},
		ValidateHarnessInvariants: func(string) commandstep.StepResult { return ok("harness invariants") },
		ValidateGoFormat:          func(string) commandstep.StepResult { return ok("gofmt") },
		ValidateRiskQATier: func(string) steps.RiskQAEvidence {
			return steps.RiskQAEvidence{Step: ok("risk QA tier")}
		},
		ValidateInspect: func(string, string) commandstep.StepResult {
			return ok("inspect smoke")
		},
		ValidateDocsIndex: func(string, string) commandstep.StepResult {
			return ok("docs index smoke")
		},
		ValidateSelfVerifyCandidate: func(string, string, int64) commandstep.StepResult {
			return ok("candidate export")
		},
		ValidateStepBudgetBaseline: func(string, string, int64) commandstep.StepResult {
			return ok("step budget baseline")
		},
		ValidateInstallDryRunSmoke: func(string, string, int64) commandstep.StepResult {
			return ok("install dry-run smoke")
		},
		ValidateCommandPolicy: func(string, string) commandstep.StepResult {
			return ok("command policy smoke")
		},
		ValidateCommandAudit: func(string, string, int64) commandstep.StepResult {
			return ok("command audit smoke")
		},
		ValidateContractCheck: func(string, string) commandstep.StepResult {
			return ok("contract check")
		},
		ValidateToolConformance: func(string, string) commandstep.StepResult {
			return ok("tool contract conformance")
		},
		ValidateWorkerLifecycle: func(string, string, int64) commandstep.StepResult {
			return ok("worker lifecycle smoke")
		},
		ValidateMCP: func(string, string) commandstep.StepResult { return ok("MCP smoke") },
		ValidateStateRoundtrip: func(string, string, int64) commandstep.StepResult {
			return ok("state roundtrip")
		},
		ValidateParallelTempIsolation: func(string, string, int64) commandstep.StepResult {
			return ok("parallel isolation")
		},
		ValidateDaemonRestartResilience: func(string, string, int64) commandstep.StepResult {
			return ok("daemon resilience")
		},
		ValidatePreflightFuzz: func(string, string, int64) commandstep.StepResult {
			return ok("preflight fuzz")
		},
		ValidateWebFetchBattery: func(string, string, int64) commandstep.StepResult {
			return ok("web fetch battery")
		},
		ValidateNativeIntegration: func(string) commandstep.StepResult {
			return ok("native integration")
		},
		ValidateRedactionAudit: func(string) commandstep.StepResult {
			return ok("redaction audit")
		},
		ValidateQAGate: func(string) commandstep.StepResult { return ok("QA gate") },
	}
}

// B5: collect-all-steps mode surfaces EVERY failing gate in an iteration (for
// concurrent regression diagnosis) and still FAILS the gate; fail-fast (default)
// stops at the first failure. Neither weakens the gate.
func TestSelfVerifyCollectAllStepsSurfacesEveryFailure(t *testing.T) {
	// "harness invariants" is the first planned step; "docs index smoke" is a
	// later one. Both fail.
	stepDeps := fakeVerifyLoopStepDepsFailing("harness invariants", "docs index smoke")

	collect, err := SelfVerify(Request{
		BaseSeed: 100, TargetScore: 95, CollectAllSteps: true,
	}, Deps{
		IssueOpsRoot: func() string { return "." }, StepDeps: stepDeps,
	})
	if err == nil || !errors.Is(err, ErrSelfVerificationGateFailed) {
		t.Fatalf("collect-all must still fail the gate, got %v", err)
	}
	if collect.OK {
		t.Fatalf("collect-all gate must not be OK: %+v", collect)
	}
	collectFailed := map[string]bool{}
	for _, run := range collect.Runs {
		for _, s := range run.Steps {
			if !s.OK {
				collectFailed[s.Label] = true
			}
		}
	}
	if !collectFailed["harness invariants"] || !collectFailed["docs index smoke"] {
		t.Fatalf("collect-all must surface BOTH failures, got %v", collectFailed)
	}

	// Fail-fast (default): stops at the first failure; the later gate never runs.
	ff, err := SelfVerify(Request{BaseSeed: 100, TargetScore: 95}, Deps{
		IssueOpsRoot: func() string { return "." }, StepDeps: stepDeps,
	})
	if err == nil || !errors.Is(err, ErrSelfVerificationGateFailed) {
		t.Fatalf("fail-fast must fail the gate, got %v", err)
	}
	ffFailedFirst, ranLater := false, false
	for _, run := range ff.Runs {
		for _, s := range run.Steps {
			if s.Label == "harness invariants" && !s.OK {
				ffFailedFirst = true
			}
			if s.Label == "docs index smoke" {
				ranLater = true
			}
		}
	}
	if !ffFailedFirst {
		t.Fatal("fail-fast must surface the first failure (harness invariants)")
	}
	if ranLater {
		t.Fatal("fail-fast must NOT run steps after the first failure (docs index smoke ran)")
	}
}
