package draftwiki

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"agent-harness/internal/core/docs"
	"agent-harness/internal/core/draftwiki/queue"
	"agent-harness/internal/core/judgement"
	"agent-harness/internal/core/lifecycle"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/projectdoc"
	"agent-harness/internal/core/prompt"
	"agent-harness/internal/adapter/outbound/state"
	coreworker "agent-harness/internal/core/worker"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir

const QueueFile = queue.File
const QueueLockFile = queue.LockFile
const MaxQueueEvents = queue.MaxEvents

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
type StructuredPromptSpec = prompt.StructuredPromptSpec
type PromptDataSection = prompt.PromptDataSection
type DraftWikiQueueAppendRequest = queue.AppendRequest
type DraftWikiQueueEvent = queue.Event
type DraftWikiQueueAppendResult = queue.AppendResult
type DraftWikiQueueProcessRequest = queue.ProcessRequest
type DraftWikiQueueProcessResult = queue.ProcessResult
type DraftWikiQueuePruneResult = queue.PruneResult
type DraftWikiQueuePruneAllResult = queue.PruneAllResult

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
	return queue.TrimMaterial(material)
}

func TrimQueueMaterial(material string) string {
	return queue.TrimMaterial(material)
}

func draftWikiQueueEventID(repoID, material, at string) string {
	return queue.EventID(repoID, material, at)
}

func QueueEventID(repoID, material, at string) string {
	return queue.EventID(repoID, material, at)
}

func acquireDraftWikiQueueLock(projectStateDir string) (func(), bool, error) {
	return queue.AcquireLock(projectStateDir)
}

func AcquireQueueLock(projectStateDir string) (func(), bool, error) {
	return queue.AcquireLock(projectStateDir)
}

func CountQueueLines(path string, limit int) (int, error) {
	return queue.CountLines(path, limit)
}

func readDraftWikiQueueEvents(path string) ([]DraftWikiQueueEvent, []string, error) {
	return queue.ReadEvents(path)
}

func ReadQueueEvents(path string) ([]DraftWikiQueueEvent, []string, error) {
	return queue.ReadEvents(path)
}

func FormatQueueMalformedWarning(lineNumber int, line string) string {
	return queue.FormatMalformedWarning(lineNumber, line)
}

func appendDraftWikiQueueEvent(path string, event DraftWikiQueueEvent) error {
	return queue.AppendEvent(path, event)
}

func AppendQueueEvent(path string, event DraftWikiQueueEvent) error {
	return queue.AppendEvent(path, event)
}

func capDraftWikiQueueEvents(path string, keep int) error {
	return queue.CapEvents(path, keep)
}

func CapQueueEvents(path string, keep int) error {
	return queue.CapEvents(path, keep)
}

func pruneDraftWikiQueuePath(path string, keep int) (DraftWikiQueuePruneResult, error) {
	return queue.PrunePath(path, keep)
}

func PruneQueuePath(path string, keep int) (DraftWikiQueuePruneResult, error) {
	return queue.PrunePath(path, keep)
}

var rewriteDraftWikiQueueEventsFunc = func(path string, events []DraftWikiQueueEvent) error {
	return queue.RewriteEvents(path, events)
}

func RewriteQueueEvents(path string, events []DraftWikiQueueEvent) error {
	return queue.RewriteEvents(path, events)
}

func SetRewriteDraftWikiQueueEventsFunc(fn func(string, []DraftWikiQueueEvent) error) func(string, []DraftWikiQueueEvent) error {
	previous := rewriteDraftWikiQueueEventsFunc
	if fn == nil {
		rewriteDraftWikiQueueEventsFunc = queue.RewriteEvents
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

func BuildStructuredPrompt(spec StructuredPromptSpec) string {
	return prompt.BuildStructuredPrompt(spec)
}

func BuildHostJudgementJSONSchemaSection(example string, fields []string) PromptDataSection {
	return judgement.BuildJSONSchemaSection(example, fields)
}

func DecodeHostJudgementStructuredJSONObject(label string, out []byte, target any) error {
	return judgement.DecodeStructuredJSONObject(label, out, target)
}

func StateDir() string {
	return state.StateDir()
}

func readDocHeadings(path string) (string, []string) {
	return docs.ReadHeadings(path)
}
