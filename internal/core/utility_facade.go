package core

import (
	"agent-harness/internal/core/audit"
	"agent-harness/internal/core/commitsuggest"
	"agent-harness/internal/core/contextregion"
	coredocs "agent-harness/internal/core/docs"
	"agent-harness/internal/core/doctor"
	"agent-harness/internal/core/externalllm"
	coreguard "agent-harness/internal/core/guard"
	"agent-harness/internal/core/hookfailure"
	"agent-harness/internal/core/hookmetrics"
	coreinspect "agent-harness/internal/core/inspect"
	coreinstall "agent-harness/internal/core/install"
	"agent-harness/internal/core/lintdiagnose"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/prompt"
	"agent-harness/internal/port"
	"time"
)

type CommandAuditRecord = audit.CommandAuditRecord

func AuditCommandPolicy(req policy.CommandPolicyRequest) (CommandAuditRecord, error) {
	return audit.AuditCommandPolicy(req)
}

type CommitSuggestRequest = commitsuggest.CommitSuggestRequest
type CommitSuggestResult = commitsuggest.CommitSuggestResult

func SuggestCommit(req CommitSuggestRequest) (CommitSuggestResult, error) {
	return commitsuggest.SuggestCommit(req)
}

const (
	RegionImmutablePrefix = contextregion.RegionImmutablePrefix
	RegionAppendOnlyLog   = contextregion.RegionAppendOnlyLog
	RegionVolatileScratch = contextregion.RegionVolatileScratch
)

var VolatileContextFields = contextregion.VolatileContextFields

func StableProjection(value any) any {
	return contextregion.StableProjection(value)
}

func StableProjectionJSON(value any) (string, error) {
	return contextregion.StableProjectionJSON(value)
}

func ContextSerializationStable(build func() any) (bool, string, error) {
	return contextregion.ContextSerializationStable(build)
}

type DocsIndexResult = coredocs.DocsIndexResult
type DocIndexInfo = coredocs.DocIndexInfo

func ListDocs(root string) []string {
	return coredocs.ListDocs(root)
}

func DocsIndex(root, version string) DocsIndexResult {
	return coredocs.DocsIndex(root, version)
}

type HarnessDoctorRequest = doctor.HarnessDoctorRequest
type HarnessDoctorResult = doctor.HarnessDoctorResult
type HarnessDoctorCheck = doctor.HarnessDoctorCheck
type HarnessDoctorIssue = doctor.HarnessDoctorIssue
type HarnessDoctorFix = doctor.HarnessDoctorFix

func HarnessDoctor(req HarnessDoctorRequest) (HarnessDoctorResult, error) {
	return doctor.HarnessDoctor(req)
}

type ExternalLLMPrintRequest = externalllm.ExternalLLMPrintRequest
type ExternalLLMPrintResult = externalllm.ExternalLLMPrintResult

func RunExternalLLMPrint(req ExternalLLMPrintRequest) (ExternalLLMPrintResult, error) {
	return externalllm.RunExternalLLMPrint(req)
}

func ExternalLLMPrintCommandPreview() string {
	return externalllm.ExternalLLMPrintCommandPreview()
}

func BuildExternalLLMJSONSchemaSection(example string, fieldTypes []string) prompt.PromptDataSection {
	return externalllm.BuildExternalLLMJSONSchemaSection(example, fieldTypes)
}

func DecodeExternalLLMStructuredJSONObject(label string, out []byte, target any) error {
	return externalllm.DecodeExternalLLMStructuredJSONObject(label, out, target)
}

type GuardCheckRequest = coreguard.GuardCheckRequest
type GuardCheckResult = coreguard.GuardCheckResult
type GuardFinding = coreguard.GuardFinding
type GuardSummary = coreguard.GuardSummary
type GuardBlockedError = coreguard.GuardBlockedError

func GuardCheck(req GuardCheckRequest) GuardCheckResult {
	return coreguard.GuardCheck(req)
}

func IsGuardBlocked(err error) bool {
	return coreguard.IsGuardBlocked(err)
}

type HookFailureEvent = hookfailure.HookFailureEvent
type HookFailureRecordResult = hookfailure.HookFailureRecordResult
type HookFailureListResult = hookfailure.HookFailureListResult
type HookFailurePruneResult = hookfailure.HookFailurePruneResult
type HookFailureStats = hookfailure.HookFailureStats
type HookMetricEvent = hookmetrics.HookMetricEvent
type HookMetricsStats = hookmetrics.HookMetricsStats
type HookMetricsPruneResult = hookmetrics.HookMetricsPruneResult

func RecordHookFailureEvent(event HookFailureEvent) (HookFailureRecordResult, error) {
	return hookfailure.RecordHookFailureEvent(event)
}

func ListHookFailureEvents(limit int) (HookFailureListResult, error) {
	return hookfailure.ListHookFailureEvents(limit)
}

func PruneHookFailureLog(maxAge time.Duration) (HookFailurePruneResult, error) {
	return hookfailure.PruneHookFailureLog(maxAge)
}

func HookFailureLogPath() string {
	return hookfailure.HookFailureLogPath()
}

func SummarizeHookFailureLog() (HookFailureStats, error) {
	return hookfailure.SummarizeHookFailureLog()
}

// HookFailureRateStats enriches the failure summary with failures/invocations
// (A2/G5) by joining the failure log against the hook-metrics invocation
// counter (both keyed by hook name). FailureRate is reported only for hooks
// with a known invocation count (>0): the failure-only "unparseable" bucket has
// no invocations and is omitted from the rate map so its 0 is not misread as
// "healthy" — its raw count stays visible via the embedded ByHook.
type HookFailureRateStats struct {
	HookFailureStats
	Invocations        map[string]int     `json:"invocations,omitempty"`
	FailureRate        map[string]float64 `json:"failure_rate,omitempty"`
	FailureRateOverall float64            `json:"failure_rate_overall"`
}

// SummarizeHookFailureStats joins the failure log with the metrics invocation
// counts to compute failure_rate = failures/invocations per hook and overall
// (A2/G5). Read-time only, no new write path. A metric-log error degrades
// gracefully to an empty invocation set rather than failing the whole summary.
func SummarizeHookFailureStats() (HookFailureRateStats, error) {
	fstats, err := hookfailure.SummarizeHookFailureLog()
	out := HookFailureRateStats{
		HookFailureStats: fstats,
		Invocations:      map[string]int{},
		FailureRate:      map[string]float64{},
	}
	if err != nil {
		out.OK = false
		return out, err
	}
	mstats, mErr := hookmetrics.SummarizeHookMetricsLog()
	if mErr == nil {
		for hook, lat := range mstats.ByHook {
			out.Invocations[hook] = lat.Count
		}
	}
	for hook, failures := range fstats.ByHook {
		if inv := out.Invocations[hook]; inv > 0 {
			out.FailureRate[hook] = hookmetrics.Rate(failures, inv)
		}
	}
	out.FailureRateOverall = hookmetrics.Rate(fstats.Total, mstats.Total)
	return out, nil
}

func RecordHookMetricEvent(event HookMetricEvent) error {
	_, err := hookmetrics.RecordHookMetricEvent(event)
	return err
}

func SummarizeHookMetricsLog() (HookMetricsStats, error) {
	return hookmetrics.SummarizeHookMetricsLog()
}

func PruneHookMetricsLog(maxAge time.Duration) (HookMetricsPruneResult, error) {
	return hookmetrics.PruneHookMetricsLog(maxAge)
}

type InspectInfo = coreinspect.InspectInfo
type SkillInfo = coreinspect.SkillInfo
type IntegrationStatus = coreinspect.IntegrationStatus

func InspectHarness(root, target, home, version, skillName string) InspectInfo {
	return coreinspect.InspectHarness(root, target, home, version, skillName)
}

func ListSkills(root, skillName string) []SkillInfo {
	return coreinspect.ListSkills(root, skillName)
}

func CodexMCPConfigured(path string) bool {
	return coreinspect.CodexMCPConfigured(path)
}

func DefaultNativeInstallRequest(root, home, codexHome, reasonixHome, binPath string) port.NativeInstallRequest {
	return coreinstall.DefaultNativeInstallRequest(root, home, codexHome, reasonixHome, binPath)
}

func InstallNative(req port.NativeInstallRequest, installers ...port.HostInstaller) (port.NativeInstallResult, error) {
	return coreinstall.InstallNative(req, installers...)
}

func ListSkillNames(root string) ([]string, error) {
	return coreinstall.ListSkillNames(root)
}

type LintDiagnoseRequest = lintdiagnose.LintDiagnoseRequest
type LintDiagnoseResult = lintdiagnose.LintDiagnoseResult

func DiagnoseCommand(req LintDiagnoseRequest) (LintDiagnoseResult, error) {
	return lintdiagnose.DiagnoseCommand(req)
}

type PromptDataSection = prompt.PromptDataSection
type StructuredPromptSpec = prompt.StructuredPromptSpec

var StructuredPromptSectionHeadings = prompt.StructuredPromptSectionHeadings

func BuildStructuredPrompt(spec StructuredPromptSpec) string {
	return prompt.BuildStructuredPrompt(spec)
}
