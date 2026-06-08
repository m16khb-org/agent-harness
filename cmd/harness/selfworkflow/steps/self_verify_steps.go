package steps

import (
	"time"

	"agent-harness/cmd/harness/commandstep"
)

type StepResult = commandstep.StepResult

type SelfVerifyPlannedStep struct {
	Label string
	Run   func() StepResult
}

type SelfVerifyStepDeps struct {
	HarnessRoot                     func() string
	RunCommandStep                  func(string, string, time.Duration, string, string, ...string) StepResult
	ValidateHarnessInvariants       func(string) StepResult
	ValidateRiskQATier              func(string) StepResult
	ValidateInspect                 func(string, string) StepResult
	ValidateDocsIndex               func(string, string) StepResult
	ValidateSelfVerifyCandidate     func(string, string, int64) StepResult
	ValidateStepBudgetBaseline      func(string, string, int64) StepResult
	ValidateInstallDryRunSmoke      func(string, string, int64) StepResult
	ValidateCommandPolicy           func(string, string) StepResult
	ValidateCommandAudit            func(string, string, int64) StepResult
	ValidateContractCheck           func(string, string) StepResult
	ValidateWorkerLifecycle         func(string, string, int64) StepResult
	ValidateMCP                     func(string, string) StepResult
	ValidateStateRoundtrip          func(string, string, int64) StepResult
	ValidateParallelTempIsolation   func(string, string, int64) StepResult
	ValidateDaemonRestartResilience func(string, string, int64) StepResult
	ValidatePreflightFuzz           func(string, string, int64) StepResult
	ValidateNativeIntegration       func(string) StepResult
	ValidateRedactionAudit          func(string) StepResult
	ValidateQAGate                  func(string) StepResult
}

func PlannedSelfVerifySteps(root string, tempBin string, seed int64, goTestStep *StepResult, deps SelfVerifyStepDeps) []SelfVerifyPlannedStep {
	return []SelfVerifyPlannedStep{
		{Label: "harness invariants", Run: func() StepResult { return deps.ValidateHarnessInvariants(root) }},
		{Label: "go test", Run: func() StepResult {
			*goTestStep = deps.RunCommandStep(root, "go test", 120*time.Second, "", "go", "test", "./...", "-count=1")
			return *goTestStep
		}},
		{Label: "contract golden tests", Run: func() StepResult {
			return CachedContractGoldenStep(*goTestStep, deps)
		}},
		{Label: "risk QA tier", Run: func() StepResult { return deps.ValidateRiskQATier(root) }},
		{Label: "go build", Run: func() StepResult {
			return deps.RunCommandStep(root, "go build", 120*time.Second, "", "go", "build", "-o", tempBin, "./cmd/harness")
		}},
		{Label: "binary drift", Run: func() StepResult {
			// After a successful build, verify the checked bin/agent-harness is
			// not stale relative to the source tree. We use the just-built tempBin
			// to run doctor --json and inspect the binary_drift check. A fresh
			// build means this step cannot produce a false positive (tempBin IS
			// current by construction), but it still exercises the drift detection
			// path as part of the self-verify QA surface.
			return deps.RunCommandStep(root, "binary drift", 10*time.Second, "", tempBin, "doctor", "--json", "--repo", root)
		}},
		{Label: "inspect smoke", Run: func() StepResult { return deps.ValidateInspect(tempBin, root) }},
		{Label: "docs index smoke", Run: func() StepResult { return deps.ValidateDocsIndex(tempBin, root) }},
		{Label: "candidate export", Run: func() StepResult { return deps.ValidateSelfVerifyCandidate(tempBin, root, seed) }},
		{Label: "step budget baseline", Run: func() StepResult { return deps.ValidateStepBudgetBaseline(tempBin, root, seed) }},
		{Label: "install dry-run smoke", Run: func() StepResult { return deps.ValidateInstallDryRunSmoke(tempBin, root, seed) }},
		{Label: "command policy smoke", Run: func() StepResult { return deps.ValidateCommandPolicy(tempBin, root) }},
		{Label: "command audit smoke", Run: func() StepResult { return deps.ValidateCommandAudit(tempBin, root, seed) }},
		{Label: "contract check", Run: func() StepResult { return deps.ValidateContractCheck(tempBin, root) }},
		{Label: "worker lifecycle smoke", Run: func() StepResult { return deps.ValidateWorkerLifecycle(tempBin, root, seed) }},
		{Label: "MCP smoke", Run: func() StepResult { return deps.ValidateMCP(tempBin, root) }},
		{Label: "state roundtrip", Run: func() StepResult { return deps.ValidateStateRoundtrip(tempBin, root, seed) }},
		{Label: "parallel isolation", Run: func() StepResult { return deps.ValidateParallelTempIsolation(tempBin, root, seed) }},
		{Label: "daemon resilience", Run: func() StepResult { return deps.ValidateDaemonRestartResilience(tempBin, root, seed) }},
		{Label: "preflight fuzz", Run: func() StepResult { return deps.ValidatePreflightFuzz(tempBin, root, seed) }},
		{Label: "native integration", Run: func() StepResult { return deps.ValidateNativeIntegration(root) }},
		{Label: "redaction audit", Run: func() StepResult { return deps.ValidateRedactionAudit(root) }},
		{Label: "QA gate", Run: func() StepResult { return deps.ValidateQAGate(root) }},
	}
}

func CachedContractGoldenStep(goTestStep StepResult, deps SelfVerifyStepDeps) StepResult {
	if goTestStep.OK {
		return StepResult{
			Label:      "contract golden tests",
			Command:    "covered by go test ./... -count=1",
			OK:         true,
			DurationMS: 0,
			Stdout:     "contract golden tests already executed by full go test suite",
		}
	}
	return deps.RunCommandStep(deps.HarnessRoot(), "contract golden tests", 120*time.Second, "", "go", "test", "./cmd/harness", "-run", "Golden", "-count=1")
}
