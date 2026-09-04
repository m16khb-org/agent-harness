package issueops

import (
	"context"
	"fmt"

	"issueops/internal/contract/issueops"
)

func PrepareExecutionReseedOwnerArtifacts(ctx context.Context, stateRoot, id string, execution issueops.Execution, readIssue ExecutionIssueSnapshotReadFunc) (ExecutionReseedArtifacts, error) {
	if execution.Lease.Generation < 2 {
		return ExecutionReseedArtifacts{}, fmt.Errorf("reseed owner artifacts require a replacement generation")
	}
	record, err := executionRecordAtGeneration(stateRoot, id, execution.Lease.Generation-1)
	if err != nil {
		return ExecutionReseedArtifacts{}, err
	}
	if record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil || execution.Mode != issueops.ExecutionModeOrca || execution.Orca == nil {
		return ExecutionReseedArtifacts{}, fmt.Errorf("reseed owner artifacts require an Orca execution")
	}
	record.Execution = &execution
	reseal, err := resealOwnerContextForReplacement(ctx, stateRoot, record, ExecutionReplaceDependencies{ReadIssue: readIssue})
	if err != nil {
		return ExecutionReseedArtifacts{}, err
	}
	return ExecutionReseedArtifacts{IssueBodySHA256: reseal.issueBodySHA256, ContextPacketPath: reseal.packetPath, ContextPacketSHA256: reseal.packetSHA256, OwnerPromptPath: reseal.promptPath, OwnerPromptSHA256: reseal.promptSHA256}, nil
}

type ExecutionReseedArtifacts struct {
	IssueBodySHA256     string
	ContextPacketPath   string
	ContextPacketSHA256 string
	OwnerPromptPath     string
	OwnerPromptSHA256   string
}

func ExecutionReseedNextCommand(id string, generation uint64, mode, claimTokenPath string) string {
	switch issueops.ExecutionMode(mode) {
	case issueops.ExecutionModeOrca:
		return ExecutionResumeRecoveryCommand(id, generation)
	case issueops.ExecutionModeDirect:
		return executionDirectClaimCommand(id, generation, claimTokenPath)
	default:
		return ""
	}
}
