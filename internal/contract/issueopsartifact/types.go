package issueopsartifact

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsleasecontract "agent-harness/internal/contract/issueopslease"
)

type Record = issueopscontract.IssueOpsRecord
type Staged = map[string]string

const (
	MaxBytes            = issueopsleasecontract.OwnerArtifactMaxBytes
	ExecutionModeOrca   = issueopscontract.ExecutionModeOrca
	LeaseStatusReleased = issueopscontract.LeaseStatusReleased
)

type RecoveryError struct {
	ID string
}

func (err *RecoveryError) Error() string {
	return "artifacts are sealed after execution prepare; only a clean released Orca generation may stage a plan, and execution replace --reseed is required before resume"
}

func (err *RecoveryError) IssueOpsErrorFields() map[string]any {
	return map[string]any{
		"code":            "artifact_stage_requires_reseed",
		"required_action": "execution replace --reseed",
		"next_command":    "agent-harness issueops execution status --id " + err.ID + " --json",
	}
}
