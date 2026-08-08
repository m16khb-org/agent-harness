package operationalhealth

import operationalhealthcontract "agent-harness/internal/contract/operationalhealth"

// 운영 건강 신호는 계약 DTO다. domain은 같은 이름으로 재노출만 한다.
type (
	Cycle            = operationalhealthcontract.Cycle
	LeaseHolderIndex = operationalhealthcontract.LeaseHolderIndex
	GitWorktree      = operationalhealthcontract.GitWorktree
	GitRef           = operationalhealthcontract.GitRef
	OrcaWorktree     = operationalhealthcontract.OrcaWorktree
	OrcaTerminal     = operationalhealthcontract.OrcaTerminal
	OrcaTask         = operationalhealthcontract.OrcaTask
	OrcaDispatch     = operationalhealthcontract.OrcaDispatch
	OrcaGate         = operationalhealthcontract.OrcaGate
	MessagePresence  = operationalhealthcontract.MessagePresence
	InventoryProblem = operationalhealthcontract.InventoryProblem
	StateArtifact    = operationalhealthcontract.StateArtifact
	Snapshot         = operationalhealthcontract.Snapshot
	Options          = operationalhealthcontract.Options
	Finding          = operationalhealthcontract.Finding
	Result           = operationalhealthcontract.Result
	CycleAuthority   = operationalhealthcontract.CycleAuthority
)

const (
	FindingInventoryUnknown       = operationalhealthcontract.FindingInventoryUnknown
	FindingDeadOwner              = operationalhealthcontract.FindingDeadOwner
	FindingWorktreeResidue        = operationalhealthcontract.FindingWorktreeResidue
	FindingTerminalResidue        = operationalhealthcontract.FindingTerminalResidue
	FindingTaskResidue            = operationalhealthcontract.FindingTaskResidue
	FindingGateResidue            = operationalhealthcontract.FindingGateResidue
	FindingMessageResidue         = operationalhealthcontract.FindingMessageResidue
	FindingNonMainBranchResidue   = operationalhealthcontract.FindingNonMainBranchResidue
	FindingStateArtifactResidue   = operationalhealthcontract.FindingStateArtifactResidue
	ProcessStatusLive             = operationalhealthcontract.ProcessStatusLive
	ProcessStatusDead             = operationalhealthcontract.ProcessStatusDead
	ProcessStatusIdentityMismatch = operationalhealthcontract.ProcessStatusIdentityMismatch
	ProcessStatusUnknown          = operationalhealthcontract.ProcessStatusUnknown
	ProfileSealed                 = operationalhealthcontract.ProfileSealed
	ProfileInteractive            = operationalhealthcontract.ProfileInteractive
	AuthorityLive                 = operationalhealthcontract.AuthorityLive
	AuthorityPreserved            = operationalhealthcontract.AuthorityPreserved
	AuthorityDead                 = operationalhealthcontract.AuthorityDead
	AuthorityUnknown              = operationalhealthcontract.AuthorityUnknown
)
