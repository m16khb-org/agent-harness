package classification

import "strings"

func ProposedKnobForStep(step string) string {
	step = strings.ToLower(step)
	switch {
	case strings.Contains(step, "contract") || strings.Contains(step, "golden"):
		return "tighten CLI/MCP contract golden coverage and update schema intentionally"
	case strings.Contains(step, "redaction") || strings.Contains(step, "secret"):
		return "extend redaction audit fixtures before adding any new logging surface"
	case strings.Contains(step, "daemon"):
		return "add daemon stale-lock/socket resilience fixture before changing runtime behavior"
	case strings.Contains(step, "policy") || strings.Contains(step, "guard"):
		return "add deterministic policy or guard fixture for the repeated failure"
	case strings.Contains(step, "go test") || strings.Contains(step, "test"):
		return "reduce failing test to a fixture-backed regression before broad harness changes"
	case strings.Contains(step, "build"):
		return "keep build failure fix in core/CLI code path and verify release build"
	default:
		return "classify the repeated trace pattern, then add the smallest harness guardrail with a fixture"
	}
}

func OverfitRiskForClass(class string) string {
	class = strings.ToLower(class)
	switch class {
	case "deterministic":
		return "medium: deterministic failures justify a guardrail, but keep it fixture-backed"
	case "intermittent":
		return "high: rerun and isolate flake signals before changing harness prompts or hooks"
	case "single_failure_observation":
		return "high: one fail-fast observation is insufficient for broad harness tuning"
	default:
		return "medium: verify on a synthetic trace before changing shared harness behavior"
	}
}

func DefaultVerificationCommand(step string) string {
	step = strings.ToLower(step)
	switch {
	case strings.Contains(step, "contract") || strings.Contains(step, "golden"):
		return "go test ./cmd/harness -run Golden -count=1"
	case strings.Contains(step, "policy") || strings.Contains(step, "guard"):
		return "go test ./internal/core -count=1"
	case strings.Contains(step, "build"):
		return "go build -o bin/agent-harness ./cmd/harness"
	default:
		return "go test ./... -count=1"
	}
}
