package draftwiki

import "agent-harness/internal/core/draftwiki/queue"

const QueueFile = queue.File

const QueueLockFile = queue.LockFile

const MaxQueueEvents = queue.MaxEvents

type DraftWikiQueueAppendRequest = queue.AppendRequest

type DraftWikiQueueEvent = queue.Event

type DraftWikiQueueAppendResult = queue.AppendResult

type DraftWikiQueueProcessRequest = queue.ProcessRequest

type DraftWikiQueueProcessResult = queue.ProcessResult

type DraftWikiQueuePruneResult = queue.PruneResult

type DraftWikiQueuePruneAllResult = queue.PruneAllResult

func TrimQueueMaterial(material string) string {
	return queue.TrimMaterial(material)
}

func QueueEventID(repoID, material, at string) string {
	return queue.EventID(repoID, material, at)
}

func AcquireQueueLock(projectStateDir string) (func(), bool, error) {
	return queue.AcquireLock(projectStateDir)
}

func CountQueueLines(path string, limit int) (int, error) {
	return queue.CountLines(path, limit)
}

func ReadQueueEvents(path string) ([]DraftWikiQueueEvent, []string, error) {
	return queue.ReadEvents(path)
}

func FormatQueueMalformedWarning(lineNumber int, line string) string {
	return queue.FormatMalformedWarning(lineNumber, line)
}

func AppendQueueEvent(path string, event DraftWikiQueueEvent) error {
	return queue.AppendEvent(path, event)
}

func CapQueueEvents(path string, keep int) error {
	return queue.CapEvents(path, keep)
}

func PruneQueuePath(path string, keep int) (DraftWikiQueuePruneResult, error) {
	return queue.PrunePath(path, keep)
}

func RewriteQueueEvents(path string, events []DraftWikiQueueEvent) error {
	return queue.RewriteEvents(path, events)
}
