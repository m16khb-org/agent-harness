package guard

import (
	guardcontract "agent-harness/internal/contract/guard"
	"strings"

	"agent-harness/internal/domain/guardpattern"
)

func guardFileFindings(rel, content string, existingSymbols map[string][]string) []guardcontract.GuardFinding {
	findings := []guardcontract.GuardFinding{}
	immutablePrefixBuilder := strings.Contains(content, pattern.ImmutablePrefixMarker)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lineNo := i + 1
		if immutablePrefixBuilder && pattern.ContextNonDeterminism.MatchString(line) && !strings.Contains(line, pattern.VolatileOKMarker) {
			findings = append(findings, guardcontract.GuardFinding{
				Severity: "warn",
				Rule:     "nondeterministic-context-serialization",
				File:     rel,
				Line:     lineNo,
				Message:  "Immutable-prefix context builder introduces a non-deterministic value; move it to a volatile region or annotate the line with volatile-ok.",
				Evidence: strings.TrimSpace(line),
			})
		}
		if isExecutableTestSourcePath(rel) {
			if pattern.AmbiguousTestName.MatchString(line) {
				findings = append(findings, guardcontract.GuardFinding{Severity: "warn", Rule: "ambiguous-test-name", File: rel, Line: lineNo, Message: "Test name is too generic to communicate the protected contract.", Evidence: strings.TrimSpace(line)})
			}
			if pattern.SleepInTest.MatchString(line) {
				findings = append(findings, guardcontract.GuardFinding{Severity: "block", Rule: "sleep-in-test", File: rel, Line: lineNo, Message: "Tests must not depend on wall-clock sleep; use deterministic synchronization or fake clocks.", Evidence: strings.TrimSpace(line)})
			}
			for _, url := range pattern.ExternalURL.FindAllString(line, -1) {
				if !guardAllowsFixtureURL(url) {
					findings = append(findings, guardcontract.GuardFinding{Severity: "block", Rule: "real-external-service-in-test", File: rel, Line: lineNo, Message: "Tests must not depend on real external services.", Evidence: url})
				}
			}
			if strings.Contains(strings.ToLower(line), "localhost") {
				findings = append(findings, guardcontract.GuardFinding{Severity: "warn", Rule: "localhost-in-test", File: rel, Line: lineNo, Message: "Local service dependencies in tests need explicit isolation and lifecycle control.", Evidence: strings.TrimSpace(line)})
			}
			if pattern.SnapshotAssertion.MatchString(line) {
				findings = append(findings, guardcontract.GuardFinding{Severity: "warn", Rule: "snapshot-test-review", File: rel, Line: lineNo, Message: "Snapshot/golden assertions should be paired with focused contract checks and intentional update notes.", Evidence: strings.TrimSpace(line)})
			}
		}
		if m := pattern.NewSymbol.FindStringSubmatch(line); len(m) == 2 {
			symbol := m[1]
			if reuseFinding, ok := guardReuseFinding(rel, lineNo, symbol, existingSymbols); ok {
				findings = append(findings, reuseFinding)
			}
		}
	}
	if isTestPath(rel) && len(content) > 200_000 {
		findings = append(findings, guardcontract.GuardFinding{Severity: "warn", Rule: "large-test-fixture", File: rel, Message: "Large test files or fixtures can hide weak assertions; prefer small named fixtures."})
	}
	return findings
}
