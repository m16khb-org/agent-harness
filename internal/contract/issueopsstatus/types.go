package issueopsstatus

import issueopscontract "agent-harness/internal/contract/issueops"

type Record = issueopscontract.IssueOpsRecord
type Phase = issueopscontract.IssueOpsPhase
type Readiness = issueopscontract.IssueOpsReadiness
type PhaseLedger = issueopscontract.IssueOpsPhaseLedger
type PhaseLedgerEntry = issueopscontract.IssueOpsPhaseLedgerEntry

const (
	PhaseProblem             = issueopscontract.IssueOpsPhaseProblem
	PhaseGrill               = issueopscontract.IssueOpsPhaseGrill
	PhasePlan                = issueopscontract.IssueOpsPhasePlan
	PhaseCompatibilityReview = issueopscontract.IssueOpsPhaseCompatibilityReview
	PhaseImplement           = issueopscontract.IssueOpsPhaseImplement
	PhaseAISlopClean         = issueopscontract.IssueOpsPhaseAISlopClean
	PhaseFeedback            = issueopscontract.IssueOpsPhaseFeedback
	PhasePR                  = issueopscontract.IssueOpsPhasePR
	PhaseDone                = issueopscontract.IssueOpsPhaseDone
)
