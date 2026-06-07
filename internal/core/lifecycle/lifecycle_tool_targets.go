package lifecycle

import (
	"agent-harness/internal/core/lifecycle/doctarget"
)

func lifecycleDocTargetsForToolUse(req HookToolUseLifecycleRequest) []string {
	return doctarget.ForToolUse(req)
}

func toolUseMayMutateLifecycleFiles(tool, command string) bool {
	return doctarget.ToolUseMayMutateLifecycleFiles(tool, command)
}

func uniqueDocUpkeepEvents(events []DocUpkeepEvent) []DocUpkeepEvent {
	return doctarget.UniqueEvents(events)
}
