package lifecycle

import (
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	"agent-harness/internal/core/lifecycle/doctarget"
)

func lifecycleDocTargetsForToolUse(req lifecyclecontract.HookToolUseLifecycleRequest) []string {
	return doctarget.ForToolUse(req)
}

func toolUseMayMutateLifecycleFiles(tool, command string) bool {
	return doctarget.ToolUseMayMutateLifecycleFiles(tool, command)
}

func uniqueDocUpkeepEvents(events []lifecyclecontract.DocUpkeepEvent) []lifecyclecontract.DocUpkeepEvent {
	return doctarget.UniqueEvents(events)
}
