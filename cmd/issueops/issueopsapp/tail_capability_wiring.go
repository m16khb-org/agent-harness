package issueopsapp

import (
	issueopsclitaildeps "issueops/cmd/issueops/issueopscli"
	remotecmdtaildeps "issueops/cmd/issueops/issueopscli/remotecmd"
	mcpclitaildeps "issueops/cmd/issueops/mcpcli"
	policyclitaildeps "issueops/cmd/issueops/policycli"
	stateiotaildeps "issueops/cmd/issueops/selfworkflow/stateio"
	summarytaildeps "issueops/cmd/issueops/selfworkflow/summary"
	webfetchtaildeps "issueops/cmd/issueops/validationcli/webfetch"
	webfetchclitaildeps "issueops/cmd/issueops/webfetchcli"
	auditadapter "issueops/internal/adapter/audit"
	failurecauseadapter "issueops/internal/adapter/failurecause"
	webfetchadapter "issueops/internal/adapter/outbound/webfetch"
	provideradapter "issueops/internal/adapter/provider"
	toolconformancetaildeps "issueops/internal/adapter/toolconformance"
	tracetaildeps "issueops/internal/adapter/trace"
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
