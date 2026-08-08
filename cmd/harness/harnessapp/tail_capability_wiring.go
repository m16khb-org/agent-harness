package harnessapp

import (
	issueopsclitaildeps "agent-harness/cmd/harness/issueopscli"
	remotecmdtaildeps "agent-harness/cmd/harness/issueopscli/remotecmd"
	mcpclitaildeps "agent-harness/cmd/harness/mcpcli"
	policyclitaildeps "agent-harness/cmd/harness/policycli"
	stateiotaildeps "agent-harness/cmd/harness/selfworkflow/stateio"
	summarytaildeps "agent-harness/cmd/harness/selfworkflow/summary"
	webfetchtaildeps "agent-harness/cmd/harness/validationcli/webfetch"
	webfetchclitaildeps "agent-harness/cmd/harness/webfetchcli"
	auditadapter "agent-harness/internal/adapter/audit"
	failurecauseadapter "agent-harness/internal/adapter/failurecause"
	webfetchadapter "agent-harness/internal/adapter/outbound/webfetch"
	provideradapter "agent-harness/internal/adapter/provider"
	toolconformancetaildeps "agent-harness/internal/adapter/toolconformance"
	tracetaildeps "agent-harness/internal/adapter/trace"
)

// configureTailCapabilities는 실패 원인 분류, 정책 감사, 웹 조회, provider 해석을
// 설치한다. 모두 파일·네트워크·프로세스에 닿는 연산이다.
func configureTailCapabilities() {
	issueopsclitaildeps.Resolve = provideradapter.Resolve
	mcpclitaildeps.AuditCommandPolicy = auditadapter.AuditCommandPolicy
	mcpclitaildeps.Fetch = webfetchadapter.Fetch
	policyclitaildeps.AuditCommandPolicy = auditadapter.AuditCommandPolicy
	remotecmdtaildeps.Resolve = provideradapter.Resolve
	stateiotaildeps.Classify = failurecauseadapter.Classify
	summarytaildeps.Classify = failurecauseadapter.Classify
	toolconformancetaildeps.ClassifyFailureCause = failurecauseadapter.Classify
	tracetaildeps.Classify = failurecauseadapter.Classify
	webfetchclitaildeps.DeterministicFixtures = webfetchadapter.DeterministicFixtures
	webfetchclitaildeps.Fetch = webfetchadapter.Fetch
	webfetchclitaildeps.RunBenchmark = webfetchadapter.RunBenchmark
	webfetchtaildeps.DeterministicFixtures = webfetchadapter.DeterministicFixtures
	webfetchtaildeps.RunBenchmark = webfetchadapter.RunBenchmark
}
