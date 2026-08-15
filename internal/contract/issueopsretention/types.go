package issueopsretention

import issueopscontract "agent-harness/internal/contract/issueops"

type Record = issueopscontract.IssueOpsRecord

type UnreadableRecordDiagnostic struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

type DeleteFailureDiagnostic struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

type Result struct {
	OK                    bool                         `json:"ok"`
	StateRoot             string                       `json:"state_root"`
	MaxAge                string                       `json:"max_age"`
	Cutoff                string                       `json:"cutoff"`
	DryRun                bool                         `json:"dry_run"`
	Pruned                []string                     `json:"pruned"`
	Kept                  []string                     `json:"kept"`
	ReadErrors            int                          `json:"read_errors"`
	Unreadable            []string                     `json:"unreadable"`
	UnreadableDiagnostics []UnreadableRecordDiagnostic `json:"unreadable_diagnostics"`
	DeleteErrors          int                          `json:"delete_errors"`
	Failed                []string                     `json:"failed"`
	DeleteDiagnostics     []DeleteFailureDiagnostic    `json:"delete_diagnostics"`
	Error                 string                       `json:"error,omitempty"`
}

const (
	LeaseStatusReleased = issueopscontract.LeaseStatusReleased
	PhaseDone           = issueopscontract.IssueOpsPhaseDone
)
