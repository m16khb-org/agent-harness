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

func buildCommitSuggestPrompt(diff string) string {
	return commitsuggest.BuildPrompt(diff)
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

func readDocHeadings(path string) (string, []string) {
	return coredocs.ReadHeadings(path)
}

type HarnessDoctorRequest = doctor.HarnessDoctorRequest
type HarnessDoctorResult = doctor.HarnessDoctorResult
type HarnessDoctorCheck = doctor.HarnessDoctorCheck
type HarnessDoctorIssue = doctor.HarnessDoctorIssue
type HarnessDoctorFix = doctor.HarnessDoctorFix

func HarnessDoctor(req HarnessDoctorRequest) (HarnessDoctorResult, error) {
	return doctor.HarnessDoctor(req)
}

func shellQuote(s string) string {
	return doctor.ShellQuote(s)
}

type ExternalLLMPrintRequest = externalllm.ExternalLLMPrintRequest
type ExternalLLMPrintResult = externalllm.ExternalLLMPrintResult

func RunExternalLLMPrint(req ExternalLLMPrintRequest) (ExternalLLMPrintResult, error) {
	return externalllm.RunExternalLLMPrint(req)
}

func ExternalLLMPrintCommandPreview(command string) string {
	return externalllm.ExternalLLMPrintCommandPreview(command)
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

func exists(path string) bool {
	return coreinspect.Exists(path)
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

func buildLintDiagnosePrompt(exitCode int, logTail string) string {
	return lintdiagnose.BuildPrompt(exitCode, logTail)
}

type PromptDataSection = prompt.PromptDataSection
type StructuredPromptSpec = prompt.StructuredPromptSpec

var StructuredPromptSectionHeadings = prompt.StructuredPromptSectionHeadings

func BuildStructuredPrompt(spec StructuredPromptSpec) string {
	return prompt.BuildStructuredPrompt(spec)
}
