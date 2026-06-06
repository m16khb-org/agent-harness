package draftwiki

import (
	"agent-harness/internal/core/docs"
	"agent-harness/internal/core/externalllm"
	"agent-harness/internal/core/lifecycle"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/projectdoc"
	"agent-harness/internal/core/prompt"
	"agent-harness/internal/core/state"
	coreworker "agent-harness/internal/core/worker"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir

const (
	WorkerStatusQueued    = coreworker.WorkerStatusQueued
	WorkerStatusRunning   = coreworker.WorkerStatusRunning
	WorkerStatusSucceeded = coreworker.WorkerStatusSucceeded
	WorkerStatusFailed    = coreworker.WorkerStatusFailed
)

const draftWikiQueueFile = QueueFile
const draftWikiQueueLockFile = QueueLockFile
const maxDraftWikiQueueEvents = MaxQueueEvents

type ProjectDocsPlannedFile = projectdoc.ProjectDocsPlannedFile
type ProjectLifecycleStatePlan = lifecycle.ProjectLifecycleStatePlan
type ExternalLLMPrintRequest = externalllm.ExternalLLMPrintRequest
type ExternalLLMPrintResult = externalllm.ExternalLLMPrintResult
type StructuredPromptSpec = prompt.StructuredPromptSpec
type PromptDataSection = prompt.PromptDataSection

func draftWikiQueuePath(repoRoot string, ensure bool) (ProjectLifecycleStatePlan, string, error) {
	plan, err := lifecycle.ValidateProjectLifecycleState(repoRoot)
	if err != nil {
		return plan, "", err
	}
	if ensure && !plan.Exists {
		plan, err = lifecycle.InitProjectLifecycleState(repoRoot, true)
		if err != nil {
			return plan, "", err
		}
	}
	if ensure && !plan.NamespaceValid {
		return plan, "", fmt.Errorf("project lifecycle namespace mismatch for %s", plan.RepoRoot)
	}
	return plan, filepath.Join(plan.ProjectStateDir, draftWikiQueueFile), nil
}

func trimDraftWikiQueueMaterial(material string) string {
	return TrimQueueMaterial(material)
}

func draftWikiQueueEventID(repoID, material, at string) string {
	return QueueEventID(repoID, material, at)
}

func acquireDraftWikiQueueLock(projectStateDir string) (func(), bool, error) {
	return AcquireQueueLock(projectStateDir)
}

func readDraftWikiQueueEvents(path string) ([]DraftWikiQueueEvent, []string, error) {
	return ReadQueueEvents(path)
}

func appendDraftWikiQueueEvent(path string, event DraftWikiQueueEvent) error {
	return AppendQueueEvent(path, event)
}

func capDraftWikiQueueEvents(path string, keep int) error {
	return CapQueueEvents(path, keep)
}

func pruneDraftWikiQueuePath(path string, keep int) (DraftWikiQueuePruneResult, error) {
	return PruneQueuePath(path, keep)
}

var rewriteDraftWikiQueueEventsFunc = func(path string, events []DraftWikiQueueEvent) error {
	return RewriteQueueEvents(path, events)
}

func SetRewriteDraftWikiQueueEventsFunc(fn func(string, []DraftWikiQueueEvent) error) func(string, []DraftWikiQueueEvent) error {
	previous := rewriteDraftWikiQueueEventsFunc
	if fn == nil {
		rewriteDraftWikiQueueEventsFunc = RewriteQueueEvents
	} else {
		rewriteDraftWikiQueueEventsFunc = fn
	}
	return previous
}

func redactFreeform(s string) string {
	return policy.RedactFreeform(s)
}

func redactStringSlice(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, redactFreeform(item))
	}
	return out
}

func plannedFileAction(path, content string) string {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "create"
	}
	if err != nil {
		return "update"
	}
	if string(b) == content {
		return "unchanged"
	}
	return "update"
}

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func RunExternalLLMPrint(req ExternalLLMPrintRequest) (ExternalLLMPrintResult, error) {
	return externalllm.RunExternalLLMPrint(req)
}

func BuildStructuredPrompt(spec StructuredPromptSpec) string {
	return prompt.BuildStructuredPrompt(spec)
}

func BuildExternalLLMJSONSchemaSection(example string, fields []string) PromptDataSection {
	return externalllm.BuildExternalLLMJSONSchemaSection(example, fields)
}

func DecodeExternalLLMStructuredJSONObject(label string, out []byte, target any) error {
	return externalllm.DecodeExternalLLMStructuredJSONObject(label, out, target)
}

func ExternalLLMPrintCommandPreview(command string) string {
	return externalllm.ExternalLLMPrintCommandPreview(command)
}

func StateDir() string {
	return state.StateDir()
}

func readDocHeadings(path string) (string, []string) {
	return docs.ReadHeadings(path)
}
