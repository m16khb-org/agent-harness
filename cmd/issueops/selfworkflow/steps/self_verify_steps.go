package steps

import (
	"time"

	"issueops/cmd/issueops/commandstep"
)

type StepResult = commandstep.StepResult

type SelfVerifyPlannedStep struct {
	Label string
	Run   func() StepResult
}

type RiskQAEvidence struct {
	Step             StepResult
	CoversFullGoTest bool
}

const selfVerifyGoTestTimeout = 10 * time.Minute

type SelfVerifyStepDeps struct {
	IssueOpsRoot                    func() string
	RunCommandStep                  func(string, string, time.Duration, string, string, ...string) StepResult
	ValidateHarnessInvariants       func(string) StepResult
	ValidateGoFormat                func(string) StepResult
	ValidateRiskQATier              func(string) RiskQAEvidence
	ValidateInspect                 func(string, string) StepResult
	ValidateDocsIndex               func(string, string) StepResult
	ValidateSelfVerifyCandidate     func(string, string, int64) StepResult
	ValidateStepBudgetBaseline      func(string, string, int64) StepResult
	ValidateInstallDryRunSmoke      func(string, string, int64) StepResult
	ValidateCommandPolicy           func(string, string) StepResult
	ValidateCommandAudit            func(string, string, int64) StepResult
	ValidateContractCheck           func(string, string) StepResult
	ValidateToolConformance         func(string, string) StepResult
	ValidateWorkerLifecycle         func(string, string, int64) StepResult
	ValidateMCP                     func(string, string) StepResult
	ValidateStateRoundtrip          func(string, string, int64) StepResult
	ValidateParallelTempIsolation   func(string, string, int64) StepResult
	ValidateDaemonRestartResilience func(string, string, int64) StepResult
	ValidatePreflightFuzz           func(string, string, int64) StepResult
	ValidateWebFetchBattery         func(string, string, int64) StepResult
	ValidateNativeIntegration       func(string) StepResult
	ValidateRedactionAudit          func(string) StepResult
	ValidateQAGate                  func(string) StepResult
}

func PlannedSelfVerifySteps(root string, tempBin string, seed int64, goTestStep *StepResult, deps SelfVerifyStepDeps) []SelfVerifyPlannedStep {
	var riskQAEvidence RiskQAEvidence
	return []SelfVerifyPlannedStep{
		{Label: "harness invariants", Run: func() StepResult { return deps.ValidateHarnessInvariants(root) }},
		// CI의 Format check와 같은 게이트를 로컬에서도 무조건 실행한다. gofmt는
		// 값싸고 결정적이므로 긴 go test보다 앞에 두어 fail-fast 모드에서 먼저 드러낸다.
		{Label: "gofmt", Run: func() StepResult { return deps.ValidateGoFormat(root) }},
		{Label: "risk QA tier", Run: func() StepResult {
			riskQAEvidence = deps.ValidateRiskQATier(root)
			return riskQAEvidence.Step
		}},
		{Label: "go test", Run: func() StepResult {
			if riskQAEvidence.CoversFullGoTest && riskQAEvidence.Step.OK {
				*goTestStep = StepResult{
					Label:   "go test",
					Command: riskQAEvidence.Step.Command,
					OK:      true,
					Stdout:  "reused successful full-suite coverage from risk QA race test",
				}
				return *goTestStep
			}
			*goTestStep = deps.RunCommandStep(root, "go test", selfVerifyGoTestTimeout, "", "go", "test", "./...", "-count=1")
			return *goTestStep
		}},
		{Label: "contract golden tests", Run: func() StepResult {
			return CachedContractGoldenStep(*goTestStep, deps)
		}},
		{Label: "go build", Run: func() StepResult {
			return deps.RunCommandStep(root, "go build", 120*time.Second, "", "go", "build", "-o", tempBin, "./cmd/issueops")
		}},
		{Label: "binary drift", Run: func() StepResult {
			// 빌드가 성공한 뒤, 커밋된 bin/issueops가 소스 트리 대비 stale하지
			// 않은지 확인한다. 방금 빌드한 tempBin으로 doctor --json을 실행해
			// binary_drift 체크를 살핀다. 갓 빌드한 tempBin은 구조상 항상 최신이므로
			// 이 단계가 false positive를 낼 수는 없지만, self-verify QA 표면의 일부로
			// drift 탐지 경로를 여전히 검증한다.
			return deps.RunCommandStep(root, "binary drift", 10*time.Second, "", tempBin, "doctor", "--static-only", "--json", "--repo", root)
		}},
		{Label: "inspect smoke", Run: func() StepResult { return deps.ValidateInspect(tempBin, root) }},
		{Label: "docs index smoke", Run: func() StepResult { return deps.ValidateDocsIndex(tempBin, root) }},
		{Label: "candidate export", Run: func() StepResult { return deps.ValidateSelfVerifyCandidate(tempBin, root, seed) }},
		{Label: "step budget baseline", Run: func() StepResult { return deps.ValidateStepBudgetBaseline(tempBin, root, seed) }},
		{Label: "install dry-run smoke", Run: func() StepResult { return deps.ValidateInstallDryRunSmoke(tempBin, root, seed) }},
		{Label: "command policy smoke", Run: func() StepResult { return deps.ValidateCommandPolicy(tempBin, root) }},
		{Label: "command audit smoke", Run: func() StepResult { return deps.ValidateCommandAudit(tempBin, root, seed) }},
		{Label: "contract check", Run: func() StepResult { return deps.ValidateContractCheck(tempBin, root) }},
		{Label: "tool contract conformance", Run: func() StepResult { return deps.ValidateToolConformance(tempBin, root) }},
		{Label: "worker lifecycle smoke", Run: func() StepResult { return deps.ValidateWorkerLifecycle(tempBin, root, seed) }},
		{Label: "MCP smoke", Run: func() StepResult { return deps.ValidateMCP(tempBin, root) }},
		{Label: "state roundtrip", Run: func() StepResult { return deps.ValidateStateRoundtrip(tempBin, root, seed) }},
		{Label: "parallel isolation", Run: func() StepResult { return deps.ValidateParallelTempIsolation(tempBin, root, seed) }},
		{Label: "daemon resilience", Run: func() StepResult { return deps.ValidateDaemonRestartResilience(tempBin, root, seed) }},
		{Label: "preflight fuzz", Run: func() StepResult { return deps.ValidatePreflightFuzz(tempBin, root, seed) }},
		{Label: "web fetch battery", Run: func() StepResult { return deps.ValidateWebFetchBattery(tempBin, root, seed) }},
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
	return deps.RunCommandStep(deps.IssueOpsRoot(), "contract golden tests", 120*time.Second, "", "go", "test", "./cmd/issueops", "-run", "Golden", "-count=1")
}
