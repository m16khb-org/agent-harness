package hookcli

import (
	loopgatet5d "agent-harness/internal/adapter/issueops/loopgate"
	doctargetadapter "agent-harness/internal/adapter/lifecycle/doctarget"
	looprunadapter "agent-harness/internal/adapter/looprun"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	ToolUseMayMutateLifecycleFiles = doctargetadapter.ToolUseMayMutateLifecycleFiles
	loopgatet5d.RepoGateMissing = looprunadapter.RepoGateMissing
}
